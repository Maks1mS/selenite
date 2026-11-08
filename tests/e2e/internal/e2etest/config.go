// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TryApplyConfig sends cfg to `machine config apply`, returning output and
// error.
func (c *Client) TryApplyConfig(cfg string) (string, error) {
	return c.TryRun("machine config apply", strings.NewReader(cfg))
}

// ApplyConfig applies cfg, failing the test on error.
func (c *Client) ApplyConfig(t testing.TB, cfg string) string {
	t.Helper()
	out, err := c.TryApplyConfig(cfg)
	require.NoError(t, err, "machine config apply\noutput: %s", out)
	return out
}

// RequireConfigRejected requires that the config is rejected by validation.
func (c *Client) RequireConfigRejected(t testing.TB, cfg string) {
	t.Helper()
	out, err := c.TryApplyConfig(cfg)
	require.Error(t, err, "config should have been rejected\noutput: %s", out)
}
