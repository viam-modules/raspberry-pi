BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
UNAME_S ?= $(shell uname -s)

module: build
	rm -f $(BIN_OUTPUT_PATH)/module.tar.gz
	tar czf $(BIN_OUTPUT_PATH)/module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi run.sh meta.json

build:
	rm -f $(BIN_OUTPUT_PATH)/raspberry-pi
	go build -o $(BIN_OUTPUT_PATH)/raspberry-pi main.go

update-rdk:
	go get go.viam.com/rdk@latest
	go mod tidy

test:
	go test -c -o $(BIN_OUTPUT_PATH)/ raspberry-pi/...
	sudo $(BIN_OUTPUT_PATH)/*.test -test.v

tool-install: $(TOOL_BIN)/golangci-lint

$(TOOL_BIN)/golangci-lint:
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint

lint: $(TOOL_BIN)/golangci-lint
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml

.PHONY: docker
docker:
	cd docker && docker buildx build --load --no-cache --platform linux/arm64 -t ghcr.io/viam-modules/raspberry-pi:arm64 .

docker-upload:
	docker push ghcr.io/viam-modules/raspberry-pi:arm64

clean:
	rm -rf $(BIN_OUTPUT_PATH)
	