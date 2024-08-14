BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
UNAME_S ?= $(shell uname -s)

.PHONY: build
build: $(BIN_OUTPUT_PATH)/raspberry-pi

$(BIN_OUTPUT_PATH)/raspberry-pi: *.go go.* */*.go */*.c */*.h
	go build -o $(BIN_OUTPUT_PATH)/raspberry-pi main.go

.PHONY: module.tar.gz
module.tar.gz: $(BIN_OUTPUT_PATH)/module.tar.gz

$(BIN_OUTPUT_PATH)/module.tar.gz: $(BIN_OUTPUT_PATH)/raspberry-pi
	tar czf $(BIN_OUTPUT_PATH)/module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi

.PHONY: update-rdk
update-rdk:
	go get go.viam.com/rdk@latest
	go mod tidy

.PHONY: test
test:
	go test ./...

.PHONY: tool-install
tool-install: $(TOOL_BIN)/combined $(TOOL_BIN)/golangci-lint $(TOOL_BIN)/actionlint

$(TOOL_BIN)/combined $(TOOL_BIN)/golangci-lint $(TOOL_BIN)/actionlint:
	GOBIN=`pwd`/$(TOOL_BIN) go install \
	github.com/edaniels/golinters/cmd/combined \
	github.com/golangci/golangci-lint/cmd/golangci-lint \
	github.com/rhysd/actionlint/cmd/actionlint

.PHONY: lint
lint: tool-install
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml

.PHONY: docker
docker:
	cd docker && docker buildx build --load --no-cache --platform linux/arm64 -t ghcr.io/viam-modules/raspberry-pi:arm64 .

.PHONY: docker-upload
docker-upload:
	docker push ghcr.io/viam-modules/raspberry-pi:arm64
