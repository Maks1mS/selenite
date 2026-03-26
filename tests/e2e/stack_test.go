//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testComposeYAML = `services:
  test:
    image: docker.io/library/busybox:latest
    command: sleep 3600
    restart: unless-stopped
`

func TestStack(t *testing.T) {
	t.Parallel()
	client := bootVM(t)
	const stackName = "test-app"

	run(t, "initial list is empty", func(t *testing.T) {
		out := mustRunCmd(t, client, "stack list")
		assert.Equal(t, "no stacks deployed\n", out)
	})

	run(t, "deploy creates stack", func(t *testing.T) {
		mustDeployStack(t, client, stackName, testComposeYAML)
		out := mustRunCmd(t, client, "stack list")
		require.Contains(t, out, stackName)
	})

	run(t, "remove removes stack", func(t *testing.T) {
		mustRunCmd(t, client, "stack remove "+stackName)
		out := mustRunCmd(t, client, "stack list")
		require.Equal(t, "no stacks deployed\n", out)
	})
}
