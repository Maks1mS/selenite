// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	cp "github.com/otiai10/copy"

	"altlinux.space/alt-atomic-lab/selenite/internal/cmdutils"
)

const (
	netbirdBackupDir    = "/run/selenite-controller/netbird-backup"
	netbirdVarLibBackup = netbirdBackupDir + "/var_lib_netbird"
	netbirdEtcBackup    = netbirdBackupDir + "/etc_netbird"
)

func main() {
	runtime.LockOSThread()
	if err := syscall.Setresgid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresgid: %v\n", err)
	}
	if err := syscall.Setresuid(0, 0, 0); err != nil {
		cmdutils.Fatal("setresuid: %v\n", err)
	}

	mode := "apply"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	switch mode {
	case "backup":
		runBackup()
	case "restore":
		runRestore()
	default:
		runApply()
	}
}

func runBackup() {
	if err := os.MkdirAll(netbirdBackupDir, 0o700); err != nil {
		cmdutils.Fatal("mkdir backup dir: %v", err)
	}
	//gosec:disable G302 -- Expected
	if err := os.Chmod(netbirdBackupDir, 0o700); err != nil {
		cmdutils.Fatal("chmod backup dir: %v", err)
	}

	_ = os.RemoveAll(netbirdVarLibBackup)
	_ = os.RemoveAll(netbirdEtcBackup)

	if err := copyDir("/var/lib/netbird", netbirdVarLibBackup); err != nil {
		cmdutils.Fatal("backup /var/lib/netbird: %v", err)
	}
	if err := copyDir("/etc/netbird", netbirdEtcBackup); err != nil {
		cmdutils.Fatal("backup /etc/netbird: %v", err)
	}
}

func runRestore() {
	mustRun("/usr/bin/systemctl", "stop", "netbird")

	_ = os.RemoveAll("/var/lib/netbird")
	_ = os.RemoveAll("/etc/netbird")

	if err := copyDir(netbirdVarLibBackup, "/var/lib/netbird"); err != nil {
		cmdutils.Fatal("restore /var/lib/netbird: %v", err)
	}
	if err := copyDir(netbirdEtcBackup, "/etc/netbird"); err != nil {
		cmdutils.Fatal("restore /etc/netbird: %v", err)
	}

	enabled := os.Getenv("SELENITE_NETBIRD_HELPER_ENABLED")
	if enabled == "1" {
		mustRun("/usr/bin/systemctl", "enable", "--now", "netbird")
	} else {
		mustRun("/usr/bin/systemctl", "disable", "--now", "netbird")
	}
}

func runApply() {
	clearState := os.Getenv("SELENITE_NETBIRD_HELPER_CLEAR_STATE")
	if clearState == "1" {
		mustRun("/usr/bin/systemctl", "stop", "netbird")
		_ = os.RemoveAll("/var/lib/netbird")
		_ = os.RemoveAll("/etc/netbird")
	}

	enabled := os.Getenv("SELENITE_NETBIRD_HELPER_ENABLED")
	if enabled == "1" {
		mustRun("/usr/bin/systemctl", "enable", "--now", "netbird")

		setupKey := os.Getenv("SELENITE_NETBIRD_HELPER_SETUP_KEY")
		mustRun("/usr/bin/netbird", "up", "--setup-key", setupKey, "--allow-server-ssh=false")
	} else {
		mustRun("/usr/bin/systemctl", "disable", "--now", "netbird")
	}
}

func mustRun(name string, args ...string) {
	//gosec:disable G204,G702 -- Expected
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		cmdutils.Fatal("%s %v: %v", name, args, err)
	}
}

func copyDir(src, dst string) error {
	if err := cp.Copy(src, dst); err != nil {
		return fmt.Errorf("cp: %w", err)
	}
	return nil
}
