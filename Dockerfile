# Build the portal binary.
# The builder runs on the *build* platform and cross-compiles to the target
# arch via GOARCH (pure Go, CGO_ENABLED=0), so multi-arch builds need no QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Module files first so the download layer is cached. go.* rather than
# go.mod go.sum: a module with no dependencies has no go.sum.
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Go sources (.dockerignore re-includes only *.go and the module files).
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -o opm-portal ./cmd/opm-portal

# Distroless static, nonroot: no shell, no package manager.
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/opm-portal .

USER 65532:65532

ENTRYPOINT ["/opm-portal"]
