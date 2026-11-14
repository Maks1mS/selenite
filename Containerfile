FROM altlinux.space/alt-atomic/core-minimal/stable:20261114@sha256:c0fa9e3b663d93e6550ab73ce94b75a5232327c36e0a1d747ade9d886503141a as base

FROM base as builder

RUN apm s install go-task golang gcc libsystemd-devel

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION
ENV VERSION=$VERSION
ARG GIT_COMMIT
ENV GIT_COMMIT=$GIT_COMMIT

RUN task dist DESTDIR=/out VERSION=$VERSION GIT_COMMIT=$GIT_COMMIT

FROM base as root

RUN --mount=type=bind,source=./distro/base/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/base/resources,target=/etc/apm/resources \
    apm system image build

ARG DEV
ENV DEV=$DEV
ARG VERSION
ENV VERSION=$VERSION
ARG GIT_COMMIT
ENV GIT_COMMIT=$GIT_COMMIT

RUN --mount=type=bind,source=./distro/image.yml,target=/etc/apm/image.yml \
    --mount=type=bind,source=./distro/resources,target=/etc/apm/resources \
    apm system image build

COPY --from=builder /out/ /

RUN chown root:_selenite-controller /usr/libexec/selenite-controller/* && \
    chown root:_selenite-dashboard /usr/libexec/selenite-controller/selenite-dashboard-helper && \
    chown root:_selenite-dashboard /usr/bin/selenite-dashboard && \
    chmod 4750 /usr/libexec/selenite-controller/* && \
    chmod 755 /usr/libexec/selenite-storage-gate && \
    chmod 750 /usr/bin/selenite-dashboard

WORKDIR /
CMD ["/sbin/init"]
