package keys

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic-lab/selenite/internal/lock"
)

var (
	errKeyWithSameName = errors.New("key with same name already exist")
	errKeyAlreadyExist = errors.New("key already exist")
	errKeyNotFound     = errors.New("key not found")
	errKeyIsTheLast    = errors.New("key is the last one, so it cannot be deleted")
)

type Key struct {
	Name    string
	Comment string
	Pk      ssh.PublicKey
}

func (k *Key) ToString() string {
	return fmt.Sprintf("%s %s;%s", bytes.TrimRight(gossh.MarshalAuthorizedKey(k.Pk), "\n"), k.Name, k.Comment)
}

func KeyFromString(s string) (Key, error) {
	pk, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(s))
	if err != nil {
		return Key{}, fmt.Errorf("parse line: %w", err)
	}
	key, err := parseComment(comment)
	if err != nil {
		return Key{}, fmt.Errorf("parse comment: %w", err)
	}
	key.Pk = pk
	return key, nil
}

type keysManager struct {
	keysFile     string
	keysFileLock *lock.Lock
}

var DefaultKeysFile = "/var/lib/selenite-controller/.ssh/authorized_keys"

func NewKeysManager() *keysManager {
	h := sha256.Sum256([]byte(DefaultKeysFile))
	return &keysManager{
		DefaultKeysFile,
		lock.New(fmt.Sprintf("keysManager-%x", h[:4])),
	}
}

func (m *keysManager) List() ([]Key, error) {
	content, err := os.ReadFile(m.keysFile)
	if err != nil {
		return []Key{}, fmt.Errorf("read keysFile: %w", err)
	}

	return parseKeyFileContent(content)
}

func parseKeyFileContent(content []byte) ([]Key, error) {
	var keys []Key
	for _, line := range bytes.Split(content, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
			continue
		}
		key, err := KeyFromString(string(line))
		if err != nil {
			return nil, fmt.Errorf("key from string: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func parseComment(comment string) (Key, error) {
	parts := strings.Split(comment, ";")
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Key{}, fmt.Errorf("name cannot be empty")
	}
	key := Key{name, "", nil}
	if len(parts) > 1 {
		key.Comment = strings.TrimSpace(parts[1])
	}
	return key, nil
}

func (m *keysManager) Add(keyStr string) (err error) {
	_ = m.keysFileLock.Lock()
	defer func() {
		_ = m.keysFileLock.Unlock()
	}()

	keys, err := m.List()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	newKey, err := KeyFromString(keyStr)
	if err != nil {
		return fmt.Errorf("key from string: %w", err)
	}
	for _, key := range keys {
		if key.Name == newKey.Name {
			return errKeyWithSameName
		}
		if bytes.Equal(key.Pk.Marshal(), newKey.Pk.Marshal()) {
			return errKeyAlreadyExist
		}
	}

	f, err := os.OpenFile(m.keysFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("read keysFile: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = f.WriteString(newKey.ToString() + "\n")
	if err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	return nil
}

func (m *keysManager) Remove(name string) (err error) {
	_ = m.keysFileLock.Lock()
	defer func() {
		_ = m.keysFileLock.Unlock()
	}()

	keys, err := m.List()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(keys) == 1 {
		return errKeyIsTheLast
	}

	tmpFile := m.keysFile + ".tmp"
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY, 0o600) //gosec:disable G304 -- Expected
	if err != nil {
		return fmt.Errorf("read keysFile: %w", err)
	}
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(tmpFile)
		}
	}()

	removed := false

	for _, key := range keys {
		if key.Name == name {
			removed = true
			continue
		}
		_, err = f.WriteString(key.ToString() + "\n")
		if err != nil {
			return fmt.Errorf("write key: %w", err)
		}
	}

	if !removed {
		return errKeyNotFound
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}

	if err := os.Rename(m.keysFile+".tmp", m.keysFile); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}
