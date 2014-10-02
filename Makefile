all: pgcov-html

pgcov-html: fetcher.go annotate.go main.go
	go build

clean:
	rm -f pgcov-html

.PHONY: clean
