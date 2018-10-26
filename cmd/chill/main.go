package main

import (
	"fmt"
	"log"
	"os"

	"github.com/fasterthanlime/chill"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("chill", "A little icecast metadata parser")

	url = app.Arg("url", "The stream to poll").String()
)

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	endpoint := chill.Endpoint{
		URL: *url,
	}
	must(chill.Poll(endpoint, chill.WithMetadataCallback(func(title string) {
		log.Printf("%s", title)
	})))
}

func must(err error) {
	if err != nil {
		panic(fmt.Sprintf("fatal error: %+v", err))
	}
}
