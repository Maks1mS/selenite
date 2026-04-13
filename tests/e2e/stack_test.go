//go:build e2e

package e2e

import (
	"bytes"
	"embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testComposeYAML = `services:
  test:
    image: docker.io/library/busybox:latest
    command: sleep 3600
    restart: unless-stopped
    secrets:
      - test_secret
    volumes:
      my_test_volume:/volume

volumes:
  my_test_volume:
    name: "my_test_volume"

secrets:
  test_secret:
    external: true
`

//go:embed testdata/*
var testdataFS embed.FS

func TestStack(t *testing.T) {
	t.Parallel()
	client := bootVMWithProvison(t)
	const stackName = "test-app"

	run(t, "initial list is empty", func(t *testing.T) {
		out := mustRunCmd(t, client, "stack list")
		assert.Equal(t, "no stacks deployed\n", out)
	})

	run(t, "initial secret list is empty", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra secret list")
		assert.Equal(t, "ID          NAME        DRIVER      CREATED     UPDATED\n", out)
	})

	run(t, "add secret", func(t *testing.T) {
		mustRunCmdWithStdin(t, client, "infra secret create test_secret", strings.NewReader("Super secret string"))
	})

	run(t, "secret added", func(t *testing.T) {
		out := mustRunCmd(t, client, "infra secret list")
		assert.Contains(t, out, "test_secret")
	})

	run(t, "deploy creates stack", func(t *testing.T) {
		mustDeployStack(t, client, stackName, testComposeYAML)
		out := mustRunCmd(t, client, "stack list")
		require.Contains(t, out, stackName)
	})

	run(t, "import volume", func(t *testing.T) {
		volumeData, err := testdataFS.ReadFile("testdata/volume.tar.zst")
		require.NoError(t, err)

		err = sshRun(
			client.ssh,
			"infra volume import my_test_volume",
			WithStdin(bytes.NewReader(volumeData)),
		)
		require.NoError(t, err)
	})

	run(t, "export volume", func(t *testing.T) {
		volumeData, err := testdataFS.ReadFile("testdata/volume.tar.zst")
		require.NoError(t, err)

		var buf bytes.Buffer

		err = sshRun(
			client.ssh,
			"infra volume export my_test_volume",
			WithStdout(&buf),
		)
		require.NoError(t, err)
		require.Equal(t, buf.Bytes(), volumeData)
	})

	run(t, "remove removes stack", func(t *testing.T) {
		mustRunCmd(t, client, "stack remove "+stackName)
		out := mustRunCmd(t, client, "stack list")
		require.Equal(t, "no stacks deployed\n", out)
	})
}
