//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiscCommandsWorks(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)

	run(t, "version works", func(t *testing.T) {
		out := mustRunCmd(t, client, "version")
		assert.Contains(t, out, "Version:")
		assert.Contains(t, out, "GitCommit:")
		assert.Contains(t, out, "Go:")
	})

	run(t, "help works", func(t *testing.T) {
		out := mustRunCmd(t, client, "help")
		assert.Contains(t, out, "Selenite is an immutable server OS built around podman compose stacks")
	})
}
