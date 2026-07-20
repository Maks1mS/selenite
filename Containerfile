# FROM registry.altlinux.org/sisyphus/golang:1.25.6@sha256:6e7d2dca3fa4dcd06f8896af7a081c8608b38d82cf4115d9852f14e137a78989 as selenite-controller
# RUN apt-get update && apt-get -y install ca-certificates
FROM golang:1.26.5@sha256:63f132d58c1f589f0dcda584933a9bb44bfda1150f1506377f5a902f34d86033 as selenite-controller

ARG VERSION
ENV VERSION=$VERSION
ARG GIT_COMMIT
ENV GIT_COMMIT=$GIT_COMMIT

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
        podman-network-helper \
        selenite-machine-helper \
        sysctl-helper \
        netbird-helper \
        alloy-helper; \
    do \
        CGO_ENABLED=1 \
        GOOS=linux \
        go build -ldflags="-s -w \
            -X altlinux.space/alt-atomic/selenite/internal/version.Version=$VERSION \
            -X altlinux.space/alt-atomic/selenite/internal/version.GitCommit=$GIT_COMMIT" \
            -trimpath -o /out/$cmd ./cmd/$cmd; \
    done

FROM altlinux.space/alt-atomic/core-minimal/stable:20260720@sha256:e69b121306c015a36a8710509d94562e45919529e0ed53b3cd535d0e7121fa37

ARG DEV
ENV DEV=$DEV
ARG VERSION
ENV VERSION=$VERSION
ARG GIT_COMMIT
ENV GIT_COMMIT=$GIT_COMMIT

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
        podman-network-helper \
        selenite-machine-helper \
        sysctl-helper \
        netbird-helper \
        alloy-helper; \
    do \
        install -o root -g _selenite-controller -m 4750 /out/$cmd /usr/libexec/selenite-controller/$cmd; \
    done

WORKDIR /
CMD ["/sbin/init"]
