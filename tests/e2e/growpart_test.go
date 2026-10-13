// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2e

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hostCmd(t *testing.T, client *testClient, cmd string) string {
	t.Helper()

	const hostOutputMarker = "--- host output ---"

	out, err := runPtyCmd(t, client.ssh,
		fmt.Sprintf(`debug quay.io/quay/busybox -- sh -c "echo '%s'; chroot /host %s"`, hostOutputMarker, cmd))
	require.NoError(t, err, "host command %q\noutput: %s", cmd, out)
	_, after, found := strings.Cut(out, hostOutputMarker)
	require.True(t, found, "host command %q: marker not found in output: %s", cmd, out)
	return strings.TrimSpace(strings.ReplaceAll(after, "\r", ""))
}

func hostCmdInt(t *testing.T, client *testClient, cmd string) int64 {
	t.Helper()
	out := hostCmd(t, client, cmd)
	n, err := strconv.ParseInt(out, 10, 64)
	require.NoError(t, err, "host command %q: not a number: %q", cmd, out)
	return n
}

var btrfsDevSizeRe = regexp.MustCompile(`devid\s+1\s+size\s+(\d+)`)

// The root partition and filesystem must grow to fill a disk that is larger
// than the image.
func TestRootFilesystemGrowsToFillDisk(t *testing.T) {
	t.Parallel()

	disk := filepath.Join(t.TempDir(), "disk.qcow2")
	out, err := exec.Command(
		"qemu-img", "create", "-q",
		"-f", "qcow2",
		"-b", baseDiskImage(), "-F", "raw",
		disk, "10G",
	).CombinedOutput()
	require.NoError(t, err, "qemu-img create: %s", out)

	client := bootVMWithDisk(t, disk, "qcow2")
	provision(t, client)

	var diskSize, partSize int64
	var rootDev string

	run(t, "root partition fills the disk", func(t *testing.T) {
		rootDev = hostCmd(t, client, "findmnt -nvo SOURCE /sysroot")
		require.Equal(t, "/dev/vda3", rootDev)

		diskSize = hostCmdInt(t, client, "lsblk -bndo SIZE /dev/vda")
		partStart := hostCmdInt(t, client, "cat /sys/class/block/vda3/start") * 512
		partSize = hostCmdInt(t, client, "lsblk -bndo SIZE "+rootDev)

		// growpart leaves only the backup GPT (~1 MiB) at the end of the disk.
		assert.InDelta(t, diskSize, partStart+partSize, 2<<20)
	})

	run(t, "root filesystem fills the partition", func(t *testing.T) {
		out := hostCmd(t, client, "btrfs filesystem show --raw /sysroot")
		m := btrfsDevSizeRe.FindStringSubmatch(out)
		require.NotNil(t, m, "btrfs device size not found in:\n%s", out)
		fsSize, err := strconv.ParseInt(m[1], 10, 64)
		require.NoError(t, err)
		// btrfs rounds the device size down to its sector size.
		const btrfsSectorSize = 4096
		assert.Equal(t, partSize/btrfsSectorSize*btrfsSectorSize, fsSize)
	})

	run(t, "bootc-generic-growpart reports the change", func(t *testing.T) {
		out := mustRunCmd(t, client, "machine logs -n 50 bootc-generic-growpart")
		assert.Contains(t, out, "CHANGED: partition=3")
	})
}
