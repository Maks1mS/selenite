// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

//go:build e2e

package e2etest

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// Option redirects the standard streams of an SSH session.
type Option func(*gossh.Session)

func WithStdin(r io.Reader) Option  { return func(s *gossh.Session) { s.Stdin = r } }
func WithStdout(w io.Writer) Option { return func(s *gossh.Session) { s.Stdout = w } }
func WithStderr(w io.Writer) Option { return func(s *gossh.Session) { s.Stderr = w } }
func WithCombinedOutput(w io.Writer) Option {
	return func(s *gossh.Session) { s.Stdout = w; s.Stderr = w }
}

// Exec runs cmd over SSH with optional stream redirection.
func (c *Client) Exec(cmd string, opts ...Option) error {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	for _, o := range opts {
		o(sess)
	}
	if err := sess.Run(cmd); err != nil {
		var ee *gossh.ExitError
		if errors.As(err, &ee) {
			return ee
		}
		return err
	}
	return nil
}

// TryRun runs cmd over SSH with combined output capture and an optional
// stdin.
func (c *Client) TryRun(cmd string, stdin io.Reader) (string, error) {
	var buf strings.Builder
	err := c.Exec(cmd, WithCombinedOutput(&buf), WithStdin(stdin))
	return buf.String(), err
}

// TryRunPTY is TryRun in a PTY session.
func (c *Client) TryRunPTY(cmd string) (string, error) {
	sess, err := c.ssh.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()

	if err := sess.RequestPty("xterm", 80, 24, gossh.TerminalModes{}); err != nil {
		return "", fmt.Errorf("request pty: %w", err)
	}

	var buf strings.Builder
	sess.Stdout = &buf
	sess.Stderr = &buf

	if err := sess.Run(cmd); err != nil {
		var ee *gossh.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee
		}
		return buf.String(), err
	}
	return buf.String(), nil
}

// Run runs cmd over SSH, returning its combined output; it fails the test on
// error.
func (c *Client) Run(t testing.TB, cmd string) string {
	t.Helper()
	return c.RunStdin(t, cmd, nil)
}

// RunStdin is Run with stdin attached.
func (c *Client) RunStdin(t testing.TB, cmd string, stdin io.Reader) string {
	t.Helper()
	out, err := c.TryRun(cmd, stdin)
	require.NoError(t, err, "command %q\noutput: %s", cmd, out)
	return out
}

// PtyText normalizes the CRLF output produced by PTY sessions.
func PtyText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
}
