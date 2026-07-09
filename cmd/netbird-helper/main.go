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

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/persist"
)

const (
	netbirdBackupDir    = "/run/selenite-controller/netbird-backup"
	netbirdVarLibBackup = netbirdBackupDir + "/var_lib_netbird"
	netbirdEtcBackup    = netbirdBackupDir + "/etc_netbird"

	netbirdVarLibDir = "/var/lib/netbird"
	netbirdEtcDir    = "/etc/netbird"
	netbirdMarker    = "netbird"
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
	case "materialize":
		runMaterialize()
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

	if err := copyDir(netbirdVarLibDir, netbirdVarLibBackup); err != nil {
		cmdutils.Fatal("backup %s: %v", netbirdVarLibDir, err)
	}
	if err := copyDir(netbirdEtcDir, netbirdEtcBackup); err != nil {
		cmdutils.Fatal("backup %s: %v", netbirdEtcDir, err)
	}
}

func runRestore() {
	mustRun("/usr/bin/systemctl", "stop", "netbird")

	_ = os.RemoveAll(netbirdVarLibDir)
	_ = os.RemoveAll(netbirdEtcDir)

	if err := copyDir(netbirdVarLibBackup, netbirdVarLibDir); err != nil {
		cmdutils.Fatal("restore %s: %v", netbirdVarLibDir, err)
	}
	if err := copyDir(netbirdEtcBackup, netbirdEtcDir); err != nil {
		cmdutils.Fatal("restore %s: %v", netbirdEtcDir, err)
	}

	if !helperEnabled() {
		setEnabled(false)
		persistRemoveEtc()
		return
	}

	setEnabled(true)
	mustRun("/usr/bin/systemctl", "start", "netbird")
	persistEtc()
}

func runApply() {
	clearState := os.Getenv("SELENITE_NETBIRD_HELPER_CLEAR_STATE")
	if clearState == "1" {
		mustRun("/usr/bin/systemctl", "stop", "netbird")
		_ = os.RemoveAll(netbirdVarLibDir)
		_ = os.RemoveAll(netbirdEtcDir)
		persistRemoveEtc()
	}

	if !helperEnabled() {
		setEnabled(false)
		persistRemoveEtc()
		return
	}

	setEnabled(true)
	mustRun("/usr/bin/systemctl", "start", "netbird")

	setupKey := os.Getenv("SELENITE_NETBIRD_HELPER_SETUP_KEY")
	mustRun("/usr/bin/netbird", "up", "--setup-key", setupKey, "--allow-server-ssh=false")

	persistEtc()
}

func runMaterialize() {
	if !helperEnabled() {
		setEnabled(false)
		persistRemoveEtc()
		return
	}

	setEnabled(true)
	if _, err := os.Stat(netbirdEtcDir); err == nil {
		persistEtc()
	}
}

func helperEnabled() bool {
	return os.Getenv("SELENITE_NETBIRD_HELPER_ENABLED") == "1"
}

func setEnabled(on bool) {
	if err := persist.SetMarker(netbirdMarker, on); err != nil {
		cmdutils.Fatal("netbird marker: %v", err)
	}
	if !on {
		mustRun("/usr/bin/systemctl", "stop", "netbird")
	}
}

func persistEtc() {
	if err := persist.SaveTree(netbirdEtcDir); err != nil {
		cmdutils.Fatal("persist %s: %v", netbirdEtcDir, err)
	}
}

func persistRemoveEtc() {
	if err := persist.Remove(netbirdEtcDir); err != nil {
		cmdutils.Fatal("unpersist %s: %v", netbirdEtcDir, err)
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
