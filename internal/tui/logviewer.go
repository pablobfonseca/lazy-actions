package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type logViewer struct {
	vp         viewport.Model
	search     textinput.Model
	searching  bool
	lines      []string
	matches    []int
	matchIndex int
	query      string
	title      string
	open       bool
	loading    bool
	err        error

	sections       []logSection
	collapsed      map[int]bool
	forceExpand    map[int]bool
	current        int   // current section index
	sectionStarts  []int // rendered-line index where each section begins
	lineToSection  []int // rendered-line index -> section index
}

func newLogViewer() *logViewer {
	ti := textinput.New()
	ti.Placeholder = "/search"
	ti.CharLimit = 128
	return &logViewer{
		vp:     viewport.New(0, 0),
		search: ti,
	}
}

func (v *logViewer) IsOpen() bool { return v.open }

func (v *logViewer) Open(title string, width, height int) {
	v.open = true
	v.loading = true
	v.title = title
	v.lines = nil
	v.err = nil
	v.query = ""
	v.matches = nil
	v.matchIndex = 0
	v.searching = false
	v.sections = nil
	v.collapsed = map[int]bool{}
	v.forceExpand = map[int]bool{}
	v.current = 0
	if width < 1 {
		width = 1
	}
	vpHeight := height - 3
	if vpHeight < 1 {
		vpHeight = 1
	}
	v.vp.Width = width
	v.vp.Height = vpHeight
	v.vp.SetContent("")
}

func (v *logViewer) Close() { v.open = false }

func (v *logViewer) SetContent(lines []string, err error) {
	v.loading = false
	v.err = err
	v.lines = lines
	v.sections = parseLogSections(lines)
	v.collapsed = map[int]bool{}
	// Collapse every group by default, except ones containing errors — if a
	// user opens logs it's almost always to find a failure, so put the user
	// on the error with as few keystrokes as possible.
	for i, sec := range v.sections {
		if sec.isGroup() && !sec.hasError {
			v.collapsed[i] = true
		}
	}
	v.forceExpand = map[int]bool{}
	v.current = firstErrorOrGroupIndex(v.sections)
	v.rerender()
	if v.current >= 0 && v.current < len(v.sectionStarts) {
		v.scrollToSection(v.current)
	} else {
		v.vp.GotoTop()
	}
}

// Update returns (cmd, consumed). When consumed is true the caller should not
// process the key further. When the viewer closes, callers should switch mode.
func (v *logViewer) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	if v.searching {
		switch msg.String() {
		case "enter":
			v.query = v.search.Value()
			v.forceExpand = sectionsContainingQuery(v.sections, v.query)
			v.rerender()
			v.computeMatches()
			v.searching = false
			v.search.Blur()
			v.jumpToMatch(0)
			return nil, true
		case "esc":
			v.searching = false
			v.search.Blur()
			v.search.SetValue("")
			return nil, true
		}
		var cmd tea.Cmd
		v.search, cmd = v.search.Update(msg)
		return cmd, true
	}

	switch msg.String() {
	case "/":
		v.searching = true
		v.search.Focus()
		v.search.SetValue("")
		return nil, true
	case "n":
		v.jumpToMatch(+1)
		return nil, true
	case "N":
		v.jumpToMatch(-1)
		return nil, true
	case "tab":
		v.jumpToSection(+1)
		return nil, true
	case "shift+tab":
		v.jumpToSection(-1)
		return nil, true
	case " ", "enter":
		v.toggleCurrent()
		return nil, true
	case "Z":
		v.expandAll()
		return nil, true
	case "X":
		v.collapseAll()
		return nil, true
	case "g":
		v.vp.GotoTop()
		v.current = firstGroupIndex(v.sections)
		v.rerender()
		return nil, true
	case "G":
		v.vp.GotoBottom()
		if len(v.sections) > 0 {
			v.current = len(v.sections) - 1
			v.rerender()
		}
		return nil, true
	case "esc", "q":
		v.Close()
		return nil, true
	}

	var cmd tea.Cmd
	v.vp, cmd = v.vp.Update(msg)
	v.syncCurrentFromScroll()
	return cmd, true
}

func (v *logViewer) rerender() {
	body, starts, lineMap := renderSections(v.sections, v.collapsed, v.forceExpand, v.current, v.query)
	v.sectionStarts = starts
	v.lineToSection = lineMap
	v.vp.SetContent(body)
}

func (v *logViewer) toggleCurrent() {
	if v.current < 0 || v.current >= len(v.sections) {
		return
	}
	if !v.sections[v.current].isGroup() {
		return
	}
	v.collapsed[v.current] = !v.collapsed[v.current]
	v.rerender()
	v.scrollToSection(v.current)
}

func (v *logViewer) expandAll() {
	for i, sec := range v.sections {
		if sec.isGroup() {
			v.collapsed[i] = false
		}
	}
	v.rerender()
	v.scrollToSection(v.current)
}

func (v *logViewer) collapseAll() {
	for i, sec := range v.sections {
		if sec.isGroup() {
			v.collapsed[i] = true
		}
	}
	v.rerender()
	v.scrollToSection(v.current)
}

// jumpToSection walks delta steps through the section list, skipping orphan
// runs so Tab feels like "next group, not next paragraph".
func (v *logViewer) jumpToSection(delta int) {
	if len(v.sections) == 0 {
		return
	}
	i := v.current
	step := 1
	if delta < 0 {
		step = -1
	}
	for k := 0; k < len(v.sections); k++ {
		i += step
		if i < 0 {
			i = len(v.sections) - 1
		}
		if i >= len(v.sections) {
			i = 0
		}
		if v.sections[i].isGroup() {
			v.current = i
			v.rerender()
			v.scrollToSection(i)
			return
		}
	}
}

func (v *logViewer) scrollToSection(i int) {
	if i < 0 || i >= len(v.sectionStarts) {
		return
	}
	v.vp.SetYOffset(v.sectionStarts[i])
}

// syncCurrentFromScroll keeps the "current section" in sync with the topmost
// visible line while the user scrolls with arrow keys / pgdn. We don't
// re-render on every scroll — just update the index so the next Tab/Space
// operates on what the user is looking at.
func (v *logViewer) syncCurrentFromScroll() {
	if len(v.lineToSection) == 0 {
		return
	}
	top := v.vp.YOffset
	if top >= len(v.lineToSection) {
		top = len(v.lineToSection) - 1
	}
	if top < 0 {
		top = 0
	}
	prev := v.current
	v.current = v.lineToSection[top]
	if v.current != prev {
		v.rerender()
	}
}

func firstGroupIndex(sections []logSection) int {
	for i, s := range sections {
		if s.isGroup() {
			return i
		}
	}
	return 0
}

// firstErrorOrGroupIndex prefers the first group that contains an error, so
// opening a failing run lands the cursor directly on the problem. Falls back
// to the first group of any kind when no errors exist.
func firstErrorOrGroupIndex(sections []logSection) int {
	for i, s := range sections {
		if s.isGroup() && s.hasError {
			return i
		}
	}
	return firstGroupIndex(sections)
}

func (v *logViewer) computeMatches() {
	v.matches = nil
	if v.query == "" {
		return
	}
	q := strings.ToLower(v.query)
	for i, ln := range v.lines {
		if strings.Contains(strings.ToLower(ln), q) {
			v.matches = append(v.matches, i)
		}
	}
	v.matchIndex = 0
}

// jumpToMatch advances through raw-line matches, mapping each back to its
// section and expanding that section so the hit is visible in the viewport.
func (v *logViewer) jumpToMatch(delta int) {
	if len(v.matches) == 0 {
		return
	}
	v.matchIndex = (v.matchIndex + delta + len(v.matches)) % len(v.matches)
	rawLine := v.matches[v.matchIndex]
	sec := sectionForRawLine(v.sections, rawLine)
	if sec >= 0 {
		v.forceExpand[sec] = true
		v.current = sec
		v.rerender()
		v.scrollToSection(sec)
	}
}

// sectionForRawLine maps an index into the flat input `lines` slice back to
// the section that contains it. Group sections span their header line, body
// lines, and (if closed) the trailing ##[endgroup] line.
func sectionForRawLine(sections []logSection, rawIdx int) int {
	cursor := 0
	for i, s := range sections {
		span := len(s.lines)
		if s.isGroup() {
			span++ // header line
			if s.closed {
				span++ // ##[endgroup] line
			}
		}
		if rawIdx >= cursor && rawIdx < cursor+span {
			return i
		}
		cursor += span
	}
	return -1
}

func (v *logViewer) View(width, height int) string {
	title := titleStyle.Render(v.title)
	var body string
	switch {
	case v.loading:
		body = dimStyle.Render("loading…")
	case v.err != nil:
		body = failureStyle.Render("failed to load logs: " + v.err.Error())
	default:
		body = v.vp.View()
	}
	var status string
	switch {
	case v.searching:
		status = v.search.View()
	case v.query != "":
		status = dimStyle.Render(fmtMatches(v.matchIndex, len(v.matches), v.query))
	default:
		status = dimStyle.Render("tab: next group   space: fold/unfold   Z/X: expand/collapse all   /: search   g/G: top/bottom   esc: close")
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, status)
}

func fmtMatches(idx, total int, q string) string {
	if total == 0 {
		return "no matches for " + q
	}
	return "match " + strconv.Itoa(idx+1) + "/" + strconv.Itoa(total) + " · " + q
}
