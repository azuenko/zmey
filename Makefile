test:
	go test -v -race -coverprofile=coverage.out github.com/stratumn/zmey/...

errcheck:
	go get github.com/kisielk/errcheck

lint: errcheck
	go vet github.com/stratumn/zmey/...
	golint github.com/stratumn/zmey/...
	errcheck github.com/stratumn/zmey/...
