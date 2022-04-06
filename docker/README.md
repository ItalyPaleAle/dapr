# Dockerfiles

This includes dockerfiles to build Dapr release and debug images and development container images for go dev environment.

* Dockerfile: Dapr Release Image
* Dockerfile-debug: Dapr Debug Image - WIP
* Dockerfile-dev: Development container image which VSCode Dev container can use.

## Building cross-platform Docker images

You can build cross-platform Docker images using Docker Buildx. For example, you can build `amd64` containers while on an `arm64` machine, or vice-versa. You will need a recent-enough version of the Docker tools installed on your system.

First, make sure that Docker Buildx is configured and that QEMU is available for emulation. You can perform the steps manually or just use the command from our Makefile:

```sh
make docker-setup-buildx
```

Then, build Dapr for the desired architecture, for example `arm64`:

```sh
GOARCH=arm64 make build-linux
```

You can then build the Docker images for Dapr, for the e2e tests, and for perf tests:

```sh
# Make sure you have the required env vars set
# DAPR_REGISTRY is the Docker registry to use (your Docker ID or the full URL to a Docker registry)
export DAPR_REGISTRY=...
# DAPR_TAG is the tag to use for the generated Docker images
export DAPR_TAG=...

# Build and push Dapr's Docker images
TARGET_ARCH=arm64 make docker-build
TARGET_ARCH=arm64 make docker-push

# Build and push images for E2E test apps
TARGET_ARCH=arm64 make build-e2e-app-all
TARGET_ARCH=arm64 make push-e2e-app-all

# Build and push images for E2E test apps
TARGET_ARCH=arm64 make build-perf-app-all
TARGET_ARCH=arm64 make push-perf-app-all
```
