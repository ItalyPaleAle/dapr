#
# Copyright 2021 The Dapr Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#


# Docker image build and push setting
DOCKER:=docker
DOCKERFILE_DIR?=./docker

DAPR_SYSTEM_IMAGE_NAME=$(RELEASE_NAME)
DAPR_RUNTIME_IMAGE_NAME=daprd
DAPR_PLACEMENT_IMAGE_NAME=placement
DAPR_SENTRY_IMAGE_NAME=sentry

WINDOWS_BASE_TAG=latest

# build docker image for linux
BIN_PATH=$(OUT_DIR)/$(TARGET_OS)_$(TARGET_ARCH)

ifeq ($(TARGET_OS), windows)
  DOCKERFILE?=Dockerfile-windows
  BIN_PATH := $(BIN_PATH)/release
else ifeq ($(origin DEBUG), undefined)
  DOCKERFILE?=Dockerfile
  BIN_PATH := $(BIN_PATH)/release
else ifeq ($(DEBUG),0)
  DOCKERFILE?=Dockerfile
  BIN_PATH := $(BIN_PATH)/release
else
  DOCKERFILE?=Dockerfile-debug
  BIN_PATH := $(BIN_PATH)/debug
endif

ifeq ($(TARGET_ARCH),arm)
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/arm/v7
else ifeq ($(TARGET_ARCH),arm64)
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/arm64/v8
else
  DOCKER_IMAGE_PLATFORM:=$(TARGET_OS)/amd64
endif

# Supported docker image architecture
DOCKER_MULTI_ARCH?=linux-amd64 linux-arm linux-arm64 windows-amd64

################################################################################
# Target: docker-build, docker-push                                            #
################################################################################

LINUX_BINS_OUT_DIR=$(OUT_DIR)/linux_$(GOARCH)
DOCKER_IMAGE_TAG=$(DAPR_REGISTRY)/$(DAPR_SYSTEM_IMAGE_NAME):$(DAPR_TAG)
DAPR_RUNTIME_DOCKER_IMAGE_TAG=$(DAPR_REGISTRY)/$(DAPR_RUNTIME_IMAGE_NAME):$(DAPR_TAG)
DAPR_PLACEMENT_DOCKER_IMAGE_TAG=$(DAPR_REGISTRY)/$(DAPR_PLACEMENT_IMAGE_NAME):$(DAPR_TAG)
DAPR_SENTRY_DOCKER_IMAGE_TAG=$(DAPR_REGISTRY)/$(DAPR_SENTRY_IMAGE_NAME):$(DAPR_TAG)

ifeq ($(LATEST_RELEASE),true)
DOCKER_IMAGE_LATEST_TAG=$(DAPR_REGISTRY)/$(DAPR_SYSTEM_IMAGE_NAME):$(LATEST_TAG)
DAPR_RUNTIME_DOCKER_IMAGE_LATEST_TAG=$(DAPR_REGISTRY)/$(DAPR_RUNTIME_IMAGE_NAME):$(LATEST_TAG)
DAPR_PLACEMENT_DOCKER_IMAGE_LATEST_TAG=$(DAPR_REGISTRY)/$(DAPR_PLACEMENT_IMAGE_NAME):$(LATEST_TAG)
DAPR_SENTRY_DOCKER_IMAGE_LATEST_TAG=$(DAPR_REGISTRY)/$(DAPR_SENTRY_IMAGE_NAME):$(LATEST_TAG)
endif


# To use buildx: https://github.com/docker/buildx#docker-ce
export DOCKER_CLI_EXPERIMENTAL=enabled

# check the required environment variables
check-docker-env:
ifeq ($(DAPR_REGISTRY),)
	$(error DAPR_REGISTRY environment variable must be set)
endif
ifeq ($(DAPR_TAG),)
	$(error DAPR_TAG environment variable must be set)
endif

check-arch:
ifeq ($(TARGET_OS),)
	$(error TARGET_OS environment variable must be set)
endif
ifeq ($(TARGET_ARCH),)
	$(error TARGET_ARCH environment variable must be set)
endif

setup-qemu:
	-$(DOCKER) buildx create --use --name daprbuild
	-$(DOCKER) run --rm --privileged multiarch/qemu-user-static --reset -p yes

docker-build: check-docker-env check-arch
ifeq ($(TARGET_OS), windows)
docker-build: get-windows-base-tag
	$(eval EXTRA_BUILD_ARG=--build-arg WINDOWS_BASE_TAG=$(WINDOWS_BASE_TAG))
endif
	$(info Building $(DOCKER_IMAGE_TAG) docker image ...)
	$(DOCKER) buildx build --build-arg PKG_FILES=* $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	$(DOCKER) buildx build --build-arg PKG_FILES=daprd $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	$(DOCKER) buildx build --build-arg PKG_FILES=placement $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	$(DOCKER) buildx build --build-arg PKG_FILES=sentry $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)

# push docker image to the registry
docker-push: docker-build
ifeq ($(TARGET_OS), windows)
docker-push: get-windows-base-tag
	$(eval EXTRA_BUILD_ARG=--build-arg WINDOWS_BASE_TAG=$(WINDOWS_BASE_TAG))
endif
	$(info Pushing $(DOCKER_IMAGE_TAG) docker image ...)
	$(DOCKER) buildx build --build-arg PKG_FILES=* $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH) --push
	$(DOCKER) buildx build --build-arg PKG_FILES=daprd $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH) --push
	$(DOCKER) buildx build --build-arg PKG_FILES=placement $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH) --push
	$(DOCKER) buildx build --build-arg PKG_FILES=sentry $(EXTRA_BUILD_ARG) --platform $(DOCKER_IMAGE_PLATFORM) -f $(DOCKERFILE_DIR)/$(DOCKERFILE) $(BIN_PATH) -t $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH) --push

# push docker image to kind cluster
docker-push-kind: docker-build
	$(info Pushing $(DOCKER_IMAGE_TAG) docker image to kind cluster...)
	kind load docker-image $(DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	kind load docker-image $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	kind load docker-image $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)
	kind load docker-image $(DAPR_SENTRY_DOCKER_IMAGE_TAG)-$(TARGET_OS)-$(TARGET_ARCH)

# publish muti-arch docker image to the registry
docker-manifest-create: check-docker-env
	$(DOCKER) manifest create $(DOCKER_IMAGE_TAG) $(DOCKER_MULTI_ARCH:%=$(DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_RUNTIME_DOCKER_IMAGE_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_SENTRY_DOCKER_IMAGE_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_SENTRY_DOCKER_IMAGE_TAG)-%)
ifeq ($(LATEST_RELEASE),true)
	$(DOCKER) manifest create $(DOCKER_IMAGE_LATEST_TAG) $(DOCKER_MULTI_ARCH:%=$(DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_RUNTIME_DOCKER_IMAGE_LATEST_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_RUNTIME_DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_PLACEMENT_DOCKER_IMAGE_LATEST_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)-%)
	$(DOCKER) manifest create $(DAPR_SENTRY_DOCKER_IMAGE_LATEST_TAG) $(DOCKER_MULTI_ARCH:%=$(DAPR_SENTRY_DOCKER_IMAGE_TAG)-%)
endif

docker-publish: docker-manifest-create
	$(DOCKER) manifest push $(DOCKER_IMAGE_TAG)
	$(DOCKER) manifest push $(DAPR_RUNTIME_DOCKER_IMAGE_TAG)
	$(DOCKER) manifest push $(DAPR_PLACEMENT_DOCKER_IMAGE_TAG)
	$(DOCKER) manifest push $(DAPR_SENTRY_DOCKER_IMAGE_TAG)
ifeq ($(LATEST_RELEASE),true)
	$(DOCKER) manifest push $(DOCKER_IMAGE_LATEST_TAG)
	$(DOCKER) manifest push $(DAPR_RUNTIME_DOCKER_IMAGE_LATEST_TAG)
	$(DOCKER) manifest push $(DAPR_PLACEMENT_DOCKER_IMAGE_LATEST_TAG)
	$(DOCKER) manifest push $(DAPR_SENTRY_DOCKER_IMAGE_LATEST_TAG)
endif

get-windows-base-tag:
	$(eval WINDOWS_BASE_TAG=$(shell $(RUN_BUILD_TOOLS) docker hash \
		--name "windows-base-1809" \
		--dir "../$(DOCKERFILE_DIR)" \
		--platform "$(DOCKER_IMAGE_PLATFORM)" \
	))
	@echo "Windows base tag: $(WINDOWS_BASE_TAG)"

check-docker-env-windows-base:
ifeq ($(DAPR_REGISTRY),)
	$(error DAPR_REGISTRY environment variable must be set)
endif

docker-windows-base-build: check-docker-env-windows-base get-windows-base-tag
	$(DOCKER) build -f $(DOCKERFILE_DIR)/windows-base-1809/Dockerfile $(DOCKERFILE_DIR)/windows-base-1809 -t $(DAPR_REGISTRY)/windows-base:$(WINDOWS_BASE_TAG)
	$(DOCKER) build --build-arg WINDOWS_VERSION=1809 -f $(DOCKERFILE_DIR)/Dockerfile-windows-php-base $(DOCKERFILE_DIR) -t $(DAPR_REGISTRY)/windows-php-base:$(WINDOWS_BASE_TAG)
	$(DOCKER) build --build-arg WINDOWS_VERSION=1809 -f $(DOCKERFILE_DIR)/Dockerfile-windows-python-base $(DOCKERFILE_DIR) -t $(DAPR_REGISTRY)/windows-python-base:$(WINDOWS_BASE_TAG)

docker-windows-base-push: check-docker-env-windows-base get-windows-base-tag
	$(DOCKER) push $(DAPR_REGISTRY)/windows-base:$(WINDOWS_BASE_TAG)
	$(DOCKER) push $(DAPR_REGISTRY)/windows-php-base:$(WINDOWS_BASE_TAG)
	$(DOCKER) push $(DAPR_REGISTRY)/windows-python-base:$(WINDOWS_BASE_TAG)

################################################################################
# Target: build-dev-container, push-dev-container                              #
################################################################################

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG?=0.1.8

# Use this to pin a specific version of the Dapr CLI to a devcontainer
DEV_CONTAINER_CLI_TAG?=1.8.0

# Dapr container image name
DEV_CONTAINER_IMAGE_NAME=dapr-dev

DEV_CONTAINER_DOCKERFILE=Dockerfile
DEV_CONTAINER_DOCKERFILE_DIR=./docker/dev-container

check-docker-env-for-dev-container:
ifeq ($(DAPR_REGISTRY),)
	$(error DAPR_REGISTRY environment variable must be set)
endif

build-dev-container:
ifeq ($(DAPR_REGISTRY),)
	$(info DAPR_REGISTRY environment variable not set, tagging image without registry prefix.)
	$(info `make tag-dev-container` should be run with DAPR_REGISTRY before `make push-dev-container.)
	$(DOCKER) build --build-arg DAPR_CLI_VERSION=$(DEV_CONTAINER_CLI_TAG) -f $(DEV_CONTAINER_DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) $(DEV_CONTAINER_DOCKERFILE_DIR)/. -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	$(DOCKER) build --build-arg DAPR_CLI_VERSION=$(DEV_CONTAINER_CLI_TAG) -f $(DEV_CONTAINER_DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) $(DEV_CONTAINER_DOCKERFILE_DIR)/. -t $(DAPR_REGISTRY)/$(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
endif

tag-dev-container: check-docker-env-for-dev-container
	$(DOCKER) tag $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) $(DAPR_REGISTRY)/$(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)

push-dev-container: check-docker-env-for-dev-container
	$(DOCKER) push $(DAPR_REGISTRY)/$(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)

build-dev-container-all-arch:
ifeq ($(DAPR_REGISTRY),)
	$(info DAPR_REGISTRY environment variable not set, tagging image without registry prefix.)
	$(DOCKER) buildx build \
		--build-arg DAPR_CLI_VERSION=$(DEV_CONTAINER_CLI_TAG) \
		-f $(DEV_CONTAINER_DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) \
		--platform linux/amd64,linux/arm64 \
		-t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) \
		$(DEV_CONTAINER_DOCKERFILE_DIR)/.
else
	$(DOCKER) buildx build \
		--build-arg DAPR_CLI_VERSION=$(DEV_CONTAINER_CLI_TAG) \
		-f $(DEV_CONTAINER_DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) \
		--platform linux/amd64,linux/arm64 \
		-t $(DAPR_REGISTRY)/$(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) \
		$(DEV_CONTAINER_DOCKERFILE_DIR)/.
endif

push-dev-container-all-arch: check-docker-env-for-dev-container
	$(DOCKER) buildx build \
		--build-arg DAPR_CLI_VERSION=$(DEV_CONTAINER_CLI_TAG) \
		-f $(DEV_CONTAINER_DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) \
		--platform linux/amd64,linux/arm64 \
		--push \
		-t $(DAPR_REGISTRY)/$(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) \
		$(DEV_CONTAINER_DOCKERFILE_DIR)/.
