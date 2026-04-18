package main

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
	rendered   string
	matches    []int
	matchIndex int
	query      string
	title      string
	open       bool
	loading    bool
	err        error
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
	v.vp.Width = width
	v.vp.Height = height - 3
	v.vp.SetContent("")
}

func (v *logViewer) Close() { v.open = false }

func (v *logViewer) SetContent(lines []string, err error) {
	v.loading = false
	v.err = err
	v.lines = lines
	v.rendered = strings.Join(lines, "\n")
	v.vp.SetContent(v.rendered)
	v.vp.GotoBottom()
}

// Update returns (cmd, consumed). When consumed is true the caller should not
// process the key further. When the viewer closes, callers should switch mode.
func (v *logViewer) Update(msg tea.KeyMsg) (tea.Cmd, bool) {
	if v.searching {
		switch msg.String() {
		case "enter":
			v.query = v.search.Value()
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
	case "g":
		v.vp.GotoTop()
		return nil, true
	case "G":
		v.vp.GotoBottom()
		return nil, true
	case "esc", "q":
		v.Close()
		return nil, true
	}

	var cmd tea.Cmd
	v.vp, cmd = v.vp.Update(msg)
	return cmd, true
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
	v.vp.SetContent(v.renderWithHighlights())
}

func (v *logViewer) jumpToMatch(delta int) {
	if len(v.matches) == 0 {
		return
	}
	v.matchIndex = (v.matchIndex + delta + len(v.matches)) % len(v.matches)
	line := v.matches[v.matchIndex]
	v.vp.SetYOffset(line)
}

func (v *logViewer) renderWithHighlights() string {
	if v.query == "" {
		return v.rendered
	}
	q := strings.ToLower(v.query)
	var b strings.Builder
	for _, ln := range v.lines {
		lower := strings.ToLower(ln)
		idx := strings.Index(lower, q)
		if idx == -1 {
			b.WriteString(ln)
			b.WriteString("\n")
			continue
		}
		b.WriteString(ln[:idx])
		b.WriteString(searchMatchStyle.Render(ln[idx : idx+len(v.query)]))
		b.WriteString(ln[idx+len(v.query):])
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
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
		status = dimStyle.Render("/: search   n/N: next/prev   g/G: top/bottom   esc: close")
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, body, status)
}

func fmtMatches(idx, total int, q string) string {
	if total == 0 {
		return "no matches for " + q
	}
	return "match " + strconv.Itoa(idx+1) + "/" + strconv.Itoa(total) + " · " + q
}
