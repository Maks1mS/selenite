// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Debug runs cmd inside the debug busybox container (`selenite debug`); the
// host filesystem is visible as /host.
func (c *Client) Debug(t testing.TB, cmd string) string {
	t.Helper()
	full := "debug quay.io/quay/busybox -- " + cmd
	out, err := c.TryRunPTY(full)
	require.NoError(t, err, "debug command %q\noutput: %s", full, out)
	return out
}

// HostCmd runs cmd on the host (inside chroot /host) and returns its output.
func (c *Client) HostCmd(t testing.TB, cmd string) string {
	t.Helper()
	const hostOutputMarker = "--- host output ---"

	out := c.Debug(t,
		fmt.Sprintf(`sh -c "echo '%s'; chroot /host %s"`, hostOutputMarker, cmd))
	_, after, found := strings.Cut(out, hostOutputMarker)
	require.True(t, found, "host command %q: marker not found in output: %s", cmd, out)
	return PtyText(after)
}

// HostCmdInt runs cmd on the host and parses its output as an integer.
func (c *Client) HostCmdInt(t testing.TB, cmd string) int64 {
	t.Helper()
	out := c.HostCmd(t, cmd)
	n, err := strconv.ParseInt(out, 10, 64)
	require.NoError(t, err, "host command %q: not a number: %q", cmd, out)
	return n
}

// HostFile returns the content of an absolute host file (e.g.
// /etc/systemd/system/foo.mount).
func (c *Client) HostFile(t testing.TB, file string) string {
	t.Helper()
	return PtyText(c.Debug(t, "cat "+path.Join("/host", file)))
}

// HostFileExists reports whether a file exists on the host.
func (c *Client) HostFileExists(t testing.TB, file string) bool {
	t.Helper()
	out := PtyText(c.Debug(t,
		fmt.Sprintf(`sh -c "test -f %s && echo yes || echo no"`, path.Join("/host", file))))
	return out == "yes"
}

// RequireHostFileContains requires that the host file at file contains every
// substring in want.
func (c *Client) RequireHostFileContains(t testing.TB, file string, want ...string) {
	t.Helper()
	out := c.HostFile(t, file)
	for _, w := range want {
		require.Contains(t, out, w, "host file %q", file)
	}
}

// RequireHostUnitState requires that the systemd unit on the host is in the
// state want, e.g. "active" or "failed".
func (c *Client) RequireHostUnitState(t testing.TB, unit, want string) {
	t.Helper()
	// is-active exits non-zero for non-active states; keep its output.
	out := PtyText(c.Debug(t,
		fmt.Sprintf(`sh -c "chroot /host /usr/bin/systemctl is-active %s; true"`, unit)))
	require.Contains(t, strings.Split(out, "\n"), want, "host unit %q state, output: %s", unit, out)
}
