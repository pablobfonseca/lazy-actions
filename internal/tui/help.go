package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpModel struct {
	open bool
}

func newHelp() *helpModel         { return &helpModel{} }
func (h *helpModel) Toggle()      { h.open = !h.open }
func (h *helpModel) Close()       { h.open = false }
func (h *helpModel) IsOpen() bool { return h.open }

func (h *helpModel) View(width, height int) string {
	var b strings.Builder
	for i, section := range keyMap {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionLabelStyle.Render(section.Title))
		b.WriteString("\n")
		for _, kb := range section.Bindings {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(strings.Join(kb.Keys, "/")))
			b.WriteString("   ")
			b.WriteString(kb.Desc)
			b.WriteString("\n")
		}
	}
	boxWidth := width - 8
	if boxWidth > 60 {
		boxWidth = 60
	}
	box := modalBorderStyle.Width(boxWidth).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
