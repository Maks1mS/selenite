// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package dashtest drives bubbletea models in tests without a terminal and
// compares their output with golden files.
package dashtest

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// settle is how long a command may run before it is treated as a timer
// (tea.Tick, cursor blinking) and dropped. Data sources in tests are fakes
// and return immediately, so only timers hit this limit.
const settle = 100 * time.Millisecond

// maxSteps guards against models that keep producing messages forever.
const maxSteps = 10000

// Program runs a model synchronously: every message is followed by the
// messages of the commands it produced, until the model is idle.
// Timers never fire, so the output does not depend on time.
type Program struct {
	t     testing.TB
	model tea.Model
}

// Start initializes m with the given terminal size.
func Start(t testing.TB, m tea.Model, width, height int) *Program {
	t.Helper()

	p := &Program{t: t, model: m}
	init := m.Init()
	p.Send(tea.WindowSizeMsg{Width: width, Height: height})
	p.process(run(init))

	return p
}

// Send delivers msgs one by one, each followed by the messages it caused.
func (p *Program) Send(msgs ...tea.Msg) {
	p.t.Helper()

	for _, msg := range msgs {
		p.process([]tea.Msg{msg})
	}
}

// Type sends the text as individual key presses.
func (p *Program) Type(text string) {
	p.t.Helper()

	for _, r := range text {
		p.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// Press sends key presses with the given codes, such as tea.KeyEnter.
func (p *Program) Press(codes ...rune) {
	p.t.Helper()

	for _, code := range codes {
		p.Send(tea.KeyPressMsg{Code: code})
	}
}

// View returns the rendered screen as plain text.
func (p *Program) View() string {
	return ansi.Strip(p.model.View().Content)
}

func (p *Program) process(queue []tea.Msg) {
	p.t.Helper()

	for step := 0; len(queue) > 0; step++ {
		if step == maxSteps {
			p.t.Fatalf("model is still busy after %d messages", maxSteps)
		}

		var cmd tea.Cmd
		p.model, cmd = p.model.Update(queue[0])
		queue = append(queue[1:], run(cmd)...)
	}
}

// run executes cmd and returns the messages it produced in a stable order.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	select {
	case msg := <-done:
		if cmds, ok := subcommands(msg); ok {
			return runAll(cmds)
		}
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	case <-time.After(settle):
		return nil
	}
}

func runAll(cmds []tea.Cmd) []tea.Msg {
	results := make([][]tea.Msg, len(cmds))

	var wg sync.WaitGroup
	for i, cmd := range cmds {
		wg.Go(func() { results[i] = run(cmd) })
	}
	wg.Wait()

	return slices.Concat(results...)
}

var cmdSliceType = reflect.TypeFor[[]tea.Cmd]()

// subcommands unpacks tea.Batch and tea.Sequence. The message type of the
// latter is unexported, so it is matched by its underlying type.
func subcommands(msg tea.Msg) ([]tea.Cmd, bool) {
	v := reflect.ValueOf(msg)
	if !v.IsValid() || v.Kind() != reflect.Slice || v.Type().Elem() != cmdSliceType.Elem() {
		return nil, false
	}

	return v.Convert(cmdSliceType).Interface().([]tea.Cmd), true
}

// RequireGolden compares view with testdata/<test name>.golden.
// Trailing spaces are trimmed so the files survive editors.
// Run the tests with UPDATE_GOLDEN=1 to rewrite the files.
func RequireGolden(t testing.TB, view string) {
	t.Helper()

	got := trimLines(view) + "\n"
	path := filepath.Join("testdata", filepath.FromSlash(t.Name())+".golden")

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
	}

	//gosec:disable G304 -- path is built from the test name
	want, err := os.ReadFile(path)
	require.NoError(t, err, "run with UPDATE_GOLDEN=1 to create the golden file")
	require.Equal(t, string(want), got, "output differs from %s; run with UPDATE_GOLDEN=1 to update", path)
}

// RequireFits checks that view occupies exactly height lines of at most width cells.
func RequireFits(t testing.TB, view string, width, height int) {
	t.Helper()

	lines := strings.Split(view, "\n")
	require.Len(t, lines, height, "view height")

	for i, line := range lines {
		require.LessOrEqual(t, ansi.StringWidth(line), width, "line %d is wider than the screen: %q", i+1, line)
	}
}

func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
