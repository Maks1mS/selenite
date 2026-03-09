FROM registry.altlinux.org/sisyphus/golang:1.25.6@sha256:6e7d2dca3fa4dcd06f8896af7a081c8608b38d82cf4115d9852f14e137a78989 as selenite-controller

RUN apt-get update && apt-get -y install ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /selenite-controller \
    ./cmd/selenite-controller

FROM altlinux.space/alt-atomic/core/stable:20260305@sha256:0ca8633b0d8a7167ac486685c5e3bfbf65db82994d5087fd9f7e1d1244b0385e as base

ARG DEV
ENV DEV=$DEV

RUN --mount=type=bind,source=./distro/base/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/base/resources,target=/etc/apm/resources \
    apm system image build

RUN --mount=type=bind,source=./distro/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/resources,target=/etc/apm/resources \
    apm system image build

COPY --from=selenite-controller /selenite-controller /usr/bin/selenite-controller

WORKDIR /
CMD ["/sbin/init"]

