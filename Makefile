.PHONY: build run demo test clean tidy

GOPATH_BIN := $(HOME)/go/bin
GO := $(shell which go 2>/dev/null || echo $(HOME)/go/bin/go)
HOTRELOAD_BIN := ./bin/hotreload
TESTSERVER_BIN := ./bin/server

build:
	@mkdir -p ./bin
	$(GO) build -o $(HOTRELOAD_BIN) .
	@echo "Built $(HOTRELOAD_BIN)"

tidy:
	$(GO) mod tidy
	@echo "Dependencies tidied"

run: build
	@mkdir -p ./bin
	$(HOTRELOAD_BIN) \
		--root ./testserver \
		--build "$(GO) build -o $(TESTSERVER_BIN) ./testserver" \
		--exec "$(TESTSERVER_BIN)"

demo: build
	@echo "hotreload demo - watching ./testserver"
	@echo "Edit testserver/main.go and watch it reload"
	@echo "curl http://localhost:8080 to test the server"
	@$(MAKE) run

test: tidy
	$(GO) test -v -race -timeout 30s ./...

clean:
	rm -rf ./bin ./testserver/bin
	@echo "Cleaned"
