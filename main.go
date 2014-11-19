package main

import (
	"flag"
	"fmt"
	logpkg "log"
	"os"
	"os/signal"
	"regexp"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  %s [--help] [--outfile FILE] [--hide-source-list FILE] [--filter FILTER]

Options:
  --help                display this help and exit
  --outfile             write the output to FILE (default: coverage.html)
  --hide-source-list    reads a newline-separated list of functions from FILE
                        for which the source code should be hidden
  --filter              only show functions whose signature match the regular
                        expression FILTER
`, os.Args[0])
}

func main() {
	var displayHelp bool
	var outputFile string
	var hideSourceListFile string
	var funcNameFilter string
	flagSet := flag.NewFlagSet("args", flag.ExitOnError)
	flagSet.BoolVar(&displayHelp, "help", false, "")
	flagSet.StringVar(&outputFile, "outfile", "coverage.html", "")
	flagSet.StringVar(&hideSourceListFile, "hide-source-list", "", "")
	flagSet.StringVar(&funcNameFilter, "filter", "", "")
	flagSet.Usage = printUsage
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not parse command-line arguments: %s\n", err)
		os.Exit(1)
	}

	if displayHelp {
		printUsage()
		os.Exit(0)
	}

	log := logpkg.New(os.Stderr, "", logpkg.LstdFlags)

	var funcNameFilterRegexp *regexp.Regexp
	if funcNameFilter != "" {
		funcNameFilterRegexp, err = regexp.Compile(funcNameFilter)
		if err != nil {
			log.Fatalf("could not compile filter: %s", err)
		}
	}

	f := &Fetcher{}
	fetcherNotify, err := f.Listen("")
	if err != nil {
		log.Fatalf("could not start fetcher: %s", err)
	}

	sigint := make(chan os.Signal, 2)
	signal.Notify(sigint, os.Interrupt)

	select {
		case <-sigint:
			log.Print("received SIGINT, writing the output file")
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

	if funcNameFilterRegexp != nil {
		var filteredFunctions []*Function
		for _, fn := range functions {
			if funcNameFilterRegexp.MatchString(fn.Signature) {
				filteredFunctions = append(filteredFunctions, fn)
			}
		}
		functions = filteredFunctions
	}

	file, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("could not open output file %#v: %s", outputFile, err)
	}
	defer file.Close()
	err = Annotate(file, functions, hideSourceListFile)
	if err != nil {
		log.Fatalf("Annotate() failed: %s", err)
	}
}
