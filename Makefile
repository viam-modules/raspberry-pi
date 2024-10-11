BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
ARM64_OUTPUT = $(BIN_OUTPUT_PATH)/raspberry-pi/arm64
ARM32_OUTPUT = $(BIN_OUTPUT_PATH)/raspberry-pi/arm32
DOCKER_ARCH ?= arm64

.PHONY: module
module: build
	rm -f $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz
	tar czf $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi run.sh meta.json

.PHONY: build
build: build-arm64

.PHONY: build-arm64
build-arm64:
	rm -f $(ARM64_OUTPUT)
	GOARCH=arm64 go build -o $(ARM64_OUTPUT) main.go

.PHONY: build-arm32
build-arm32:
	rm -f $(ARM32_OUTPUT)
	GOARCH=arm GOARM=7 go build -o $(ARM32_OUTPUT) main.go

.PHONY: update-rdk
update-rdk:
	go get go.viam.com/rdk@latest
	go mod tidy

.PHONY: test
test:
	go test -c -o $(BIN_OUTPUT_PATH)/ raspberry-pi/...
	sudo $(BIN_OUTPUT_PATH)/*.test -test.v

.PHONY: tool-install
tool-install: $(TOOL_BIN)/golangci-lint

$(TOOL_BIN)/golangci-lint:
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: lint
lint: $(TOOL_BIN)/golangci-lint
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml

.PHONY: docker
docker:
	cd docker && docker buildx build --load --no-cache --platform linux/$(DOCKER_ARCH) -t ghcr.io/viam-modules/raspberry-pi:$(DOCKER_ARCH) .

docker-32-bit:
	cd docker && docker build --platform linux/arm -t ghcr.io/viam-modules/raspberry-pi:arm32 .

.PHONY: docker-upload
docker-upload:
	docker push ghcr.io/viam-modules/raspberry-pi:arm64

docker-upload-32:
	docker push ghcr.io/viam-modules/raspberry-pi:arm32

.PHONY: setup 
setup: 
	sudo apt-get install -qqy libpigpio-dev libpigpiod-if-dev pigpio

clean:
	rm -rf $(BIN_OUTPUT_PATH)
