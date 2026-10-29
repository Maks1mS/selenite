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

	"github.com/charmbracelet/x/ansi"
	"github.com/coreos/go-systemd/v22/sdjournal"
)

// Record is a journal entry formatted for display.
type Record struct {
	Cursor string
	Text   string
}

// Reader reads journal records.
type Reader interface {
	// Read returns up to a batch of records after cursor, or before it when
	// older is set, in chronological order. An empty cursor reads the most
	// recent records. more reports whether further records are available.
	// Records read before an error are returned along with it.
	Read(cursor string, older bool) (records []Record, more bool, err error)
}

// SystemReader reads system messages from the systemd journal.
type SystemReader struct{}

// Each call owns its journal handle, including its OS thread. Reopening by
// cursor avoids a persistent goroutine or descriptor requiring parent cleanup.
func (SystemReader) Read(cursor string, older bool) ([]Record, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	reader, err := sdjournal.NewJournal()
	if err != nil {
		return nil, false, fmt.Errorf("open journal: %w", err)
	}
	defer func() { _ = reader.Close() }()

	if err := applyFilters(reader); err != nil {
		return nil, false, fmt.Errorf("filter journal: %w", err)
	}

	available, err := seek(reader, cursor, older)
	if err != nil {
		return nil, false, fmt.Errorf("seek journal: %w", err)
	}

	step := reader.Next
	if older {
		step = reader.Previous
	}

	var records []Record
	for count := 0; available > 0 && count < batchSize; count++ {
		journalEntry, entryErr := reader.GetEntry()
		if entryErr != nil {
			err = fmt.Errorf("read journal: %w", entryErr)

			break
		}

		records = append(records, Record{
			Cursor: journalEntry.Cursor,
			Text:   formatEntry(journalEntry),
		})

		available, err = step()
		if err != nil {
			err = fmt.Errorf("advance journal: %w", err)

			break
		}
	}

	if older {
		slices.Reverse(records)
	}

	return records, available > 0, err
}

func seek(reader *sdjournal.Journal, cursor string, older bool) (uint64, error) {
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
	if older {
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

// filterGroups are OR-ed groups of AND-ed matches. The console log units show
// the same records, see selenite-console-log.service.
var filterGroups = [][]string{
	{"_TRANSPORT=kernel"},
	{"_TRANSPORT=syslog"},
	{"_TRANSPORT=driver"},
	{"_PID=1"},
	// Podman events from the controller cgroup use _TRANSPORT=journal and are left out.
	{"_SYSTEMD_UNIT=selenite-controller.service", "_TRANSPORT=stdout"},
}

func applyFilters(reader *sdjournal.Journal) error {
	for i, group := range filterGroups {
		if i > 0 {
			if err := reader.AddDisjunction(); err != nil {
				return fmt.Errorf("add disjunction: %w", err)
			}
		}

		for _, match := range group {
			if err := reader.AddMatch(match); err != nil {
				return fmt.Errorf("add match: %w", err)
			}
		}
	}

	return nil
}
