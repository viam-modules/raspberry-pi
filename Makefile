BIN_OUTPUT_PATH = bin
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)
ARM64_OUTPUT = $(BIN_OUTPUT_PATH)/raspberry-pi/arm64
ARM32_OUTPUT = $(BIN_OUTPUT_PATH)/raspberry-pi/arm32
DOCKER_ARCH ?= arm64

IMAGE_NAME = ghcr.io/viam-modules/raspberry-pi
ARM32_TAG = $(IMAGE_NAME):arm
ARM64_TAG = $(IMAGE_NAME):arm64

.PHONY: module
module: build
	rm -f $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz
	tar czf $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi run.sh meta.json

.PHONY: build-all
build-all: build-arm64 build-arm32

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

.PHONY: docker-all
docker-all: docker-build-64 docker-build-32

.PHONY: docker-build-64
docker-build-64: 
	cd docker && docker buildx build --load --no-cache --platform linux/arm64 -t $(ARM64_TAG) --build-arg ARCH=arm64 .

.PHONY: docker-build-32
docker-build-32: 
	cd docker && docker buildx build --load --no-cache --platform linux/arm -t $(ARM32_TAG) --build-arg ARCH=arm .

.PHONY: docker-upload-all
docker-upload-all: docker-push-arm32 docker-push-arm64 docker-manifest

.PHONY: docker-push-arm32
docker-push-arm32:
	docker push $(ARM32_TAG)

.PHONY: docker-push-arm64
docker-push-arm64:
	docker push $(ARM64_TAG)

.PHONY: docker-manifest
docker-manifest:
	docker manifest create --amend $(IMAGE_NAME):latest $(ARM32_TAG) $(ARM64_TAG)
	docker manifest push $(IMAGE_NAME):latest

.PHONY: setup 
setup: 
	sudo apt-get install -qqy libpigpio-dev libpigpiod-if-dev pigpio

clean:
	rm -rf $(BIN_OUTPUT_PATH)
