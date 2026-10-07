GO := /tmp/go/bin/go

.PHONY: all build test vet race bench run demo clean

all: build vet test

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test -count=1 ./...

race:
	$(GO) test -race -count=1 ./...

bench:
	$(GO) test -bench=. -benchmem ./...

run:
	$(GO) run .

demo:
	$(GO) run ./cmd/demo/

clean:
	rm -f okxclient/okxclient_test.go.backup
