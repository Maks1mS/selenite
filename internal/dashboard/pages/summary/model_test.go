// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package summary

import (
	"testing"

	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/dashboard/pages/base"
)

func TestActivationRestartsJournalPolling(t *testing.T) {
	t.Parallel()

	m := New(nil, nil)
	var activated base.ActivatedMsg

	for range 2 {
		model, cmd := m.Update(activated)
		require.Same(t, m, model)
		require.NotNil(t, cmd)
	}
}
