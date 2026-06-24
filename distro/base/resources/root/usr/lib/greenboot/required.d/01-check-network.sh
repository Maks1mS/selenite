#!/bin/bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.


set -euo pipefail

TIMEOUT=10
RETRIES=3
HOSTS=()

BOOTC_IMAGE=$(bootc status --format json 2>/dev/null | jq -r '.status.booted.image.image.image // empty')
if [ -n "$BOOTC_IMAGE" ]; then
    REGISTRY_HOST=$(echo "$BOOTC_IMAGE" | cut -d'/' -f1)
    echo "Detected bootc registry: ${REGISTRY_HOST}"
    HOSTS+=("$REGISTRY_HOST")
fi

HOSTS+=(
    "ya.ru"
    "vk.ru"
)

check_host() {
    local host="$1"

    for i in $(seq 1 $RETRIES); do
        if curl --silent --fail --max-time "$TIMEOUT" "https://${host}" > /dev/null 2>&1; then
            return 0
        fi
        sleep 2
    done

    return 1
}

for host in "${HOSTS[@]}"; do
    echo "Checking ${host}..."
    if check_host "$host"; then
        echo "OK: ${host} is reachable, connectivity confirmed"
        exit 0
    fi
    echo "FAIL: ${host} is not reachable"
done

echo "FAIL: no hosts reachable"
exit 1
