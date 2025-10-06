BIN_OUTPUT_PATH = bin
BUILD_OUTPUT_PATH = build
TOOL_BIN = bin/gotools/$(shell uname -s)-$(shell uname -m)

DPKG_ARCH ?= $(shell dpkg --print-architecture)
ifeq ($(DPKG_ARCH),armhf)
DOCKER_ARCH ?= arm
else ifeq ($(DPKG_ARCH),arm64)
DOCKER_ARCH ?= arm64
else
DOCKER_ARCH ?= unknown
endif

OUTPUT_PATH = $(BIN_OUTPUT_PATH)/raspberry-pi-$(DOCKER_ARCH)

IMAGE_NAME = ghcr.io/viam-modules/raspberry-pi

.PHONY: module
module: build-$(DOCKER_ARCH) $(BIN_OUTPUT_PATH)/pigpiod-$(DOCKER_ARCH)/pigpiod
	rm -f $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz
	cp $(BIN_OUTPUT_PATH)/raspberry-pi-$(DOCKER_ARCH) $(BIN_OUTPUT_PATH)/raspberry-pi
	tar czf $(BIN_OUTPUT_PATH)/raspberry-pi-module.tar.gz $(BIN_OUTPUT_PATH)/raspberry-pi-$(DOCKER_ARCH) $(BIN_OUTPUT_PATH)/pigpiod-$(DOCKER_ARCH) run.sh meta.json

.PHONY: build-$(DOCKER_ARCH)
build-$(DOCKER_ARCH):
	go build -o $(BIN_OUTPUT_PATH)/raspberry-pi-$(DOCKER_ARCH) main.go

$(BIN_OUTPUT_PATH)/pigpiod-$(DOCKER_ARCH)/pigpiod:
	mkdir -p $(BIN_OUTPUT_PATH)/pigpiod-$(DOCKER_ARCH)
	mkdir -p $(BUILD_OUTPUT_PATH)
	cd $(BUILD_OUTPUT_PATH) && \
		wget https://github.com/joan2937/pigpio/archive/master.tar.gz && \
		tar zxf master.tar.gz && \
		cd pigpio-master && \
		make pigpiod
	cp $(BUILD_OUTPUT_PATH)/pigpio-master/pigpiod $(BUILD_OUTPUT_PATH)/pigpio-master/libpigpio.so.1 $(BIN_OUTPUT_PATH)/pigpiod-$(DOCKER_ARCH)

.PHONY: update-rdk
update-rdk:
	go get go.viam.com/rdk@latest
	go mod tidy

.PHONY: test
test:
	go test -c -o $(BIN_OUTPUT_PATH)/raspberry-pi-tests-$(DOCKER_ARCH)/ ./...
	for test in $$(ls $(BIN_OUTPUT_PATH)/raspberry-pi-tests-$(DOCKER_ARCH)/*.test) ; do \
	sudo $$test -test.v || exit $?; \
	done

.PHONY: tool-install
tool-install: $(TOOL_BIN)/golangci-lint

$(TOOL_BIN)/golangci-lint:
	GOBIN=`pwd`/$(TOOL_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: lint
lint: $(TOOL_BIN)/golangci-lint
	go mod tidy
	$(TOOL_BIN)/golangci-lint run -v --fix --config=./etc/.golangci.yaml --timeout 5m

.PHONY: docker-all
docker-all: docker-build-64 docker-build-32

.PHONY: docker-build
docker-build:
	cd docker && docker buildx build --load --no-cache --platform linux/$(DOCKER_ARCH) -t $(IMAGE_NAME):$(DOCKER_ARCH) .

.PHONY: docker-build-64
docker-build-64: 
	DOCKER_ARCH=arm64 make docker-build

.PHONY: docker-build-32
docker-build-32: 
	DOCKER_ARCH=arm make docker-build

.PHONY: docker-upload-all
docker-upload-all: docker-push-arm32 docker-push-arm64 docker-manifest

.PHONY: docker-push-arm32
docker-push-arm32:
	docker push $(IMAGE_NAME):arm

.PHONY: docker-push-arm64
docker-push-arm64:
	docker push $(IMAGE_NAME):arm64

.PHONY: docker-manifest
docker-manifest:
	docker manifest create --amend $(IMAGE_NAME):latest $(IMAGE_NAME):arm $(IMAGE_NAME):arm64
	docker manifest push $(IMAGE_NAME):latest

.PHONY: setup 
setup: 
	sudo apt-get install -qqy pigpio

clean:
	rm -rf $(BIN_OUTPUT_PATH) $(BUILD_OUTPUT_PATH)
