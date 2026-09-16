#!/bin/bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

set -euo pipefail

FAILED=0

check_active() {
    local unit="$1"
    local state
    state=$(systemctl is-active "$unit" 2>/dev/null || true)
    if [ "$state" = "active" ]; then
        echo "OK: ${unit} is active"
    else
        echo "FAIL: ${unit} is ${state}"
        FAILED=1
    fi
}

check_active selenite-controller.service

if [ "$FAILED" -ne 0 ]; then
    exit 1
fi
