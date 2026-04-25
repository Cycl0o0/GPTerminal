BINARY := gpterminal
VERSION := 0.1.0
GOFLAGS := -ldflags="-s -w"

.PHONY: build install clean

build:
	go build $(GOFLAGS) -o $(BINARY) .

install: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
