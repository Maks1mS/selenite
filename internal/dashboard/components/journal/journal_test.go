// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package journal

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"
)

func numberedEntries(first, count int) []entry {
	entries := make([]entry, count)
	for i := range entries {
		cursor := strconv.Itoa(first + i)
		entries[i] = entry{cursor: cursor, text: "line " + cursor}
	}

	return entries
}

func TestInitRestartsPolling(t *testing.T) {
	t.Parallel()

	for name, paused := range map[string]bool{"following": false, "paused": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testInitRestartsPolling(t, paused)
		})
	}
}

func testInitRestartsPolling(t *testing.T, paused bool) {
	t.Helper()

	m := New()
	m.SetWidth(80)
	m.SetHeight(3)
	require.NotNil(t, m.Init(), "initialization must start reading")

	m.Update(entriesMsg{
		target: m, generation: m.generation, mode: readTail,
		entries: numberedEntries(1, 5),
	})
	if paused {
		m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	}

	generation, offset := m.generation, m.viewport.YOffset()
	require.NotNil(t, m.Init(), "reactivation must restart polling even if the previous result was lost")

	require.NotEqual(t, generation, m.generation, "reactivation must invalidate the previous polling chain")

	require.NotEqual(t, paused, m.follow, "reactivation changed follow mode")
	require.Equal(t, offset, m.viewport.YOffset(), "reactivation changed scroll position")
	require.Equal(t, "5", m.cursor, "reactivation changed cursor")

	assertStaleGenerationIgnored(t, m, generation)

	_, cmd := m.Update(entriesMsg{
		target: m, generation: m.generation, mode: readFollow,
		entries: numberedEntries(6, 1),
	})
	require.NotNil(t, cmd, "the new polling chain stopped")
	require.Equal(t, !paused, m.follow, "reactivation changed follow mode")

	assertReactivatedWindow(t, m, paused, offset)
}

func assertStaleGenerationIgnored(t *testing.T, m *Model, generation uint64) {
	t.Helper()

	_, cmd := m.Update(entriesMsg{
		target: m, generation: generation, entries: numberedEntries(100, 1),
		more: true, err: errors.New("stale read failure"),
	})
	require.Nil(t, cmd, "a stale result scheduled another read")
	require.Len(t, m.lines, 5, "a stale result changed the lines")
	require.Equal(t, "5", m.cursor, "a stale result changed the cursor")
	require.NoError(t, m.Err(), "a stale result changed the error state")

	_, cmd = m.Update(pollMsg{target: m, generation: generation})
	require.Nil(t, cmd, "a stale timer restarted the previous polling chain")
}

func assertReactivatedWindow(t *testing.T, m *Model, paused bool, offset int) {
	t.Helper()

	if paused {
		require.Len(t, m.lines, 5, "a pending live read changed the paused history window")
		require.Equal(t, "5", m.cursor, "a pending live read changed the cursor")
		require.Equal(t, offset, m.viewport.YOffset(), "a pending live read changed the scroll position")

		return
	}

	require.Len(t, m.lines, 6, "follow did not display new entries after reactivation")
	require.Equal(t, "6", m.cursor, "follow did not display new entries after reactivation")
	require.True(t, m.viewport.AtBottom(), "follow did not display new entries after reactivation")
}

func TestPendingTickKeepsGeneration(t *testing.T) {
	t.Parallel()

	m := New()
	m.Init()
	_, tick := m.Update(entriesMsg{target: m, generation: m.generation})
	require.NotNil(t, tick, "expected a polling timer")

	m.Init()
	_, cmd := m.Update(tick())
	require.Nil(t, cmd, "a timer created before reactivation must not join the new polling chain")
}

func TestHistoryPagingPreservesAnchor(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetWidth(80)
	m.SetHeight(3)
	m.applyEntries(entriesMsg{mode: readTail, entries: numberedEntries(maxLines+1, maxLines)})
	m.follow = false
	m.viewport.SetYOffset(10)
	anchor := m.lines[m.viewport.YOffset()]

	m.applyEntries(entriesMsg{
		mode: readOlder, entries: numberedEntries(maxLines-batchSize+1, batchSize), more: true,
	})
	require.Len(t, m.lines, maxLines, "loading history exceeded the buffer")
	require.Equal(t, anchor, m.lines[m.viewport.YOffset()], "loading history moved the visible entry")

	require.False(t, m.atTail, "the history window did not move backwards")
	require.Equal(t, strconv.Itoa(maxLines-batchSize+1), m.entries[0].cursor, "the history window did not move backwards")

	m.viewport.GotoBottom()
	anchor = m.lines[m.viewport.YOffset()]
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.NotNil(t, cmd, "the bottom of a historical window must load newer entries")
	require.False(t, m.follow, "the bottom of a historical window must not enable follow")
	require.True(t, m.loading, "the bottom of a historical window must load newer entries")

	m.Update(entriesMsg{
		target: m, generation: m.generation, mode: readNewer,
		entries: numberedEntries(2*maxLines-batchSize+1, batchSize),
	})
	require.Len(t, m.lines, maxLines, "paging forward did not preserve the visible entry")
	require.Equal(t, anchor, m.lines[m.viewport.YOffset()], "paging forward did not preserve the visible entry")
	require.False(t, m.follow, "paging forward did not preserve the pause")

	m.viewport.GotoBottom()
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.True(t, m.follow, "reaching the live tail must resume follow")
}

func TestHistoryLoadsOnceAndStopsAtJournalHead(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetWidth(80)
	m.SetHeight(3)
	m.applyEntries(entriesMsg{mode: readTail, entries: numberedEntries(10, 5)})
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.NotNil(t, cmd, "scrolling near the beginning must request older entries")
	require.True(t, m.loading, "scrolling near the beginning must request older entries")
	require.False(t, m.follow, "scrolling near the beginning must pause follow")

	generation := m.generation
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd, "repeated scrolling must not start concurrent history reads")
	require.Equal(t, generation, m.generation, "repeated scrolling must not start concurrent history reads")

	m.Update(entriesMsg{
		target: m, generation: generation, mode: readOlder, entries: numberedEntries(1, 9),
	})
	m.viewport.GotoTop()
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd, "history reads must stop at the actual beginning of the journal")
	require.True(t, m.atHead, "history reads must stop at the actual beginning of the journal")
}

func TestEndJumpsToTailAndRejectsPendingHistory(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetWidth(80)
	m.SetHeight(3)
	m.applyEntries(entriesMsg{mode: readTail, entries: numberedEntries(10, 5)})
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	generation := m.generation

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	require.NotNil(t, cmd, "End must request the live tail immediately")
	require.True(t, m.follow, "End must request the live tail immediately")

	_, cmd = m.Update(entriesMsg{
		target: m, generation: generation, mode: readOlder, entries: numberedEntries(1, 9),
	})
	require.Nil(t, cmd, "a pending history read scheduled another read after End")
	require.Len(t, m.entries, 5, "a pending history read changed the window after End")

	m.Update(entriesMsg{
		target: m, generation: m.generation, mode: readTail, entries: numberedEntries(1000, 5),
	})
	require.Len(t, m.entries, 5, "the tail snapshot must replace history, not append to it")
	require.Equal(t, "1000", m.entries[0].cursor, "the tail snapshot must replace history, not append to it")
	require.True(t, m.viewport.AtBottom(), "the tail snapshot must replace history, not append to it")
}

func TestEvictionKeepsWholeMultilineEntries(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetSize(80, 3)
	m.applyEntries(entriesMsg{mode: readTail, entries: []entry{
		{cursor: "1", text: strings.Repeat("line\n", maxLines-1) + "last"},
		{cursor: "2", text: "new"},
	}})
	require.Len(t, m.entries, 1, "eviction must not leave partial entries that cannot be reloaded by cursor")
	require.Len(t, m.lines, 1, "eviction must not leave partial entries that cannot be reloaded by cursor")
	require.Equal(t, "2", m.entries[0].cursor, "eviction must not leave partial entries that cannot be reloaded by cursor")

	m.follow = false
	m.applyEntries(entriesMsg{mode: readOlder, entries: []entry{
		{cursor: "1", text: strings.Repeat("line\n", maxLines) + "last"},
	}})
	require.Len(t, m.entries, 1, "a single oversized entry must remain readable in full")
	require.Len(t, m.lines, maxLines+1, "a single oversized entry must remain readable in full")
	require.Equal(t, "1", m.entries[0].cursor, "a single oversized entry must remain readable in full")
}

func TestWrappedNavigationAndFollow(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetSize(4, 2)
	m.applyEntries(entriesMsg{mode: readTail, entries: []entry{{cursor: "1", text: "abcdefghijklmnop"}}})
	m.atHead = true
	require.Equal(t, 2, m.viewport.YOffset(), "follow must start at the last screenful, not the last source line")

	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.False(t, m.follow, "Up must pause and move one wrapped row")
	require.Equal(t, 1, m.viewport.YOffset(), "Up must pause and move one wrapped row")

	m.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	require.Equal(t, 0, m.viewport.YOffset(), "PageUp must move by a screenful of wrapped rows")
	require.False(t, m.follow, "PageUp must move by a screenful of wrapped rows")

	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	require.True(t, m.follow, "PageDown to the actual bottom must resume follow")
	require.Equal(t, 2, m.viewport.YOffset(), "PageDown to the actual bottom must resume follow")

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	require.False(t, m.follow, "mouse scrolling must use wrapped rows")
	require.Equal(t, 0, m.viewport.YOffset(), "mouse scrolling must use wrapped rows")

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	require.True(t, m.follow, "mouse scrolling to the bottom must resume follow")
	require.True(t, m.viewport.AtBottom(), "mouse scrolling to the bottom must resume follow")

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp, Mod: tea.ModShift})
	require.True(t, m.follow, "horizontal navigation must not pause follow")

	m.SetSize(2, 3)
	m.applyEntries(entriesMsg{mode: readFollow, entries: []entry{{cursor: "2", text: "qrstuvwx"}}})
	require.True(t, m.follow, "resizing and new wrapped entries must keep follow at the bottom")
	require.Equal(t, 9, m.viewport.YOffset(), "resizing and new wrapped entries must keep follow at the bottom")
	require.True(t, m.viewport.AtBottom(), "resizing and new wrapped entries must keep follow at the bottom")
}

func TestReflowPreservesPausedTextPosition(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetSize(4, 6)
	m.applyEntries(entriesMsg{mode: readTail, entries: []entry{
		{cursor: "1", text: strings.Repeat("abcdefgh", 5)},
	}})
	m.follow = false
	position := m.position()

	// Reflowing at the old height first would clamp the offset to zero.
	m.SetSize(8, 2)
	require.False(t, m.follow, "simultaneous width/height changes must preserve the paused text position")
	require.Equal(t, position, m.position(), "simultaneous width/height changes must preserve the paused text position")
	require.Equal(t, 2, m.viewport.YOffset(), "simultaneous width/height changes must preserve the paused text position")

	m.SetSize(0, 0)
	require.Empty(t, m.View().Content, "a hidden viewport must render nothing")

	m.applyEntries(entriesMsg{mode: readOlder, entries: []entry{{cursor: "0", text: "older"}}})
	m.SetSize(4, 2)
	require.False(t, m.follow, "showing a resized viewport must retain its text anchor, including pending history")
	require.Equal(t, position, m.position(), "showing a resized viewport must retain its text anchor, including pending history")

	m.SetSize(80, 10)
	require.False(t, m.follow, "fitting all text on screen must not implicitly resume follow")
}

func TestWrappedHistoryPreservesAnchor(t *testing.T) {
	t.Parallel()

	m := New()
	m.SetSize(5, 3)
	m.applyEntries(entriesMsg{mode: readTail, entries: numberedEntries(maxLines+1, maxLines)})
	m.follow = false
	m.viewport.SetYOffset(maxLines + 1)
	position := m.position()

	m.applyEntries(entriesMsg{
		mode: readOlder, entries: numberedEntries(maxLines-batchSize+1, batchSize), more: true,
	})
	require.Len(t, m.entries, maxLines, "prepending history and evicting its tail must preserve the wrapped-row anchor")
	require.Len(t, m.lines, 2*maxLines, "prepending history and evicting its tail must preserve the wrapped-row anchor")
	require.Equal(t, position, m.position(), "prepending history and evicting its tail must preserve the wrapped-row anchor")

	m.applyEntries(entriesMsg{
		mode: readNewer, entries: numberedEntries(2*maxLines-batchSize+1, batchSize),
	})
	require.Len(t, m.entries, maxLines, "appending history and evicting its head must preserve the wrapped-row anchor")
	require.Len(t, m.lines, 2*maxLines, "appending history and evicting its head must preserve the wrapped-row anchor")
	require.Equal(t, position, m.position(), "appending history and evicting its head must preserve the wrapped-row anchor")

	m.SetWidth(3)
	require.Len(t, m.entries, maxLines, "narrowing the window must not evict history")
	require.False(t, m.follow, "narrowing the window must not evict history or resume follow")
}
