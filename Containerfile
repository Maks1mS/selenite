FROM altlinux.space/alt-atomic/core-minimal/stable:20261127@sha256:fef2fbd80c3ed78e579b3b7084d3069750bb1f21f7c513170f30726b73b45b80 as base

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
