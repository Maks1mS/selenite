// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package journal

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/coreos/go-systemd/v22/sdjournal"
)

const (
	pollInterval = 500 * time.Millisecond
	batchSize    = 256
	maxLines     = 2000
)

type Model struct {
	viewport       viewport.Model
	entries        []entry
	lines          []string
	positions      []textPosition
	hiddenPosition *textPosition
	cursor         string
	follow         bool
	loading        bool
	atHead         bool
	atTail         bool
	generation     uint64
	err            error
}

// textPosition identifies the original text at the start of a screen row.
// Byte offsets remain stable when wrapping, prepending, or evicting entries.
type textPosition struct {
	cursor string
	offset int
}

type pollMsg struct {
	target     *Model
	generation uint64
}

type readMode uint8

const (
	readFollow readMode = iota
	readTail
	readOlder
	readNewer
)

type entry struct {
	cursor string
	text   string
}

func (e entry) lineCount() int {
	return strings.Count(e.text, "\n") + 1
}

type entriesMsg struct {
	target     *Model
	generation uint64
	mode       readMode
	entries    []entry
	more       bool
	err        error
}

func New() *Model {
	vp := viewport.New()
	vp.FillHeight = true
	vp.SetHorizontalStep(0)

	return &Model{viewport: vp, follow: true}
}

func (m *Model) Init() tea.Cmd {
	m.generation++
	m.loading = false

	if m.follow || len(m.entries) == 0 {
		return m.readCmd(readFollow)
	}

	return m.waitCmd()
}

func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case entriesMsg:
		return m.handleEntries(msg)
	case pollMsg:
		return m.handlePoll(msg)
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

		return m, nil
	}

	// Hidden viewports must not change follow mode in response to navigation.
	if m.viewport.Width() == 0 || m.viewport.Height() == 0 {
		return m, nil
	}

	return m.navigate(msg)
}

func (m *Model) handleEntries(msg entriesMsg) (*Model, tea.Cmd) {
	if msg.target != m || msg.generation != m.generation {
		return m, nil
	}

	m.loading, m.err = false, msg.err

	// A live read may finish after the user has started browsing history.
	// Do not let it evict the entries currently being viewed.
	if msg.mode == readFollow && !m.follow {
		if m.viewport.AtTop() && !m.atHead && len(m.entries) > 0 {
			return m, m.readCmd(readOlder)
		}

		return m, m.waitCmd()
	}

	if msg.mode == readOlder && m.follow {
		return m, m.readCmd(readFollow)
	}

	m.applyEntries(msg)

	if m.follow && msg.more && msg.err == nil {
		return m, m.readCmd(readFollow)
	}

	return m, m.waitCmd()
}

func (m *Model) handlePoll(msg pollMsg) (*Model, tea.Cmd) {
	if msg.target != m || msg.generation != m.generation || m.loading {
		return m, nil
	}

	if m.follow {
		return m, m.readCmd(readFollow)
	}

	return m, m.waitCmd()
}

func (m *Model) navigate(msg tea.Msg) (*Model, tea.Cmd) {
	up, down := false, false

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "home", "g":
			m.viewport.GotoTop()
			m.follow = false

			return m, m.olderCmd()
		case "end", "G":
			m.viewport.GotoBottom()
			m.follow = true
			m.loading = false

			// Jump straight to the live tail, invalidating any history read.
			return m, m.readCmd(readTail)
		}

		keys := m.viewport.KeyMap
		up = key.Matches(msg, keys.Up, keys.PageUp, keys.HalfPageUp)
		down = key.Matches(msg, keys.Down, keys.PageDown, keys.HalfPageDown)
	case tea.MouseWheelMsg:
		// Shift-wheel is horizontal navigation in viewport, not vertical scrolling.
		if !msg.Mod.Contains(tea.ModShift) {
			up = msg.Button == tea.MouseWheelUp
			down = msg.Button == tea.MouseWheelDown
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	if up {
		m.follow = false
		if m.viewport.YOffset() <= m.viewport.Height() {
			return m, tea.Batch(cmd, m.olderCmd())
		}
	} else if down && m.viewport.AtBottom() {
		if m.atTail {
			m.follow = true

			return m, tea.Batch(cmd, m.readCmd(readFollow))
		}

		// The end of the cached window is not necessarily the live tail.
		return m, tea.Batch(cmd, m.readCmd(readNewer))
	}

	return m, cmd
}

// SetWidth sets the available content width, excluding any parent's border.
func (m *Model) SetWidth(width int) {
	m.SetSize(width, m.viewport.Height())
}

// SetHeight sets the log area's height. Resizing never resumes a pause.
func (m *Model) SetHeight(height int) {
	m.SetSize(m.viewport.Width(), height)
}

// SetSize changes both dimensions before reflowing so the old height cannot
// incorrectly clamp the scroll position during a simultaneous width change.
func (m *Model) SetSize(width, height int) {
	width, height = max(0, width), max(0, height)
	if width == m.viewport.Width() && height == m.viewport.Height() {
		return
	}

	position := m.position()
	widthChanged := width != m.viewport.Width()
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)

	if widthChanged {
		m.reflow(position)

		return
	}

	m.viewport.SetYOffset(m.viewport.YOffset())
	if m.follow {
		m.viewport.GotoBottom()
	}
}

// Err reports the most recent read error without adding a status line to the view.
func (m *Model) Err() error {
	return m.err
}

// View renders only the scrollable log area.
func (m *Model) View() tea.View {
	v := tea.NewView("")
	v.MouseMode = tea.MouseModeCellMotion

	if m.viewport.Width() > 0 && m.viewport.Height() > 0 {
		v.Content = m.viewport.View()
	}

	return v
}

func (m *Model) applyEntries(msg entriesMsg) {
	if msg.err == nil {
		if msg.mode == readOlder {
			m.atHead = !msg.more
		} else {
			m.atTail = !msg.more
		}
	}

	if len(msg.entries) == 0 {
		return
	}

	position := m.position()

	switch msg.mode {
	case readTail:
		m.entries = msg.entries
		m.atHead = false
		position = textPosition{}
	case readOlder:
		m.entries = append(slices.Clone(msg.entries), m.entries...)
	case readFollow, readNewer:
		m.entries = append(m.entries, msg.entries...)
	}

	total := 0
	for _, e := range m.entries {
		total += e.lineCount()
	}

	start, end := 0, len(m.entries)
	// Evict whole entries from the opposite end of the window. Keep at least
	// one entry even if a single multiline message exceeds the line budget.
	for total > maxLines && end-start > 1 {
		if msg.mode == readOlder {
			end--
			total -= m.entries[end].lineCount()
			if position.cursor == m.entries[end].cursor {
				previous := m.entries[end-1]
				position = textPosition{cursor: previous.cursor, offset: len(previous.text)}
			}

			m.atTail = false
		} else {
			total -= m.entries[start].lineCount()
			if position.cursor == m.entries[start].cursor {
				position = textPosition{cursor: m.entries[start+1].cursor}
			}

			start++
			m.atHead = false
		}
	}

	m.entries = slices.Clone(m.entries[start:end])
	m.cursor = m.entries[len(m.entries)-1].cursor
	m.reflow(position)
}

func (m *Model) position() textPosition {
	if m.hiddenPosition != nil {
		return *m.hiddenPosition
	}

	if len(m.positions) == 0 {
		return textPosition{}
	}

	return m.positions[min(m.viewport.YOffset(), len(m.positions)-1)]
}

func (m *Model) reflow(position textPosition) {
	// Keep the last layout and its anchor while hidden. Reflow once a real
	// width arrives, rather than expanding every entry into one-column rows.
	if m.viewport.Width() == 0 {
		m.hiddenPosition = &position

		return
	}

	m.hiddenPosition = nil
	m.lines = nil
	m.positions = nil
	for _, e := range m.entries {
		m.wrapEntry(e)
	}

	m.viewport.SetContentLines(m.lines)
	m.viewport.SetXOffset(0)
	if m.follow {
		m.viewport.GotoBottom()

		return
	}

	offset := 0
	for i, row := range m.positions {
		if row.cursor != position.cursor {
			continue
		}

		if row.offset > position.offset {
			break
		}

		offset = i
	}

	m.viewport.SetYOffset(offset)
}

// Wrap sanitized text by grapheme clusters, not bytes or terminal-cell slices:
// slicing cells can lose a wide character that straddles a wrap boundary.
// The viewport receives physical rows, so all its scroll metrics agree with View.
func (m *Model) wrapEntry(e entry) {
	var row strings.Builder
	rowStart, offset, cells := 0, 0, 0

	flush := func() {
		m.lines = append(m.lines, row.String())
		m.positions = append(m.positions, textPosition{cursor: e.cursor, offset: rowStart})
		row.Reset()
		cells = 0
	}

	for offset < len(e.text) {
		cluster, width := ansi.FirstGraphemeCluster(e.text[offset:], ansi.GraphemeWidth)
		if cluster == "\n" {
			flush()
			offset += len(cluster)
			rowStart = offset

			continue
		}

		if cells > 0 && cells+width > m.viewport.Width() {
			flush()
			rowStart = offset
		}

		if width > m.viewport.Width() {
			row.WriteRune(utf8.RuneError)
			cells++
		} else {
			row.WriteString(cluster)
			cells += width
		}

		offset += len(cluster)
	}

	flush()
}

func (m *Model) olderCmd() tea.Cmd {
	if m.atHead || len(m.entries) == 0 {
		return nil
	}

	return m.readCmd(readOlder)
}

func (m *Model) waitCmd() tea.Cmd {
	generation := m.generation

	return tea.Tick(pollInterval, func(time.Time) tea.Msg {
		return pollMsg{target: m, generation: generation}
	})
}

func (m *Model) readCmd(mode readMode) tea.Cmd {
	if m.loading {
		return nil
	}

	cursor := m.cursor
	if mode == readOlder {
		cursor = m.entries[0].cursor
	} else if mode == readTail || cursor == "" {
		cursor = ""
		mode = readTail
	}

	// Starting a read also invalidates any previously scheduled polling timer.
	m.generation++
	m.loading = true
	generation := m.generation

	return func() tea.Msg {
		msg := readEntries(cursor, mode)
		msg.target = m
		msg.generation = generation

		return msg
	}
}

// Each command owns its journal handle, including its OS thread. Reopening by
// cursor avoids a persistent goroutine or descriptor requiring parent cleanup.
func readEntries(cursor string, mode readMode) entriesMsg {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := entriesMsg{mode: mode}
	reader, err := sdjournal.NewJournal()
	if err != nil {
		result.err = fmt.Errorf("open journal: %w", err)

		return result
	}
	defer func() { _ = reader.Close() }()

	if err := applyFilters(reader); err != nil {
		result.err = fmt.Errorf("filter journal: %w", err)
		return result
	}

	available, err := seek(reader, cursor, mode)
	if err != nil {
		result.err = fmt.Errorf("seek journal: %w", err)

		return result
	}

	step := reader.Next
	if mode == readOlder {
		step = reader.Previous
	}

	for count := 0; available > 0 && count < batchSize; count++ {
		journalEntry, entryErr := reader.GetEntry()
		if entryErr != nil {
			result.err = fmt.Errorf("read journal: %w", entryErr)

			break
		}

		result.entries = append(result.entries, entry{
			cursor: journalEntry.Cursor,
			text:   formatEntry(journalEntry),
		})

		available, err = step()
		if err != nil {
			result.err = fmt.Errorf("advance journal: %w", err)

			break
		}
	}

	if mode == readOlder {
		slices.Reverse(result.entries)
	}

	result.more = available > 0

	return result
}

func seek(reader *sdjournal.Journal, cursor string, mode readMode) (uint64, error) {
	if cursor == "" {
		if err := reader.SeekTail(); err != nil {
			return 0, fmt.Errorf("seek tail: %w", err)
		}

		available, err := reader.PreviousSkip(batchSize)
		if err != nil {
			return 0, fmt.Errorf("skip recent entries: %w", err)
		}

		return available, nil
	}

	if err := reader.SeekCursor(cursor); err != nil {
		return 0, fmt.Errorf("seek cursor: %w", err)
	}

	step := reader.Next
	if mode == readOlder {
		step = reader.Previous
	}

	available, err := step()
	if err != nil {
		return available, fmt.Errorf("advance to cursor: %w", err)
	}

	if available == 0 {
		return available, nil
	}

	// SeekCursor is approximate after vacuum/rotation. Only skip an entry if
	// it is the one already displayed, not its nearest surviving neighbor.
	if err := reader.TestCursor(cursor); err != nil {
		if errors.Is(err, sdjournal.ErrNoTestCursor) {
			return available, nil
		}

		return 0, fmt.Errorf("test cursor: %w", err)
	}

	available, err = step()
	if err != nil {
		return available, fmt.Errorf("skip current entry: %w", err)
	}

	return available, nil
}

func formatEntry(entry *sdjournal.JournalEntry) string {
	source := entry.Fields["SYSLOG_IDENTIFIER"]
	if source == "" {
		source = entry.Fields["_SYSTEMD_UNIT"]
	}

	if source == "" {
		source = entry.Fields["_COMM"]
	}

	if source == "" {
		source = "journal"
	}

	if pid := entry.Fields["_PID"]; pid != "" {
		source += "[" + pid + "]"
	}

	return fmt.Sprintf("%s %s: %s",
		journalTime(entry.RealtimeTimestamp).Format("Jan 02 15:04:05"),
		strings.ReplaceAll(sanitize(source), "\n", " "),
		sanitize(entry.Fields["MESSAGE"]),
	)
}

// journalTime converts a journal timestamp in microseconds since the Unix
// epoch, clamping to the int64 range to avoid an overflow conversion.
func journalTime(micros uint64) time.Time {
	if micros > math.MaxInt64 {
		micros = math.MaxInt64
	}

	return time.UnixMicro(int64(micros))
}

// Journal fields are untrusted text, not terminal instructions.
func sanitize(text string) string {
	text = strings.ReplaceAll(ansi.Strip(text), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")

	return strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}

		return r
	}, text)
}

func applyFilters(reader *sdjournal.Journal) error {
	transports := []string{"kernel", "syslog", "driver"}
	for i, t := range transports {
		if i > 0 {
			if err := reader.AddDisjunction(); err != nil {
				return fmt.Errorf("add disjunction: %w", err)
			}
		}

		if err := reader.AddMatch("_TRANSPORT=" + t); err != nil {
			return fmt.Errorf("add match: %w", err)
		}
	}

	return nil
}
