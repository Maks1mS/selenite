// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

const (
	testImageTag  = "selenite-e2e-image:latest"
	testImageMark = "hello-from-loaded-image"
)

func TestImage(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)
	const stackName = "image-test"

	archive := e2etest.SaveImage(t, testImageTag, fmt.Sprintf(
		`FROM registry.altlinux.org/sisyphus/distroless-toybox:latest
CMD ["sh", "-c", "echo %s && sleep 1000"]`, testImageMark))

	e2etest.Step(t, "load image", func(t *testing.T) {
		out, err := client.TryRun("infra image load", archive)
		require.NoError(t, err, "image load\noutput: %s", out)
		assert.Contains(t, out, "Loaded image")
	})

	e2etest.Step(t, "image list shows the loaded image", func(t *testing.T) {
		out := client.Run(t, "infra image list")
		assert.Contains(t, out, "selenite-e2e-image")
	})

	e2etest.Step(t, "stack runs from the loaded image without a registry", func(t *testing.T) {
		client.DeployStack(t, stackName, fmt.Sprintf(`services:
  test:
    image: %s
    pull_policy: never
    restart: unless-stopped
`, testImageTag))
		client.WaitStackRunning(t, stackName)
		time.Sleep(2 * time.Second)
		out := client.Run(t, "stack logs "+stackName)
		assert.Contains(t, out, testImageMark)
	})

	e2etest.Step(t, "image remove removes the image", func(t *testing.T) {
		client.Run(t, "stack remove "+stackName)
		client.Run(t, "infra image remove "+testImageTag)
		out := client.Run(t, "infra image list")
		assert.NotContains(t, out, "selenite-e2e-image")
	})
}
