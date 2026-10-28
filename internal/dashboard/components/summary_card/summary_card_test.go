// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary_card

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitRestartsPolling(t *testing.T) {
	t.Parallel()

	m := New(nil)
	require.NotNil(t, m.Init())
	stale := tickMsg{generation: m.generation}

	require.NotNil(t, m.Init(), "reactivation must restart polling even if the previous tick was lost")

	_, cmd := m.Update(stale)
	require.Nil(t, cmd, "a tick from a previous polling chain must be ignored")

	_, cmd = m.Update(tickMsg{generation: m.generation})
	require.NotNil(t, cmd, "a tick from the current polling chain must keep polling")
}
