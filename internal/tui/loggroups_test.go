package tui

import (
	"reflect"
	"testing"
)

// ts is a fixed, realistic timestamp prefix. Every raw log line from the
// GitHub Actions API starts with one of these.
const ts = "2026-04-21T10:00:00.0000000Z "

func TestParseLogSections_SplitsGroupsAndOrphans(t *testing.T) {
	lines := []string{
		ts + "Starting job",
		ts + "##[group]Setup node",
		ts + "downloading…",
		ts + "installed",
		ts + "##[endgroup]",
		ts + "running tests",
		ts + "##[group]Upload artifact",
		ts + "uploading…",
		ts + "##[endgroup]",
	}

	got := parseLogSections(lines)

	if len(got) != 4 {
		t.Fatalf("want 4 sections, got %d: %+v", len(got), got)
	}
	if got[0].isGroup() || len(got[0].lines) != 1 {
		t.Errorf("section 0 should be orphan run with 1 line, got %+v", got[0])
	}
	if !got[1].isGroup() || got[1].title != "Setup node" || len(got[1].lines) != 2 {
		t.Errorf("section 1 should be 'Setup node' group with 2 body lines, got %+v", got[1])
	}
	if got[2].isGroup() || len(got[2].lines) != 1 {
		t.Errorf("section 2 should be orphan run with 1 line, got %+v", got[2])
	}
	if !got[3].isGroup() || got[3].title != "Upload artifact" || len(got[3].lines) != 1 {
		t.Errorf("section 3 should be 'Upload artifact' group with 1 body line, got %+v", got[3])
	}
}

func TestParseLogSections_UnclosedGroupConsumesRest(t *testing.T) {
	lines := []string{
		ts + "##[group]Run",
		ts + "line a",
		ts + "line b",
	}
	got := parseLogSections(lines)
	if len(got) != 1 {
		t.Fatalf("want 1 section, got %d", len(got))
	}
	if !got[0].isGroup() || got[0].title != "Run" || len(got[0].lines) != 2 {
		t.Errorf("unclosed group should absorb remaining lines, got %+v", got[0])
	}
}

// TODO: implement TestSectionForRawLine.
//
// Purpose: sectionForRawLine must return the section that "owns" a raw line
// index for ANY raw index in [0, len(lines)). The tricky part is that
// section.lines excludes the ##[group] header AND the ##[endgroup] line —
// but those lines DO exist in the raw input and search matches can land on
// them.
//
// Pick coverage that pins the behavior. At minimum, assert:
//   - An orphan line before any group maps to the orphan section.
//   - A group header line maps to that group's section.
//   - A line inside a group body maps to that group's section.
//   - A ##[endgroup] line maps to the group it closes (NOT the next section).
//   - An orphan line after a group maps to the trailing orphan section.
//
// Use the same `lines` fixture as TestParseLogSections_SplitsGroupsAndOrphans
// so the expected section indices are easy to reason about.
func TestSectionForRawLine(t *testing.T) {
	t.Skip("implement me — see comment above")
	_ = reflect.DeepEqual // silences unused import until you implement
}
