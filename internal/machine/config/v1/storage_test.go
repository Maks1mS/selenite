// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/mo"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/internal/executil/mocks"
)

func TestStorageApplierValidate(t *testing.T) {
	ctx := context.Background()

	validMount := MountSpec{
		Type:   "nfs",
		Source: "192.168.2.152:/volume3/music/abooks",
		Target: "/var/mnt/abooks",
	}

	tests := []struct {
		name    string
		mounts  []MountSpec
		wantErr string
	}{
		{
			name:   "empty mounts - ok",
			mounts: nil,
		},
		{
			name:   "valid nfs mount - ok",
			mounts: []MountSpec{validMount},
		},
		{
			name: "valid with options - ok",
			mounts: []MountSpec{{
				Type:    "nfs",
				Source:  "192.168.2.152:/volume3/music/abooks",
				Target:  "/var/mnt/abooks",
				Options: mo.Some("ro,soft,timeo=150,retrans=3,nfsvers=4.1"),
			}},
		},
		{
			name: "hostname source - ok",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "nas.local:/volume1/video",
				Target: "/var/mnt/video",
			}},
		},
		{
			name: "valid disk mount - ok",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev/disk/by-label/data",
				Fs:     "ext4",
				Target: "/var/mnt/data",
			}},
		},
		{
			name: "valid disk mount with options - ok",
			mounts: []MountSpec{{
				Type:    "disk",
				Source:  "/dev/disk/by-uuid/1234-abcd",
				Fs:      "btrfs",
				Target:  "/var/mnt/data",
				Options: mo.Some("ro"),
			}},
		},
		{
			name: "disk source not a device path",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "vdb",
				Fs:     "ext4",
				Target: "/var/mnt/data",
			}},
			wantErr: "invalid source",
		},
		{
			name: "disk source is /dev itself",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev",
				Fs:     "ext4",
				Target: "/var/mnt/data",
			}},
			wantErr: "invalid source",
		},
		{
			name: "disk source with dotdot segment",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev/disk/../etc",
				Fs:     "ext4",
				Target: "/var/mnt/data",
			}},
			wantErr: "invalid source",
		},
		{
			name: "disk source with newline injection",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev/vdb\n[Service]\nExecStart=/bin/evil",
				Fs:     "ext4",
				Target: "/var/mnt/data",
			}},
			wantErr: "invalid source",
		},
		{
			name: "disk mount without fs",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev/disk/by-label/data",
				Target: "/var/mnt/data",
			}},
			wantErr: "fs is required",
		},
		{
			name: "disk mount with invalid fs",
			mounts: []MountSpec{{
				Type:   "disk",
				Source: "/dev/disk/by-label/data",
				Fs:     "ext4;rm -rf /",
				Target: "/var/mnt/data",
			}},
			wantErr: "invalid fs",
		},
		{
			name: "nfs mount with fs set",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume1/video",
				Fs:     "ext4",
				Target: "/var/mnt/video",
			}},
			wantErr: "fs is only valid",
		},
		{
			name: "unsupported type",
			mounts: []MountSpec{{
				Type:   "cifs",
				Source: "//192.168.2.152/video",
				Target: "/var/mnt/video",
			}},
			wantErr: "unsupported type",
		},
		{
			name: "empty type",
			mounts: []MountSpec{{
				Source: "192.168.2.152:/volume3/music/abooks",
				Target: "/var/mnt/abooks",
			}},
			wantErr: "unsupported type",
		},
		{
			name: "source without export path",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152",
				Target: "/var/mnt/abooks",
			}},
			wantErr: "invalid source",
		},
		{
			name: "source with space",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume1/my files",
				Target: "/var/mnt/abooks",
			}},
			wantErr: "invalid source",
		},
		{
			name: "source with newline injection",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume1/video\n[Service]\nExecStart=/bin/evil",
				Target: "/var/mnt/abooks",
			}},
			wantErr: "invalid source",
		},
		{
			name: "target outside /var/mnt",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume3/music/abooks",
				Target: "/etc/data",
			}},
			wantErr: "invalid target",
		},
		{
			name: "target is /var/mnt itself",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume3/music/abooks",
				Target: "/var/mnt",
			}},
			wantErr: "invalid target",
		},
		{
			name: "target with path traversal",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume3/music/abooks",
				Target: "/var/mnt/../etc",
			}},
			wantErr: "invalid target",
		},
		{
			name: "target starting with dash",
			mounts: []MountSpec{{
				Type:   "nfs",
				Source: "192.168.2.152:/volume3/music/abooks",
				Target: "/var/mnt/-bad",
			}},
			wantErr: "invalid target",
		},
		{
			name: "duplicate target",
			mounts: []MountSpec{
				validMount,
				{
					Type:   "nfs",
					Source: "192.168.2.152:/volume1/other",
					Target: "/var/mnt/abooks",
				},
			},
			wantErr: "duplicate target",
		},
		{
			name: "options with semicolon",
			mounts: []MountSpec{{
				Type:    "nfs",
				Source:  "192.168.2.152:/volume3/music/abooks",
				Target:  "/var/mnt/abooks",
				Options: mo.Some("ro;rm -rf /"),
			}},
			wantErr: "invalid options",
		},
		{
			name: "options with space",
			mounts: []MountSpec{{
				Type:    "nfs",
				Source:  "192.168.2.152:/volume3/music/abooks",
				Target:  "/var/mnt/abooks",
				Options: mo.Some("ro nosuid"),
			}},
			wantErr: "invalid options",
		},
		{
			name: "options with newline injection",
			mounts: []MountSpec{{
				Type:    "nfs",
				Source:  "192.168.2.152:/volume3/music/abooks",
				Target:  "/var/mnt/abooks",
				Options: mo.Some("ro\n[Service]\nExecStart=/bin/evil"),
			}},
			wantErr: "invalid options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewStorageApplier(StorageConfig{Mounts: tt.mounts}, StorageConfig{}, nil)
			err := a.Validate(ctx)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStorageApplierApply(t *testing.T) {
	ctx := context.Background()

	curr := StorageConfig{Mounts: []MountSpec{{
		Type:    "nfs",
		Source:  "192.168.2.152:/volume3/music/abooks",
		Target:  "/var/mnt/abooks",
		Options: mo.Some("ro"),
	}}}

	t.Run("no-op when config unchanged", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		require.NoError(t, NewStorageApplier(curr, curr, r).Apply(ctx))
	})

	t.Run("runs helper apply when config changes", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, storageHelperBin, []string{"apply"}, mock.Anything).
			Return(nil)

		require.NoError(t, NewStorageApplier(curr, StorageConfig{}, r).Apply(ctx))
	})

	t.Run("runs helper apply when section is removed", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, storageHelperBin, []string{"apply"}, mock.Anything).
			Return(nil)

		require.NoError(t, NewStorageApplier(StorageConfig{}, curr, r).Apply(ctx))
	})

	t.Run("helper error is propagated", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, storageHelperBin, []string{"apply"}, mock.Anything).
			Return(errors.New("helper failed"))

		err := NewStorageApplier(curr, StorageConfig{}, r).Apply(ctx)
		require.ErrorContains(t, err, "helper failed")
	})
}

func TestStorageApplierRollback(t *testing.T) {
	ctx := context.Background()

	prev := StorageConfig{Mounts: []MountSpec{{
		Type:   "nfs",
		Source: "192.168.2.152:/volume3/music/abooks",
		Target: "/var/mnt/abooks",
	}}}

	t.Run("restores previous mounts", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, storageHelperBin, []string{"apply"}, mock.Anything).
			Return(nil)

		require.NoError(t, NewStorageApplier(StorageConfig{}, prev, r).Rollback(ctx))
	})

	t.Run("error is propagated", func(t *testing.T) {
		r := mocks.NewMockRunner(t)
		r.EXPECT().
			Run(ctx, storageHelperBin, []string{"apply"}, mock.Anything).
			Return(errors.New("helper failed"))

		err := NewStorageApplier(StorageConfig{}, prev, r).Rollback(ctx)
		require.ErrorContains(t, err, "helper failed")
	})
}
