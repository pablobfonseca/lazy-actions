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

func TestParseLogSections_FlagsErrorsAndWarnings(t *testing.T) {
	lines := []string{
		ts + "##[group]Build",
		ts + "compiling…",
		ts + "##[warning]deprecated API",
		ts + "##[endgroup]",
		ts + "##[group]Test",
		ts + "##[error]assertion failed",
		ts + "##[endgroup]",
		ts + "##[group]Upload",
		ts + "done",
		ts + "##[endgroup]",
	}
	got := parseLogSections(lines)
	if len(got) != 3 {
		t.Fatalf("want 3 sections, got %d", len(got))
	}
	if !got[0].hasWarning || got[0].hasError {
		t.Errorf("Build section: want warn-only, got err=%v warn=%v", got[0].hasError, got[0].hasWarning)
	}
	if !got[1].hasError || got[1].hasWarning {
		t.Errorf("Test section: want error-only, got err=%v warn=%v", got[1].hasError, got[1].hasWarning)
	}
	if got[2].hasError || got[2].hasWarning {
		t.Errorf("Upload section: want clean, got err=%v warn=%v", got[2].hasError, got[2].hasWarning)
	}
}

func TestFirstErrorOrGroupIndex_PrefersErrorGroup(t *testing.T) {
	sections := []logSection{
		{lines: []string{"orphan"}},                                // 0: orphan
		{header: "h", title: "Setup", lines: []string{"ok"}},       // 1: clean group
		{header: "h", title: "Build", lines: []string{"err"}, hasError: true}, // 2: error group
		{header: "h", title: "Upload", lines: []string{"ok"}},      // 3: clean group
	}
	if got := firstErrorOrGroupIndex(sections); got != 2 {
		t.Errorf("want 2 (error group), got %d", got)
	}
}

func TestFirstErrorOrGroupIndex_FallsBackToFirstGroup(t *testing.T) {
	sections := []logSection{
		{lines: []string{"orphan"}},
		{header: "h", title: "Setup", lines: []string{"ok"}},
	}
	if got := firstErrorOrGroupIndex(sections); got != 1 {
		t.Errorf("want 1 (first group), got %d", got)
	}
}

func TestCountMatches(t *testing.T) {
	sec := logSection{lines: []string{"hello world", "HELLO again", "nothing"}}
	if n := countMatches(sec, ""); n != 0 {
		t.Errorf("empty query: want 0, got %d", n)
	}
	if n := countMatches(sec, "hello"); n != 2 {
		t.Errorf("case-insensitive: want 2, got %d", n)
	}
	if n := countMatches(sec, "missing"); n != 0 {
		t.Errorf("no hits: want 0, got %d", n)
	}
}

func TestSectionForRawLine_OutOfRange(t *testing.T) {
	sections := parseLogSections([]string{ts + "only line"})
	if got := sectionForRawLine(sections, 99); got != -1 {
		t.Errorf("out-of-range index should return -1, got %d", got)
	}
}
