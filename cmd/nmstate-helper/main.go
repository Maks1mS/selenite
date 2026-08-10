// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"altlinux.space/alt-atomic/selenite/internal/cmdutils"
	"altlinux.space/alt-atomic/selenite/internal/executil"
)

const (
	nmstateBin        = "/usr/bin/nmstatectl"
	nmstateBackupDir  = "/run/selenite-controller/nmstate-backup"
	nmstateBackupFile = nmstateBackupDir + "/current.yaml"
	nmstateApplyFile  = nmstateBackupDir + "/desired.yaml"
	baselineFile      = "/var/lib/selenite/nmstate-baseline.yaml"
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
	case "cleanup":
		runCleanup()
	case "apply":
		runApply()
	default:
		cmdutils.Fatal("unknown mode: %s\n", mode)
	}
}

// runBackup captures the current network state with `nmstatectl show`. The first
// time it is called it also saves the captured state as the baseline so that a
// later cleanup can restore network configuration.
func runBackup() {
	if err := os.MkdirAll(nmstateBackupDir, 0o700); err != nil {
		cmdutils.Fatal("mkdir backup dir: %v", err)
	}
	//gosec:disable G302 -- Expected
	if err := os.Chmod(nmstateBackupDir, 0o700); err != nil {
		cmdutils.Fatal("chmod backup dir: %v", err)
	}

	var stdout bytes.Buffer
	if err := runCapture(nmstateBin, []string{"show"}, &stdout); err != nil {
		cmdutils.Fatal("nmstatectl show: %v\n", err)
	}

	if _, err := os.Stat(baselineFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(baselineFile), 0o700); err != nil {
			cmdutils.Fatal("mkdir baseline dir: %v\n", err)
		}
		if err := os.WriteFile(baselineFile, stdout.Bytes(), 0o600); err != nil {
			cmdutils.Fatal("write baseline: %v\n", err)
		}
	}

	if err := os.WriteFile(nmstateBackupFile, stdout.Bytes(), 0o600); err != nil {
		cmdutils.Fatal("write backup: %v\n", err)
	}
}

// runCleanup restores the original (baseline) network state saved before the
// first apply.
func runCleanup() {
	data, err := os.ReadFile(baselineFile)
	if err != nil {
		cmdutils.Fatal("read baseline: %v\n", err)
	}
	if err := applyState(data); err != nil {
		cmdutils.Fatal("cleanup: %v\n", err)
	}
}

// runRestore applies the previously backed-up network state.
func runRestore() {
	data, err := os.ReadFile(nmstateBackupFile)
	if err != nil {
		cmdutils.Fatal("read backup: %v\n", err)
	}
	if err := applyState(data); err != nil {
		cmdutils.Fatal("restore: %v\n", err)
	}
}

// runApply reads the desired nmstate YAML from stdin and applies it.
func runApply() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		cmdutils.Fatal("read stdin: %v\n", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		cmdutils.Fatal("empty desired state\n")
	}
	if err := applyState(data); err != nil {
		cmdutils.Fatal("apply: %v\n", err)
	}
}

// applyState writes the given nmstate YAML to a temporary file and applies it
// with `nmstatectl apply`.
func applyState(data []byte) error {
	if err := os.MkdirAll(nmstateBackupDir, 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(nmstateApplyFile, data, 0o600); err != nil { //gosec:disable G703 -- Trusted constant path
		return fmt.Errorf("write desired: %w", err)
	}

	var stderr bytes.Buffer
	err := executil.ExecRunner{}.
		Run(context.Background(),
			nmstateBin,
			[]string{"apply", nmstateApplyFile},
			executil.WithStdout(os.Stdout),
			executil.WithStderr(&stderr),
		)
	if err != nil {
		return fmt.Errorf("nmstatectl apply: %w: %s", err, stderr.String())
	}

	return nil
}

func runCapture(name string, args []string, stdout io.Writer) error {
	var stderr bytes.Buffer

	err := executil.ExecRunner{}.
		Run(context.Background(),
			name,
			args,
			executil.WithStdout(stdout),
			executil.WithStderr(&stderr),
		)
	if err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}

	return nil
}
