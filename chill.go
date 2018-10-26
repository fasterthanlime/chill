package chill

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/itchio/httpkit/timeout"
	"golang.org/x/text/encoding/charmap"

	"github.com/pkg/errors"
)

type MetadataCallback func(title string)

type Endpoint struct {
	// The address of the icecast server
	URL string

	// The encoding of the metadata (optional, defaults to iso-8859-1)
	Encoding string
}

type pollOptions struct {
	Sink     io.Writer
	Callback MetadataCallback
}

type Option func(opts *pollOptions)

func WithMetadataCallback(callback MetadataCallback) Option {
	return func(opts *pollOptions) {
		opts.Callback = callback
	}
}

func WithAudioSink(sink io.Writer) Option {
	return func(opts *pollOptions) {
		opts.Sink = sink
	}
}

func Poll(endpoint Endpoint, o ...Option) error {
	err := doPoll(endpoint, o...)
	if err != nil {
		return errors.WithMessagef(err, "polling %s", endpoint.URL)
	}
	return nil
}

const metadataBlockSize = 16

func doPoll(endpoint Endpoint, o ...Option) error {
	opts := &pollOptions{}
	for _, opt := range o {
		opt(opts)
	}
	if opts.Sink == nil {
		opts.Sink = ioutil.Discard
	}

	// this has something like 10s connect timeout, 30s idle timeout, etc.
	client := timeout.NewDefaultClient()

	// errors here are DNS errors, mostly
	req, err := http.NewRequest("GET", endpoint.URL, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	// this is needed for icecast servers to send us interleaved metadata
	req.Header.Set("icy-metadata", "1")

	// this follows redirects
	res, err := client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}

	if res.StatusCode != 200 {
		return errors.Errorf("HTTP code %d", res.StatusCode)
	}

	metaInt := res.Header.Get("icy-metaint")
	if metaInt == "" {
		return errors.Errorf("Missing metadata header")
	}

	audioBytes, err := strconv.ParseInt(metaInt, 10, 64)
	if err != nil {
		return errors.WithStack(err)
	}

	stream := bufio.NewReader(res.Body)
	metadataBuffer := new(bytes.Buffer)
	decoder := charmap.ISO8859_1.NewDecoder()

	for {
		// for each "frame", forward audio data
		_, err := io.CopyN(opts.Sink, stream, audioBytes)
		if err != nil {
			return errors.WithStack(err)
		}

		metadataBlocks, err := stream.ReadByte()
		if err != nil {
			return errors.WithStack(err)
		}
		metadataSize := int64(metadataBlocks) * metadataBlockSize

		metadataBuffer.Reset()
		_, err = io.CopyN(metadataBuffer, stream, metadataSize)
		if err != nil {
			return errors.WithStack(err)
		}
		payload := metadataBuffer.String()
		payload = strings.Trim(payload, "\x00")
		payload, err = decoder.String(payload)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, pair := range strings.Split(payload, ";") {
			if pair == "" {
				continue
			}

			tokens := strings.SplitN(pair, "=", 2)
			key, value := tokens[0], tokens[1]
			if key == "StreamTitle" {
				value = unquoteMetadataValue(value)
				if strings.Trim(value, " -") == "" {
					continue
				}
				if opts.Callback != nil {
					opts.Callback(value)
				}
			}
		}
	}
}

// unquoteMetadataValue turns foo into foo and 'foo' into foo.
func unquoteMetadataValue(s string) string {
	if strings.HasPrefix(s, "'") {
		return strings.TrimPrefix(strings.TrimSuffix(s, "'"), "'")
	}
	return s
}
