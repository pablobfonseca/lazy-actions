package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	groupPrefix    = "##[group]"
	endGroupPrefix = "##[endgroup]"
	errorPrefix    = "##[error]"
	warningPrefix  = "##[warning]"
	groupIndent    = "  "
)

// logSection is either a group (header != "") or a run of orphan lines
// (header == ""). Orphan runs are coalesced so the user navigates between
// meaningful blocks, not individual lines.
type logSection struct {
	header     string   // raw header line, e.g. "2026-...Z ##[group]Setup node". Empty for orphan run.
	title      string   // the group name stripped of timestamp + "##[group]" prefix. Empty for orphan.
	lines      []string // content lines (excludes the header and ##[endgroup]).
	closed     bool     // true if this group ended with an explicit ##[endgroup]. Groups only.
	hasError   bool     // true if any body line starts with ##[error]
	hasWarning bool     // true if any body line starts with ##[warning]
}

func (s logSection) isGroup() bool { return s.header != "" }

// parseLogSections walks raw log lines and groups them by ##[group]/##[endgroup].
// Unclosed groups at end-of-input are terminated implicitly. Nested groups are
// not supported: GitHub Actions does not emit them, and an inner ##[group]
// within a group's body is treated as ordinary content until the first
// ##[endgroup] closes the outer group.
func parseLogSections(lines []string) []logSection {
	var sections []logSection
	var orphan []string
	flushOrphan := func() {
		if len(orphan) == 0 {
			return
		}
		hasErr, hasWarn := scanSeverity(orphan)
		sections = append(sections, logSection{lines: orphan, hasError: hasErr, hasWarning: hasWarn})
		orphan = nil
	}

	i := 0
	for i < len(lines) {
		ln := lines[i]
		rest := stripTimestamp(ln)

		if strings.HasPrefix(rest, groupPrefix) {
			flushOrphan()
			title := strings.TrimPrefix(rest, groupPrefix)
			body := make([]string, 0, 8)
			j := i + 1
			for j < len(lines) {
				inner := stripTimestamp(lines[j])
				if strings.HasPrefix(inner, endGroupPrefix) {
					break
				}
				body = append(body, lines[j])
				j++
			}
			closed := j < len(lines)
			hasErr, hasWarn := scanSeverity(body)
			sections = append(sections, logSection{
				header: ln, title: title, lines: body, closed: closed,
				hasError: hasErr, hasWarning: hasWarn,
			})
			if closed {
				i = j + 1
			} else {
				i = j
			}
			continue
		}

		orphan = append(orphan, ln)
		i++
	}
	flushOrphan()
	return sections
}

// stripTimestamp returns the line without its leading ISO-8601 timestamp,
// reusing the same detector logic as colorize.go.
func stripTimestamp(line string) string {
	if i := timestampEnd(line); i > 0 {
		return line[i:]
	}
	return line
}

// scanSeverity reports whether any line carries an ##[error] or ##[warning]
// prefix (after the timestamp is stripped). Used to flag sections so they
// can auto-expand and show a badge.
func scanSeverity(lines []string) (hasError, hasWarning bool) {
	for _, ln := range lines {
		rest := stripTimestamp(ln)
		switch {
		case strings.HasPrefix(rest, errorPrefix):
			hasError = true
		case strings.HasPrefix(rest, warningPrefix):
			hasWarning = true
		}
		if hasError && hasWarning {
			return
		}
	}
	return
}

// sectionLineCount counts how many content lines a section has.
func sectionLineCount(s logSection) int { return len(s.lines) }

// renderSections produces the viewport body text plus metadata the viewer
// uses for navigation and search.
//
// collapsed[i] == true means section i is folded (group sections only; orphan
// runs are never collapsed). forceExpand is a set of section indices that
// must render expanded regardless of the collapsed map (used when a search
// query forces visibility of hits).
//
// Returns:
//   - body: the rendered text
//   - sectionStarts: rendered-line-index where each section begins
//   - lineToSection: rendered-line-index -> section index (len == rendered line count)
//   - query matches are painted if query != "".
func renderSections(
	sections []logSection,
	collapsed map[int]bool,
	forceExpand map[int]bool,
	current int,
	query string,
) (body string, sectionStarts []int, lineToSection []int) {
	var b strings.Builder
	sectionStarts = make([]int, len(sections))
	lineToSection = make([]int, 0, 256)
	lineIdx := 0

	emit := func(s string, section int) {
		b.WriteString(s)
		b.WriteString("\n")
		lineToSection = append(lineToSection, section)
		lineIdx++
	}

	for i, sec := range sections {
		sectionStarts[i] = lineIdx

		if !sec.isGroup() {
			for _, ln := range sec.lines {
				emit(renderLineWithQuery(ln, query), i)
			}
			continue
		}

		folded := collapsed[i] && !forceExpand[i]
		matchCount := countMatches(sec, query)
		emit(renderGroupHeader(sec, folded, current == i, matchCount), i)
		if folded {
			continue
		}
		for _, ln := range sec.lines {
			emit(groupIndent+renderLineWithQuery(ln, query), i)
		}
	}

	return strings.TrimRight(b.String(), "\n"), sectionStarts, lineToSection
}

// countMatches returns the number of body lines in sec that contain query
// (case-insensitive). Returns 0 when query is empty.
func countMatches(sec logSection, query string) int {
	if query == "" {
		return 0
	}
	q := strings.ToLower(query)
	n := 0
	for _, ln := range sec.lines {
		if strings.Contains(strings.ToLower(ln), q) {
			n++
		}
	}
	return n
}

func renderGroupHeader(sec logSection, folded, selected bool, matchCount int) string {
	icon := "▼"
	if folded {
		icon = "▶"
	}
	title := sec.title
	if title == "" {
		title = "(group)"
	}

	var parts []string
	if folded {
		n := sectionLineCount(sec)
		unit := "lines"
		if n == 1 {
			unit = "line"
		}
		parts = append(parts, strconv.Itoa(n)+" "+unit)
	}
	if matchCount > 0 {
		unit := "matches"
		if matchCount == 1 {
			unit = "match"
		}
		parts = append(parts, strconv.Itoa(matchCount)+" "+unit)
	}
	meta := ""
	if len(parts) > 0 {
		meta = dimStyle.Render(" (" + strings.Join(parts, " · ") + ")")
	}

	badge := ""
	switch {
	case sec.hasError:
		badge = " " + logErrorStyle.Render("✗")
	case sec.hasWarning:
		badge = " " + logWarningStyle.Render("⚠")
	}

	head := logGroupHeaderStyle.Render(icon+" "+title) + badge
	if selected {
		return logGroupSelectedStyle.Render("▸ ") + head + meta
	}
	return "  " + head + meta
}

// renderLineWithQuery applies timestamp/level coloring and highlights query hits.
// Mirrors logviewer.renderWithHighlights for a single line.
func renderLineWithQuery(line, query string) string {
	if query == "" {
		return colorizeLogLine(line)
	}
	q := strings.ToLower(query)
	lower := strings.ToLower(line)
	idx := strings.Index(lower, q)
	if idx == -1 {
		return colorizeLogLine(line)
	}
	var b strings.Builder
	b.WriteString(colorizeLogLine(line[:idx]))
	b.WriteString(searchMatchStyle.Render(line[idx : idx+len(query)]))
	b.WriteString(colorizeLogLine(line[idx+len(query):]))
	return b.String()
}

// sectionsContainingQuery returns the set of section indices whose content
// (or group title) contains the query. Used to force-expand matching groups
// while a search is active.
func sectionsContainingQuery(sections []logSection, query string) map[int]bool {
	out := map[int]bool{}
	if query == "" {
		return out
	}
	q := strings.ToLower(query)
	for i, sec := range sections {
		if sec.isGroup() && strings.Contains(strings.ToLower(sec.title), q) {
			out[i] = true
			continue
		}
		for _, ln := range sec.lines {
			if strings.Contains(strings.ToLower(ln), q) {
				out[i] = true
				break
			}
		}
	}
	return out
}

var (
	logGroupHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "97", Dark: "141"}).
				Bold(true)

	logGroupSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"}).
				Bold(true)
)
