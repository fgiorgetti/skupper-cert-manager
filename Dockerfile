ARG GO_IMAGE_BASE_TAG=1.24
FROM --platform=$BUILDPLATFORM golang:${GO_IMAGE_BASE_TAG} AS builder

ARG TARGETARCH

#RUN apt update && apt install -y jq

WORKDIR /go/src/app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

# Build statically linked binary
ENV CGO_ENABLED=0
RUN make -B build GOARCH=$TARGETARCH

# Use scratch for minimal image size
FROM scratch

LABEL \
  org.opencontainers.image.title="Skupper Cert Manager Integration" \
  org.opencontainers.image.description="Kubernetes controller for reconciling Skupper Certificates using Cert-Manager"

WORKDIR /app

# Copy the statically linked binary
COPY --from=builder /go/src/app/skupper-cert-manager .

# Use numeric user ID (no need to create user in scratch)
USER 10000

CMD ["/app/skupper-cert-manager"]

