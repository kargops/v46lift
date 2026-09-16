BINARY := v46lift

.PHONY: build test fmt clean

build:
	mkdir -p bin
	go build -o bin/$(BINARY) ./cmd/v46lift

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

clean:
	rm -rf bin dist
