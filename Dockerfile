# Build the portal binary.
# The builder runs on the *build* platform and cross-compiles to the target
# arch via GOARCH (pure Go, CGO_ENABLED=0), so multi-arch builds need no QEMU.
FROM --platform=$BUILDPLATFORM golang:1.26.8@sha256:2dbae744204892730b7032501f5973360ce57cfe118a194b338a97b5aa2d40cc AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Module files first so the download layer is cached. go.* rather than
# go.mod go.sum: a module with no dependencies has no go.sum.
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Go sources (.dockerignore re-includes only the module files and the cmd/,
# internal/ and api/ trees, minus tests).
COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -o opm-portal ./cmd/opm-portal

# Distroless static, nonroot: no shell, no package manager.
FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
WORKDIR /
COPY --from=builder /workspace/opm-portal .

USER 65532:65532

ENTRYPOINT ["/opm-portal"]
