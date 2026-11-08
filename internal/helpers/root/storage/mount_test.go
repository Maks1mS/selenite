// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	t.Run("empty input is an empty list", func(t *testing.T) {
		t.Parallel()
		mounts, err := Decode(strings.NewReader(""))
		require.NoError(t, err)
		require.Empty(t, mounts)
	})

	t.Run("whitespace input is an empty list", func(t *testing.T) {
		t.Parallel()
		mounts, err := Decode(strings.NewReader("\n  \n"))
		require.NoError(t, err)
		require.Empty(t, mounts)
	})

	t.Run("valid wire format", func(t *testing.T) {
		t.Parallel()
		mounts, err := Decode(strings.NewReader(
			`[{"type":"nfs","source":"nas.local:/video","target":"/var/mnt/video","options":"ro"}]`,
		))
		require.NoError(t, err)
		require.Equal(t, []Mount{{
			Type:    "nfs",
			Source:  "nas.local:/video",
			Target:  "/var/mnt/video",
			Options: "ro",
		}}, mounts)
	})

	t.Run("invalid JSON is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := Decode(strings.NewReader(`[{"type":`))
		require.ErrorContains(t, err, "parse mounts")
	})

	t.Run("read errors are propagated", func(t *testing.T) {
		t.Parallel()
		_, err := Decode(errReader{})
		require.ErrorContains(t, err, "read input")
	})
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mounts  []Mount
		wantErr string
	}{
		{
			name:   "empty set - ok",
			mounts: nil,
		},
		{
			name: "nfs and disk mounts - ok",
			mounts: []Mount{
				{Type: "nfs", Source: "nas.local:/volume1/video", Target: "/var/mnt/video", Options: "ro,soft"},
				{Type: "disk", Source: "/dev/disk/by-label/data", Target: "/var/mnt/data", Fs: "ext4"},
			},
		},
		{
			name:    "unsupported type",
			mounts:  []Mount{{Type: "cifs", Source: "//nas/video", Target: "/var/mnt/video"}},
			wantErr: "unsupported type",
		},
		{
			name:    "nfs source without export path",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local", Target: "/var/mnt/video"}},
			wantErr: "invalid source",
		},
		{
			name:    "nfs source with newline injection",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local:/video\n[Service]\nExecStart=/bin/evil", Target: "/var/mnt/video"}},
			wantErr: "invalid source",
		},
		{
			name:    "nfs mount with fs",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/video", Fs: "ext4"}},
			wantErr: "fs is only valid",
		},
		{
			name:    "disk source not a device path",
			mounts:  []Mount{{Type: "disk", Source: "vdb", Target: "/var/mnt/data", Fs: "ext4"}},
			wantErr: "invalid source",
		},
		{
			name:    "disk source with dotdot segment",
			mounts:  []Mount{{Type: "disk", Source: "/dev/disk/../etc", Target: "/var/mnt/data", Fs: "ext4"}},
			wantErr: "invalid source",
		},
		{
			name:    "disk mount without fs",
			mounts:  []Mount{{Type: "disk", Source: "/dev/vdb", Target: "/var/mnt/data"}},
			wantErr: "fs is required",
		},
		{
			name:    "disk mount with invalid fs",
			mounts:  []Mount{{Type: "disk", Source: "/dev/vdb", Target: "/var/mnt/data", Fs: "ext4;rm -rf /"}},
			wantErr: "invalid fs",
		},
		{
			name:    "target outside /var/mnt",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local:/video", Target: "/etc/video"}},
			wantErr: "invalid target",
		},
		{
			name:    "target with path traversal",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/../etc"}},
			wantErr: "invalid target",
		},
		{
			name: "duplicate target",
			mounts: []Mount{
				{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/data"},
				{Type: "nfs", Source: "nas.local:/other", Target: "/var/mnt/data"},
			},
			wantErr: "duplicate target",
		},
		{
			name:    "options with newline injection",
			mounts:  []Mount{{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/video", Options: "ro\n[Service]\nExecStart=/bin/evil"}},
			wantErr: "invalid options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(tt.mounts, "storage.mounts")
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestValidateErrorPrefix(t *testing.T) {
	t.Parallel()

	bad := []Mount{
		{Type: "nfs", Source: "nas.local:/video", Target: "/var/mnt/video"},
		{Type: "nfs", Source: "nas.local:/other", Target: "/etc/other"},
	}

	require.ErrorContains(t, Validate(bad, "storage.mounts"), "storage.mounts[1]:")
	require.ErrorContains(t, Validate(bad, "mounts"), "mounts[1]:")
}
