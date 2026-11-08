// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"bytes"
	"embed"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

const testComposeYAML = `services:
  test:
    image: registry.altlinux.org/sisyphus/distroless-toybox:latest
    command: sh -c 'echo start-log-test && while true; do date; sleep 1; done'
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
	client := e2etest.BootVMProvisioned(t)
	const stackName = "test-app"

	e2etest.Step(t, "initial list is empty", func(t *testing.T) {
		out := client.Run(t, "stack list")
		assert.Equal(t, "no stacks deployed\n", out)
	})

	e2etest.Step(t, "initial secret list is empty", func(t *testing.T) {
		out := client.Run(t, "infra secret list")
		assert.Equal(t, "ID          NAME        DRIVER      CREATED     UPDATED\n", out)
	})

	e2etest.Step(t, "add secret", func(t *testing.T) {
		client.RunStdin(t, "infra secret create test_secret", strings.NewReader("Super secret string"))
	})

	e2etest.Step(t, "secret added", func(t *testing.T) {
		out := client.Run(t, "infra secret list")
		assert.Contains(t, out, "test_secret")
	})

	e2etest.Step(t, "deploy creates stack", func(t *testing.T) {
		client.DeployStack(t, stackName, testComposeYAML)
		out := client.Run(t, "stack list")
		require.Contains(t, out, stackName)
	})

	e2etest.Step(t, "stack logs returns output", func(t *testing.T) {
		// Give the container a moment to emit the startup log line.
		time.Sleep(2 * time.Second)
		out := client.Run(t, "stack logs "+stackName)
		assert.Contains(t, out, "start-log-test")
	})

	e2etest.Step(t, "import volume", func(t *testing.T) {
		volumeData, err := testdataFS.ReadFile("testdata/volume.tar.zst")
		require.NoError(t, err)

		err = client.Exec(
			"infra volume import my_test_volume",
			e2etest.WithStdin(bytes.NewReader(volumeData)),
		)
		require.NoError(t, err)
	})

	e2etest.Step(t, "export volume", func(t *testing.T) {
		volumeData, err := testdataFS.ReadFile("testdata/volume.tar.zst")
		require.NoError(t, err)

		var buf bytes.Buffer

		err = client.Exec(
			"infra volume export my_test_volume",
			e2etest.WithStdout(&buf),
		)
		require.NoError(t, err)
		require.Equal(t, buf.Bytes(), volumeData)
	})

	e2etest.Step(t, "remove removes stack", func(t *testing.T) {
		client.Run(t, "stack remove "+stackName)
		out := client.Run(t, "stack list")
		require.Equal(t, "no stacks deployed\n", out)
	})
}
