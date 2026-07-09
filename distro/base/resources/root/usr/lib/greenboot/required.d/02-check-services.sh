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

check_not_failed() {
    local unit="$1"
    local state
    state=$(systemctl is-failed "$unit" 2>/dev/null || true)
    if [ "$state" = "failed" ]; then
        echo "FAIL: ${unit} is in failed state"
        FAILED=1
    else
        echo "OK: ${unit} is not failed (${state})"
    fi
}

check_active selenite-controller.service
# selenite-setup runs only on first boot, so we only verify it hasn't failed
check_not_failed selenite-setup.service
check_not_failed selenite-etc-restore.service
check_not_failed selenite-reconcile.service

if [ "$FAILED" -ne 0 ]; then
    exit 1
fi
