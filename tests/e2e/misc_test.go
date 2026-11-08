// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

func TestMiscCommandsWorks(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)

	// e2etest.Step(t, "version works", func(t *testing.T) {
	// 	out := client.Run(t, "version")
	// 	assert.Contains(t, out, "Version:")
	// 	assert.Contains(t, out, "GitCommit:")
	// 	assert.Contains(t, out, "Go:")
	// })

	e2etest.Step(t, "help works", func(t *testing.T) {
		out := client.Run(t, "help")
		assert.Contains(t, out, "Selenite is an immutable server OS built around podman compose stacks")
	})

	e2etest.Step(t, "debug requires PTY", func(t *testing.T) {
		out, err := client.TryRun("debug quay.io/quay/busybox", nil)
		require.Error(t, err)
		assert.Contains(t, out, "requires an active PTY")
	})

	e2etest.Step(t, "debug requires image", func(t *testing.T) {
		out, err := client.TryRunPTY("debug")
		require.Error(t, err)
		assert.Contains(t, out, "image is required")
	})

	e2etest.Step(t, "debug runs a command in container with /host mounted", func(t *testing.T) {
		out, err := client.TryRunPTY("debug quay.io/quay/busybox cat /host/etc/os-release")
		require.NoError(t, err, "output: %s", out)
		assert.Contains(t, strings.TrimSpace(out), "Selenite")
	})
}
