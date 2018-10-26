package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/fasterthanlime/chill"
	"github.com/getlantern/systray"
	"github.com/martinlindhe/notify"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("chillplay", "A little icecast player")

	url = app.Arg("url", "The stream to play").String()
)

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	iconBytes, err := Asset("data/icon.ico")
	must(err)
	systray.SetIcon(iconBytes)
	systray.SetTitle("chill")
	nowPlaying := systray.AddMenuItem("Now playing", "")
	quit := systray.AddMenuItem("Quit", "")

	go func() {
		<-quit.ClickedCh
		os.Exit(1)
	}()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	endpoint := chill.Endpoint{
		URL: *url,
	}

	r, w := io.Pipe()
	go func() {
		must(chill.Poll(endpoint, chill.WithMetadataCallback(func(title string) {
			log.Printf("%s", title)
			nowPlaying.SetTitle(title)
			notify.Notify("chill", "Now playing", title, "")
		}), chill.WithAudioSink(w)))
	}()

	log.Printf("Opening audio stream...")
	stream, format, err := mp3.Decode(r)
	must(err)
	log.Printf("Opened! %v", format)

	must(speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)))
	log.Printf("Speaker initialized! Playing...")

	done := make(chan struct{})
	speaker.Play(beep.Seq(stream, beep.Callback(func() {
		close(done)
	})))
	<-done
}

func onExit() {
	os.Exit(0)
}

func must(err error) {
	if err != nil {
		panic(fmt.Sprintf("fatal error: %+v", err))
	}
}
