// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnitName(t *testing.T) {
	t.Parallel()

	// Mirrors `systemd-escape --path --suffix=mount`.
	tests := []struct {
		target string
		want   string
	}{
		{target: "/var/mnt/video", want: "var-mnt-video.mount"},
		{target: "/var/mnt/data", want: "var-mnt-data.mount"},
		{target: "/var/mnt/my-data", want: `var-mnt-my\x2ddata.mount`},
		{target: "/var/mnt/foo-", want: `var-mnt-foo\x2d.mount`},
		{target: "/var/mnt/abooks_v2", want: "var-mnt-abooks_v2.mount"},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, UnitName(tt.target))
		})
	}
}

func TestUnitContent(t *testing.T) {
	t.Parallel()

	t.Run("nfs without options", func(t *testing.T) {
		t.Parallel()
		content := UnitContent(Mount{
			Type:   "nfs",
			Source: "nas.local:/volume1/video",
			Target: "/var/mnt/video",
		})
		require.Contains(t, content, marker+"\n")
		require.Contains(t, content, "Description=Selenite storage mount nas.local:/volume1/video at /var/mnt/video\n")
		require.Contains(t, content, "What=nas.local:/volume1/video\n")
		require.Contains(t, content, "Where=/var/mnt/video\n")
		require.Contains(t, content, "Type=nfs\n")
		require.NotContains(t, content, "Options=")
		require.Contains(t, content, "WantedBy=remote-fs.target\n")
	})

	t.Run("nfs with options", func(t *testing.T) {
		t.Parallel()
		content := UnitContent(Mount{
			Type:    "nfs",
			Source:  "nas.local:/volume1/video",
			Target:  "/var/mnt/video",
			Options: "ro,soft",
		})
		require.Contains(t, content, "Options=ro,soft\n")
	})

	t.Run("disk uses fs as the type", func(t *testing.T) {
		t.Parallel()
		content := UnitContent(Mount{
			Type:   "disk",
			Source: "/dev/vdb",
			Target: "/var/mnt/data",
			Fs:     "ext4",
		})
		require.Contains(t, content, "What=/dev/vdb\n")
		require.Contains(t, content, "Type=ext4\n")
		require.NotContains(t, content, "Type=disk")
	})
}

func TestGateDropinContent(t *testing.T) {
	t.Parallel()

	content := GateDropinContent([]Mount{
		{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/video"},
		{Type: "disk", Source: "/dev/vdb", Target: "/var/mnt/data", Fs: "ext4"},
	}, "/usr/libexec/selenite-storage-gate")

	require.Contains(t, content, marker+"\n")
	require.Contains(t, content,
		"ExecStartPre=/usr/libexec/selenite-storage-gate /var/mnt/data /var/mnt/video\n")
	require.Contains(t, content, "Restart=on-failure\n")
	require.Contains(t, content, "RestartSec=60s\n")
}
