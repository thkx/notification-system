GO ?= go
GOFLAGS ?=

.PHONY: fmt test test-short bench run ci

fmt:
	$(GO)fmt ./...

test:
	$(GO) test $(GOFLAGS) ./...

test-short:
	$(GO) test $(GOFLAGS) -short ./...

bench:
	$(GO) test $(GOFLAGS) -run '^$$' -bench . -benchmem ./...

run:
	$(GO) run ./cmd

ci: fmt test
