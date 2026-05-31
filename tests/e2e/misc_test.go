// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiscCommandsWorks(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	// run(t, "version works", func(t *testing.T) {
	// 	out := mustRunCmd(t, client, "version")
	// 	assert.Contains(t, out, "Version:")
	// 	assert.Contains(t, out, "GitCommit:")
	// 	assert.Contains(t, out, "Go:")
	// })

	run(t, "help works", func(t *testing.T) {
		out := mustRunCmd(t, client, "help")
		assert.Contains(t, out, "Selenite is an immutable server OS built around podman compose stacks")
	})
}
