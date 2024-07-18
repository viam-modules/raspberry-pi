BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
UNAME_S ?= $(shell uname -s)

build:
        rm -f $(BIN_OUTPUT_PATH)/raspberry-pi
        CGO_LDFLAGS='-lpigpiod_if2' go build -o $(BIN_OUTPUT_PATH)/raspberry-pi main.go

module.tar.gz: build
	rm -f $(BIN_OUTPUT_PATH)/module.tar.gz
	tar czf $(BIN_OUTPUT_PATH)/module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi

test:
	sudo apt install libnlopt-dev
	go test ./...


tool-install:
	GOBIN=`pwd`/$(TOOL_BIN) go install \
	github.com/edaniels/golinters/cmd/combined \
	github.com/golangci/golangci-lint/cmd/golangci-lint \
	github.com/rhysd/actionlint/cmd/actionlint

lint: tool-install
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml