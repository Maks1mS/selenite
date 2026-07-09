// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package persist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cp "github.com/otiai10/copy"
)

const (
	// Root is the persistent selenite state directory.
	Root = "/var/lib/selenite"
	// EtcDir mirrors the managed subset of /etc.
	EtcDir = Root + "/etc-persist"
	// MarkersDir holds unit activation markers referenced by
	// ConditionPathExists= drop-ins.
	MarkersDir = Root + "/markers"
)

// etcRel maps an absolute /etc path to its location inside EtcDir.
func etcRel(etcPath string) (string, error) {
	cleaned := filepath.Clean(etcPath)
	rel := strings.TrimPrefix(cleaned, "/etc/")
	if rel == cleaned || rel == "" || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path %q is not under /etc", etcPath)
	}
	return filepath.Join(EtcDir, rel), nil
}

// SaveFile mirrors a single managed file from /etc into the persistent tree.
func SaveFile(etcPath string) error {
	dst, err := etcRel(etcPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(etcPath) //gosec:disable G304 -- etcPath is validated by etcRel to stay under /etc
	if err != nil {
		return fmt.Errorf("read %s: %w", etcPath, err)
	}
	info, err := os.Stat(etcPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", etcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //gosec:disable G301 -- mirrors /etc layout
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, info.Mode().Perm()); err != nil { //gosec:disable G703 -- dst is derived via etcRel
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("rename %s: %w", dst, err)
	}
	return nil
}

// SaveTree mirrors a managed directory from /etc into the persistent tree.
func SaveTree(etcPath string) error {
	dst, err := etcRel(etcPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //gosec:disable G301 -- mirrors /etc layout
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	tmp := dst + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := cp.Copy(etcPath, tmp, cp.Options{PreserveOwner: true}); err != nil {
		return fmt.Errorf("copy %s: %w", etcPath, err)
	}
	_ = os.RemoveAll(dst)
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("rename %s: %w", dst, err)
	}
	return nil
}

// Remove drops a file or tree from the persistent mirror. Missing entries are
// not an error, so disable paths stay idempotent.
func Remove(etcPath string) error {
	dst, err := etcRel(etcPath)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove %s: %w", dst, err)
	}
	return nil
}

// SetMarker creates or removes the persistent activation marker for a unit
// gated by ConditionPathExists=/var/lib/selenite/markers/<name>.
func SetMarker(name string, on bool) error {
	path := filepath.Join(MarkersDir, name)
	if !on {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove marker %s: %w", path, err)
		}
		return nil
	}
	if err := os.MkdirAll(MarkersDir, 0o755); err != nil { //gosec:disable G301 -- world-readable markers
		return fmt.Errorf("mkdir %s: %w", MarkersDir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644) //gosec:disable G302,G304 -- empty marker file
	if err != nil {
		return fmt.Errorf("create marker %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("marker close %s: %w", path, err)
	}

	return nil
}
