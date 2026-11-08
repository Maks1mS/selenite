// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import "testing"

// Step runs a named sub-step; if it fails, the parent test is aborted.
// Use for ordered sequential steps where step N+1 depends on step N.
func Step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if !t.Run(name, fn) {
		t.Fatal("step failed, aborting")
	}
}
