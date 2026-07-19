// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package privdrop provides helpers for privilege-dropping executor binaries.
package privdrop

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const PodmanExecutorUser = "_selenite-podman-executor"

// ToUser drops privileges to the named user.
func ToUser(username string) (*user.User, error) {
	runtime.LockOSThread()

	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	gidStrs, err := u.GroupIds()
	if err != nil {
		return nil, fmt.Errorf("resolve groups: %w", err)
	}
	gids := make([]int, 0, len(gidStrs))
	for _, gs := range gidStrs {
		g, err := strconv.Atoi(gs)
		if err != nil {
			return nil, fmt.Errorf("parse gid %q: %w", gs, err)
		}
		gids = append(gids, g)
	}

	if err := syscall.Setgroups(gids); err != nil {
		return nil, fmt.Errorf("setgroups: %w", err)
	}
	if err := syscall.Setresgid(gid, gid, gid); err != nil {
		return nil, fmt.Errorf("setresgid: %w", err)
	}
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		return nil, fmt.Errorf("setresuid: %w", err)
	}
	return u, nil
}

// BuildEnv constructs a sanitized environment for the target user.
func BuildEnv(u *user.User) []string {
	override := map[string]string{
		"HOME":            u.HomeDir,
		"USER":            u.Username,
		"LOGNAME":         u.Username,
		"PATH":            "/usr/local/bin:/usr/bin:/bin",
		"XDG_RUNTIME_DIR": fmt.Sprintf("/run/user/%s", u.Uid),
	}

	drop := map[string]struct{}{
		"DBUS_SESSION_BUS_ADDRESS": {},
		"XDG_SESSION_ID":           {},
	}

	result := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		key := strings.SplitN(e, "=", 2)[0]
		if _, skip := drop[key]; skip {
			continue
		}
		if val, ok := override[key]; ok {
			result = append(result, key+"="+val)
			delete(override, key)
		} else {
			result = append(result, e)
		}
	}
	for k, v := range override {
		result = append(result, k+"="+v)
	}
	return result
}
