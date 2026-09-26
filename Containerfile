FROM altlinux.space/alt-atomic/core-minimal/stable:20260926@sha256:e9eb51243be140c425f0568789f0acf5fe7090cfa55830171d7d520f799a35cc as base

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
    chmod 750 /usr/bin/selenite-dashboard

FROM altlinux.space/alt-atomic/sbom-tool:v0.2.0@sha256:7403d01ad02d80c2948944bf30e921d815e7b0843071e76b440eb707c09cbf5d AS sbom
RUN --mount=type=bind,from=root,source=/,target=/rootfs,rw generate-sbom

FROM root AS final
COPY --from=sbom /sbom.cdx.json /usr/share/doc/atomic-sbom.cdx.json
WORKDIR /
CMD ["/sbin/init"]
