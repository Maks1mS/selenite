# FROM registry.altlinux.org/sisyphus/golang:1.25.6@sha256:6e7d2dca3fa4dcd06f8896af7a081c8608b38d82cf4115d9852f14e137a78989 as selenite-controller
# RUN apt-get update && apt-get -y install ca-certificates
FROM golang:1.26.2@sha256:b53c282df83967299380adbd6a2dc67e750a58217f39285d6240f6f80b19eaad as selenite-controller

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir /out && \
    for cmd in \
        selenite-controller \
        podman-compose-helper \
        podman-secret-helper \
        podman-volume-helper \
        selenite-machine-helper \
        sysctl-helper \
        netbird-helper; \
    do \
        CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -trimpath -o /out/$cmd ./cmd/$cmd; \
    done

FROM altlinux.space/alt-atomic/core-minimal/nightly:20260416@sha256:d24116b8461da6dd7fd51d1ba65d83d7a6b7337b8692169aef52fc81706bae4d

ARG DEV
ENV DEV=$DEV

RUN --mount=type=bind,source=./distro/base/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/base/resources,target=/etc/apm/resources \
    apm system image build

RUN --mount=type=bind,source=./distro/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/resources,target=/etc/apm/resources \
    apm system image build

COPY --from=selenite-controller /out/selenite-controller /usr/bin/selenite-controller
RUN --mount=from=selenite-controller,source=/out,target=/out \
    for cmd in \
        podman-compose-helper \
        podman-secret-helper \
        podman-volume-helper \
        selenite-machine-helper \
        sysctl-helper \
        netbird-helper; \
    do \
        install -o root -g _selenite-controller -m 4750 /out/$cmd /usr/libexec/selenite-controller/$cmd; \
    done

WORKDIR /
CMD ["/sbin/init"]

