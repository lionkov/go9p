## Dockerfile for running Linux builds/tests from macOS.
##
## Usage:
##   docker build -t go9p:test --target test .
##   docker run --rm go9p:test
##
## Optional:
##   docker build -t go9p:race --target race .
##   docker run --rm go9p:race

ARG GO_VERSION=1.26.0

FROM golang:${GO_VERSION}-bookworm AS base
WORKDIR /src

# Keep the module download layer stable.
COPY go.mod ./
RUN go mod download

COPY . .

FROM base AS test
RUN go test ./...
CMD ["go", "test", "./..."]

FROM base AS race
# Race detector requires CGO; Debian-based images support this out of the box.
RUN go test -race ./...
CMD ["go", "test", "-race", "./..."]

