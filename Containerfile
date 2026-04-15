# FROM registry.altlinux.org/sisyphus/golang:1.25.6@sha256:6e7d2dca3fa4dcd06f8896af7a081c8608b38d82cf4115d9852f14e137a78989 as selenite-controller
# RUN apt-get update && apt-get -y install ca-certificates
FROM golang:1.26.2@sha256:b53c282df83967299380adbd6a2dc67e750a58217f39285d6240f6f80b19eaad as selenite-controller

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /selenite-controller \
    ./cmd/selenite-controller

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /podman-compose-helper \
    ./cmd/podman-compose-helper

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /podman-secret-helper \
    ./cmd/podman-secret-helper

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /podman-volume-helper \
    ./cmd/podman-volume-helper

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /selenite-machine-executor \
    ./cmd/selenite-machine-executor

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /sysctl-helper \
    ./cmd/sysctl-helper

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /netbird-helper \
    ./cmd/netbird-helper    

FROM altlinux.space/alt-atomic/core-minimal/nightly:20260415@sha256:20e260eab949bee29c3404bf6f2a072680bbf9df8dc1388d6bc822abfbbf5b32

ARG DEV
ENV DEV=$DEV

RUN --mount=type=bind,source=./distro/base/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/base/resources,target=/etc/apm/resources \
    apm system image build

RUN --mount=type=bind,source=./distro/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/resources,target=/etc/apm/resources \
    apm system image build

COPY --from=selenite-controller /selenite-controller /usr/bin/selenite-controller

COPY --from=selenite-controller /podman-compose-helper /usr/libexec/selenite-controller/podman-compose-helper
RUN chown root:_selenite-controller /usr/libexec/selenite-controller/podman-compose-helper && \
    chmod 4750 /usr/libexec/selenite-controller/podman-compose-helper

COPY --from=selenite-controller /podman-secret-helper /usr/libexec/selenite-controller/podman-secret-helper
RUN chown root:_selenite-controller /usr/libexec/selenite-controller/podman-secret-helper && \
    chmod 4750 /usr/libexec/selenite-controller/podman-secret-helper

COPY --from=selenite-controller /podman-volume-helper /usr/libexec/selenite-controller/podman-volume-helper
RUN chown root:_selenite-controller /usr/libexec/selenite-controller/podman-volume-helper && \
    chmod 4750 /usr/libexec/selenite-controller/podman-volume-helper

COPY --from=selenite-controller /selenite-machine-executor /usr/libexec/selenite-machine-executor
RUN chown root:_selenite-controller /usr/libexec/selenite-machine-executor && \
    chmod 4750 /usr/libexec/selenite-machine-executor

COPY --from=selenite-controller /sysctl-helper /usr/libexec/selenite-controller/sysctl-helper
RUN chown root:_selenite-controller /usr/libexec/selenite-controller/sysctl-helper && \
    chmod 4750 /usr/libexec/selenite-controller/sysctl-helper

COPY --from=selenite-controller /netbird-helper /usr/libexec/selenite-controller/netbird-helper
RUN chown root:_selenite-controller /usr/libexec/selenite-controller/netbird-helper && \
    chmod 4750 /usr/libexec/selenite-controller/netbird-helper

WORKDIR /
CMD ["/sbin/init"]

