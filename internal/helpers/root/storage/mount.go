// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Mount describes a single host-level storage mount.
type Mount struct {
	Type    string `json:"type"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	Fs      string `json:"fs,omitempty"`
	Options string `json:"options"`
}

// Decode reads a mount list in the wire format from r.
func Decode(r io.Reader) ([]Mount, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	mounts := []Mount{}
	if len(bytes.TrimSpace(data)) == 0 {
		return mounts, nil
	}
	if err := json.Unmarshal(data, &mounts); err != nil {
		return nil, fmt.Errorf("parse mounts: %w", err)
	}
	return mounts, nil
}

var (
	nfsSourceRe  = regexp.MustCompile(`^[A-Za-z0-9._-]+:/[A-Za-z0-9/._-]+$`) // "host:/export/path"
	diskSourceRe = regexp.MustCompile(`^/dev/[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)
	fsRe         = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	targetRe     = regexp.MustCompile(`^/var/mnt/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	optionsRe    = regexp.MustCompile(`^[A-Za-z0-9,._=-]*$`)
)

// Validate validates a set of mounts.
func Validate(mounts []Mount, label string) error {
	seen := map[string]struct{}{}
	for i, m := range mounts {
		prefix := fmt.Sprintf("%s[%d]", label, i)
		switch m.Type {
		case "nfs":
			if !nfsSourceRe.MatchString(m.Source) {
				return fmt.Errorf("%s: invalid source %q (expected host:/export/path)", prefix, m.Source)
			}
			if m.Fs != "" {
				return fmt.Errorf("%s: fs is only valid for type \"disk\"", prefix)
			}
		case "disk":
			if err := validateDiskSource(prefix, m.Source, m.Fs); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s: unsupported type %q (supported types: \"nfs\", \"disk\")", prefix, m.Type)
		}
		if !targetRe.MatchString(m.Target) {
			return fmt.Errorf("%s: invalid target %q (must be /var/mnt/<name>)", prefix, m.Target)
		}
		if _, dup := seen[m.Target]; dup {
			return fmt.Errorf("%s: duplicate target %q", prefix, m.Target)
		}
		seen[m.Target] = struct{}{}
		if !optionsRe.MatchString(m.Options) {
			return fmt.Errorf("%s: invalid options %q (allowed characters: letters, digits, ',', '.', '_', '=', '-')", prefix, m.Options)
		}
	}
	return nil
}

// validateDiskSource validates a "disk" source.
func validateDiskSource(prefix, source, fs string) error {
	if !diskSourceRe.MatchString(source) {
		return fmt.Errorf("%s: invalid source %q (expected a device path like /dev/disk/by-label/data)", prefix, source)
	}
	for _, seg := range strings.Split(strings.TrimPrefix(source, "/"), "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("%s: invalid source %q (dot and dotdot path segments are not allowed)", prefix, source)
		}
	}
	if fs == "" {
		return fmt.Errorf("%s: fs is required for type \"disk\"", prefix)
	}
	if !fsRe.MatchString(fs) {
		return fmt.Errorf("%s: invalid fs %q (allowed characters: letters, digits, '.', '_', '-')", prefix, fs)
	}
	return nil
}
