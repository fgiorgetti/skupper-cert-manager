LDFLAGS_EXTRA ?= -s -w # default to building stripped executables
LDFLAGS := ${LDFLAGS_EXTRA}
GO_IMAGE_BASE_TAG := 1.24.9

GOOS ?= linux
GOARCH ?= amd64

DOCKER := docker
REGISTRY := quay.io/fgiorgetti
IMAGE := skupper-cert-manager
IMAGE_TAG := latest
PLATFORMS ?= linux/amd64

build:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -ldflags="${LDFLAGS}" -o skupper-cert-manager ./main.go

docker-build-push:
	${DOCKER} buildx build --push \
	--platform ${PLATFORMS} \
	-t "${REGISTRY}/${IMAGE}:${IMAGE_TAG}" \
	.

