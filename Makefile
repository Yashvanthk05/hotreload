.PHONY: build run demo test clean tidy

GOPATH_BIN := $(HOME)/go/bin
GO := $(shell which go 2>/dev/null || echo $(HOME)/go/bin/go)
HOTRELOAD_BIN := ./bin/hotreload
TESTSERVER_BIN := ./testserver/bin/server

## build: compile the hotreload binary
build:
	@mkdir -p ./bin
	$(GO) build -o $(HOTRELOAD_BIN) .
	@echo "✓ Built $(HOTRELOAD_BIN)"

## tidy: download dependencies and tidy go.mod/go.sum
tidy:
	$(GO) mod tidy
	@echo "✓ Dependencies tidied"

## run: build and run hotreload with the testserver
run: build
	@mkdir -p ./testserver/bin
	$(HOTRELOAD_BIN) \
		--root ./testserver \
		--build "$(GO) build -o $(TESTSERVER_BIN) ./testserver" \
		--exec "$(TESTSERVER_BIN)"

## demo: same as run, with a hint to the developer
demo: build
	@echo ""
	@echo "==================================================="
	@echo "  hotreload demo — watching ./testserver"
	@echo "  Edit testserver/main.go and watch it reload!"
	@echo "  curl http://localhost:8080 to test the server"
	@echo "==================================================="
	@echo ""
	@$(MAKE) run

## test: run all tests
test: tidy
	$(GO) test -v -race -timeout 30s ./...

## clean: remove built binaries
clean:
	rm -rf ./bin ./testserver/bin
	@echo "✓ Cleaned"

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
