.PHONY: generate build test clean

generate:
	templ generate ./...

build: generate
	CGO_ENABLED=0 go build -o bin/touchstone ./cmd/server/

test:
	go test -race -v -count=1 -timeout 120s ./...

clean:
	rm -rf bin/
