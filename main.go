package main

import (
	logpkg "log"
	"os"
	"os/signal"
)

func main() {
	log := logpkg.New(os.Stderr, "", logpkg.LstdFlags)

	f := &Fetcher{}
	fetcherNotify, err := f.Listen("")
	if err != nil {
		log.Fatalf("could not start fetcher: %s", err)
	}

	sigint := make(chan os.Signal, 2)
	signal.Notify(sigint, os.Interrupt)

	select {
		case <-sigint:
			log.Print("received SIGINT, writing coverage.html")
			/* success */
		case err = <-fetcherNotify:
			log.Fatalf("fetcher failed: %s", err)
	}

	go func() {
		<-sigint
		log.Fatalf("received a second SIGINT")
	}()

	functions, err := f.Done(fetcherNotify)
	if err != nil {
		log.Fatalf("fetcher failed: %s", err)
	}
	file, err := os.Create("coverage.html")
	if err != nil {
		log.Fatalf("could not open coverage.html: %s", err)
	}
	defer file.Close()
	err = Annotate(file, functions)
	if err != nil {
		log.Fatalf("Annotate() failed: %s", err)
	}
}
