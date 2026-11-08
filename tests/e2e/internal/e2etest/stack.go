// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TryDeployStack sends composeYAML via stdin to `stack deploy <name>`,
// returning stderr and the error.
func (c *Client) TryDeployStack(name, composeYAML string) (string, error) {
	var buf strings.Builder
	err := c.Exec("stack deploy "+name,
		WithStdin(strings.NewReader(composeYAML)),
		WithStderr(&buf))
	return buf.String(), err
}

// DeployStack deploys a compose stack, failing the test on error.
func (c *Client) DeployStack(t testing.TB, name, composeYAML string) {
	t.Helper()
	out, err := c.TryDeployStack(name, composeYAML)
	require.NoError(t, err, "stack deploy %q (out: %q)", name, out)
}

// WaitStackRunning waits until a container of the stack is up.
func (c *Client) WaitStackRunning(t testing.TB, stackName string) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		out, err := c.TryRun("stack ps "+stackName, nil)
		if err == nil && strings.Contains(out, "Up ") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("stack %q did not come up in time\nlast output: %s\nlast error: %v", stackName, out, err)
		}
		time.Sleep(2 * time.Second)
	}
}
