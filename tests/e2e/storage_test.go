// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

const (
	mountUnitPath  = "/etc/systemd/system/var-mnt-video.mount"
	gateDropInPath = "/var/lib/selenite-podman-executor/.config/systemd/user/podman-restart.service.d/10-selenite-storage.conf"
)

func storageConfig(client *e2etest.Client, storage string) string {
	keyStr := strings.TrimRight(string(gossh.MarshalAuthorizedKey(client.PublicKey())), "\n")
	cfg := fmt.Sprintf(`
access:
  users:
    - name: provisioner
      roles:
        - root
      keys:
        - %q
`, keyStr)
	if storage != "" {
		cfg += storage
	}
	return cfg
}

// 10.0.2.2 (the QEMU gateway) runs no NFS server, so it plays the role of an
// unreachable NAS.
const nfsMountSection = `storage:
  mounts:
    - type: nfs
      source: 10.0.2.2:/volume1/video
      target: /var/mnt/video
      options: ro,soft,timeo=10,retrans=1,retry=0,nfsvers=4.1
`

// mountSection renders a storage.mounts entry; empty options/fs are omitted.
func mountSection(mountType, source, target, fs, options string) string {
	section := fmt.Sprintf(`storage:
  mounts:
    - type: %s
      source: %s
      target: %s
`, mountType, source, target)
	if fs != "" {
		section += fmt.Sprintf("      fs: %s\n", fs)
	}
	if options != "" {
		section += fmt.Sprintf("      options: %q\n", options)
	}
	return section
}

const nfsBindCompose = `services:
  t:
    image: registry.altlinux.org/sisyphus/distroless-toybox:latest
    command: sleep 60
    volumes:
      - /var/mnt/video:/mnt:ro
`

func TestStorageConfig(t *testing.T) {
	t.Parallel()
	client := e2etest.BootVMProvisioned(t)

	e2etest.Step(t, "rejected when type is not supported", func(t *testing.T) {
		client.RequireConfigRejected(t, storageConfig(client,
			mountSection("cifs", "192.168.2.152:/volume1/video", "/var/mnt/video", "", "")))
	})

	e2etest.Step(t, "rejected when target is outside /var/mnt", func(t *testing.T) {
		client.RequireConfigRejected(t, storageConfig(client,
			mountSection("nfs", "192.168.2.152:/volume1/video", "/etc/video", "", "")))
	})

	e2etest.Step(t, "rejected on unit-file injection in options", func(t *testing.T) {
		client.RequireConfigRejected(t, storageConfig(client,
			mountSection("nfs", "192.168.2.152:/volume1/video", "/var/mnt/video", "",
				"ro\n[Service]\nExecStart=/bin/true")))
	})

	e2etest.Step(t, "apply succeeds even when the storage server is unreachable", func(t *testing.T) {
		client.ApplyConfig(t, storageConfig(client, nfsMountSection))
	})

	e2etest.Step(t, "config show contains the storage section", func(t *testing.T) {
		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "storage:")
		assert.Contains(t, out, "10.0.2.2:/volume1/video")
	})

	e2etest.Step(t, "mount unit is written on the host", func(t *testing.T) {
		client.RequireHostFileContains(t, mountUnitPath,
			"What=10.0.2.2:/volume1/video",
			"Where=/var/mnt/video",
			"Type=nfs",
			"Options=ro,soft,timeo=10,retrans=1,retry=0,nfsvers=4.1",
		)
	})

	e2etest.Step(t, "mount unit is in failed state (server unreachable)", func(t *testing.T) {
		client.RequireHostUnitState(t, "var-mnt-video.mount", "failed")
	})

	e2etest.Step(t, "retry timer is active", func(t *testing.T) {
		client.RequireHostUnitState(t, "selenite-mounts-retry.timer", "active")
	})

	e2etest.Step(t, "podman-restart gate drop-in is written", func(t *testing.T) {
		client.RequireHostFileContains(t, gateDropInPath,
			"selenite-storage-gate",
			"/var/mnt/video",
		)
	})

	e2etest.Step(t, "stack deploy on an unmounted storage path fails with a clear error", func(t *testing.T) {
		out, err := client.TryDeployStack("nfs-media", nfsBindCompose)
		require.Error(t, err)
		assert.Contains(t, out, "storage mounts are not ready")
	})

	e2etest.Step(t, "removing the storage section cleans up units", func(t *testing.T) {
		client.ApplyConfig(t, storageConfig(client, ""))

		out := client.Run(t, "machine config show")
		assert.NotContains(t, out, "storage:")

		assert.False(t, client.HostFileExists(t, mountUnitPath), "%s should be removed", mountUnitPath)
		assert.False(t, client.HostFileExists(t, gateDropInPath), "%s should be removed", gateDropInPath)
	})

	e2etest.Step(t, "stack deploy works again without storage mounts", func(t *testing.T) {
		client.DeployStack(t, "plain", `services:
  t:
    image: registry.altlinux.org/sisyphus/distroless-toybox:latest
    command: sleep 60
`)
		client.WaitStackRunning(t, "plain")
	})
}

const (
	diskMountUnitPath = "/etc/systemd/system/var-mnt-data.mount"
	diskMountTarget   = "/var/mnt/data"
)

const diskMountSection = `storage:
  mounts:
    - type: disk
      source: /dev/vdb
      target: /var/mnt/data
      fs: ext4
`

// The container exits immediately on a broken mount (marker file gone),
// making waitStackRunning fail.
const diskBindCompose = `services:
  t:
    image: registry.altlinux.org/sisyphus/distroless-toybox:latest
    command: sh -c 'test -f /mnt/data.txt && sleep 60'
    volumes:
      - /var/mnt/data:/mnt:ro
`

func TestStorageDiskMount(t *testing.T) {
	t.Parallel()

	diskImage := filepath.Join(t.TempDir(), "data.raw")
	out, err := exec.Command("truncate", "-s", "64M", diskImage).CombinedOutput()
	require.NoError(t, err, "truncate: %s", out)

	client := e2etest.BootVMProvisioned(t, diskImage)

	e2etest.Step(t, "scratch disk is formatted and seeded with a marker file", func(t *testing.T) {
		client.Debug(t, `sh -c "dev=$(cat /sys/block/vdb/dev); `+
			`[ -b /dev/vdb ] || mknod /dev/vdb b ${dev%%:*} ${dev##*:} || exit 1; `+
			`mkfs.ext2 -F /dev/vdb && mkdir -p /m && mount -t ext4 /dev/vdb /m && `+
			`echo disk-ok > /m/data.txt && umount /m"`)
	})

	e2etest.Step(t, "rejected when source is not a device path", func(t *testing.T) {
		client.RequireConfigRejected(t, storageConfig(client,
			mountSection("disk", "vdb", diskMountTarget, "ext4", "")))
	})

	e2etest.Step(t, "rejected when fs is missing", func(t *testing.T) {
		client.RequireConfigRejected(t, storageConfig(client,
			mountSection("disk", "/dev/vdb", diskMountTarget, "", "")))
	})

	e2etest.Step(t, "disk mount is applied and becomes active", func(t *testing.T) {
		client.ApplyConfig(t, storageConfig(client, diskMountSection))

		out := client.Run(t, "machine config show")
		assert.Contains(t, out, "type: disk")
		assert.Contains(t, out, "/dev/vdb")

		client.RequireHostFileContains(t, diskMountUnitPath,
			"What=/dev/vdb",
			"Where="+diskMountTarget,
			"Type=ext4",
		)
		client.RequireHostUnitState(t, "var-mnt-data.mount", "active")
	})

	e2etest.Step(t, "marker file is visible through the mounted device", func(t *testing.T) {
		out := client.HostCmd(t, "cat "+path.Join(diskMountTarget, "data.txt"))
		require.Equal(t, "disk-ok", out)
	})

	e2etest.Step(t, "podman-restart gate drop-in lists the disk mount", func(t *testing.T) {
		client.RequireHostFileContains(t, gateDropInPath,
			"selenite-storage-gate",
			diskMountTarget,
		)
	})

	e2etest.Step(t, "stack deploy works against the active disk mount", func(t *testing.T) {
		client.DeployStack(t, "data-disk", diskBindCompose)
		client.WaitStackRunning(t, "data-disk")
	})

	e2etest.Step(t, "removing the storage section cleans up units", func(t *testing.T) {
		client.ApplyConfig(t, storageConfig(client, ""))

		out := client.Run(t, "machine config show")
		assert.NotContains(t, out, "storage:")

		assert.False(t, client.HostFileExists(t, diskMountUnitPath),
			"%s should be removed", diskMountUnitPath)
		assert.False(t, client.HostFileExists(t, gateDropInPath),
			"%s should be removed", gateDropInPath)
	})
}
