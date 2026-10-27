// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const gib = 1024 * 1024 * 1024

func TestFormatUptime(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{999 * time.Millisecond, "0s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m0s"},
		{time.Hour + 5*time.Second, "1h0m5s"},
		{24*time.Hour - time.Second, "23h59m59s"},
		{24 * time.Hour, "1d0h0m0s"},
		{3*24*time.Hour + 4*time.Hour + 5*time.Minute + 6*time.Second, "3d4h5m6s"},
	} {
		require.Equal(t, tc.want, formatUptime(tc.in), "formatUptime(%s)", tc.in)
	}
}

func TestFormatMem(t *testing.T) {
	t.Parallel()

	require.Equal(t, "0.5/2.0 GB (25.0%)", formatMem(gib/2, 2*gib, 25))
}

func TestFormatSwapPart(t *testing.T) {
	t.Parallel()

	require.Empty(t, formatSwapPart(0, 0, 0), "no swap must hide the SWAP part")
	require.Equal(t, ", SWAP 1.0/4.0 GB (25.0%)", formatSwapPart(4*gib, gib, 25))
}

func TestFormatCPUInfo(t *testing.T) {
	t.Parallel()

	require.Equal(t, "4x2.4GHz", formatCPUInfo(4, 2.4))
	require.Equal(t, "2 cores", formatCPUInfo(2, 0), "unknown frequency")
}
