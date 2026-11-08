// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"altlinux.space/alt-atomic/selenite/tests/e2e/internal/e2etest"
)

var btrfsDevSizeRe = regexp.MustCompile(`devid\s+1\s+size\s+(\d+)`)

// The root partition and filesystem must grow to fill a disk that is larger
// than the image.
func TestRootFilesystemGrowsToFillDisk(t *testing.T) {
	t.Parallel()

	disk := filepath.Join(t.TempDir(), "disk.qcow2")
	out, err := exec.Command(
		"qemu-img", "create", "-q",
		"-f", "qcow2",
		"-b", e2etest.BaseDiskImage(), "-F", "raw",
		disk, "10G",
	).CombinedOutput()
	require.NoError(t, err, "qemu-img create: %s", out)

	client := e2etest.BootVMWithDisk(t, disk, "qcow2")
	client.Provision(t)

	var diskSize, partSize int64
	var rootDev string

	e2etest.Step(t, "root partition fills the disk", func(t *testing.T) {
		rootDev = client.HostCmd(t, "findmnt -nvo SOURCE /sysroot")
		require.Equal(t, "/dev/vda3", rootDev)

		diskSize = client.HostCmdInt(t, "lsblk -bndo SIZE /dev/vda")
		partStart := client.HostCmdInt(t, "cat /sys/class/block/vda3/start") * 512
		partSize = client.HostCmdInt(t, "lsblk -bndo SIZE "+rootDev)

		// growpart leaves only the backup GPT (~1 MiB) at the end of the disk.
		assert.InDelta(t, diskSize, partStart+partSize, 2<<20)
	})

	e2etest.Step(t, "root filesystem fills the partition", func(t *testing.T) {
		out := client.HostCmd(t, "btrfs filesystem show --raw /sysroot")
		m := btrfsDevSizeRe.FindStringSubmatch(out)
		require.NotNil(t, m, "btrfs device size not found in:\n%s", out)
		fsSize, err := strconv.ParseInt(m[1], 10, 64)
		require.NoError(t, err)
		// btrfs rounds the device size down to its sector size.
		const btrfsSectorSize = 4096
		assert.Equal(t, partSize/btrfsSectorSize*btrfsSectorSize, fsSize)
	})

	e2etest.Step(t, "bootc-generic-growpart reports the change", func(t *testing.T) {
		out := client.Run(t, "machine logs -n 50 bootc-generic-growpart")
		assert.Contains(t, out, "CHANGED: partition=3")
	})
}
