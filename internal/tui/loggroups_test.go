package tui

import "testing"

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

func TestSectionForRawLine(t *testing.T) {
	// Raw indices map out as:
	//   0: orphan           -> section 0 (orphan)
	//   1: ##[group]Setup   -> section 1 (group)
	//   2: body             -> section 1
	//   3: body             -> section 1
	//   4: ##[endgroup]     -> section 1  (closes group, not the next section)
	//   5: orphan           -> section 2 (orphan)
	//   6: ##[group]Upload  -> section 3 (group)
	//   7: body             -> section 3
	//   8: ##[endgroup]     -> section 3
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
	sections := parseLogSections(lines)

	want := []int{0, 1, 1, 1, 1, 2, 3, 3, 3}
	for raw, expected := range want {
		got := sectionForRawLine(sections, raw)
		if got != expected {
			t.Errorf("rawIdx=%d: got section %d, want %d", raw, got, expected)
		}
	}
}

func TestSectionForRawLine_OutOfRange(t *testing.T) {
	sections := parseLogSections([]string{ts + "only line"})
	if got := sectionForRawLine(sections, 99); got != -1 {
		t.Errorf("out-of-range index should return -1, got %d", got)
	}
}
