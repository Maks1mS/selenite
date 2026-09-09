#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE_TAG="selenite-tmp"
WORK_DIR="${WORK_DIR:-/var/tmp/selenite}"
DISK_IMAGE="disk.raw"
DISK_SIZE="4G"

info()  { echo "=== $*"; }
die()   { echo "ERROR: $*" >&2; exit 1; }

FORCE="${FORCE:-0}"
if [[ "${1:-}" == "--force" ]]; then FORCE=1; fi

SUDO=""
if [[ "$(id -u)" -ne 0 ]]; then
    SUDO="sudo"
fi

info "Building image"
podman build \
    --tag "$IMAGE_TAG" \
    --build-arg DEV=1 \
    --build-arg VERSION="$(git describe --tags 2>/dev/null || git rev-parse --short HEAD)" \
    --build-arg GIT_COMMIT="$(git rev-parse HEAD)" \
    "$SCRIPT_DIR/../"

IMAGE_ID=$(podman image inspect --format '{{.Id}}' "$IMAGE_TAG")
SCRIPT_HASH=$(sha256sum "$0" | awk '{print $1}')
CACHE_KEY="${IMAGE_ID}:${SCRIPT_HASH}"
CACHE_FILE="$WORK_DIR/$DISK_IMAGE.cache-key"

mkdir -p "$WORK_DIR"
if [[ "$FORCE" != "1" && -f "$WORK_DIR/$DISK_IMAGE" && -f "$CACHE_FILE" ]]; then
    if [[ "$CACHE_KEY" == "$(cat "$CACHE_FILE")" ]]; then
        info "Disk image is up to date, skipping rebuild"
        exit 0
    fi
fi

if [[ -n "$SUDO" ]]; then
    info "Transferring image to root storage"
    podman save "$IMAGE_TAG" | $SUDO podman load
fi

info "Installing to disk"
$SUDO rm -f "$WORK_DIR/$DISK_IMAGE"
$SUDO truncate -s "$DISK_SIZE" "$WORK_DIR/$DISK_IMAGE"
$SUDO podman run --rm --privileged --pid=host \
    -v /var/lib/containers:/var/lib/containers \
    -v /dev:/dev \
    --security-opt label=type:unconfined_t \
    -e LANG=C.UTF-8 \
    -v "$WORK_DIR:/output" \
    "$IMAGE_TAG" \
    bootc install to-disk \
        --generic-image \
        --via-loopback \
        --filesystem btrfs \
        --disable-selinux \
        "/output/$DISK_IMAGE"

$SUDO bash -c "echo \"$CACHE_KEY\" > \"$CACHE_FILE\""

$SUDO chmod 666 "$WORK_DIR/$DISK_IMAGE"
$SUDO chmod 666 "$CACHE_FILE"
