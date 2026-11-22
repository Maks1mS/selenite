// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SaveImage builds an image from dockerfile on the test host with podman and
// returns an open image archive for upload via `image load`. The host image is
// removed on cleanup. Requires podman and network access on the test host.
func SaveImage(t testing.TB, tag, dockerfile string) *os.File {
	t.Helper()
	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Containerfile")
	require.NoError(t, os.WriteFile(dockerfilePath, []byte(dockerfile), 0o600))

	runPodman := func(args ...string) {
		t.Helper()
		out, err := exec.Command("podman", args...).CombinedOutput() //nolint:gosec // test helper arguments are constants
		require.NoError(t, err, "podman %s: %s", strings.Join(args, " "), out)
	}

	archive := filepath.Join(dir, "image.tar")
	runPodman("build", "--tag", tag, "--file", dockerfilePath, dir)
	runPodman("save", "--output", archive, tag)
	t.Cleanup(func() {
		_ = exec.Command("podman", "rmi", "--force", tag).Run() //nolint:gosec // best-effort cleanup of a test image
	})

	f, err := os.Open(archive) //nolint:gosec // path is inside t.TempDir()
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}
