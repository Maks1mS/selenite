# FROM registry.altlinux.org/sisyphus/golang:1.25.6@sha256:6e7d2dca3fa4dcd06f8896af7a081c8608b38d82cf4115d9852f14e137a78989 as selenite-controller
# RUN apt-get update && apt-get -y install ca-certificates
FROM golang:1.26.1@sha256:e2ddb153f786ee6210bf8c40f7f35490b3ff7d38be70d1a0d358ba64225f6428 as selenite-controller

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /selenite-controller \
    ./cmd/selenite-controller

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /selenite-podman-executor \
    ./cmd/selenite-podman-executor

FROM altlinux.space/alt-atomic/core-minimal/nightly:20260323@sha256:0b0fef657304ca2531ba5729b5d0ee9c5cfb279d3ca3a7b784a52027521cf0ab as base

ARG DEV
ENV DEV=$DEV

RUN --mount=type=bind,source=./distro/base/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/base/resources,target=/etc/apm/resources \
    apm system image build

RUN --mount=type=bind,source=./distro/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/resources,target=/etc/apm/resources \
    apm system image build

COPY --from=selenite-controller /selenite-controller /usr/bin/selenite-controller
COPY --from=selenite-controller /selenite-podman-executor /usr/libexec/selenite-podman-executor
RUN chown root:_selenite-controller /usr/libexec/selenite-podman-executor && \
    chmod 4750 /usr/libexec/selenite-podman-executor

WORKDIR /
CMD ["/sbin/init"]

