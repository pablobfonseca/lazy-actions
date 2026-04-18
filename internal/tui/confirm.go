package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type confirmModel struct {
	Message string
	onYes   tea.Cmd
	open    bool
}

func newConfirm() *confirmModel { return &confirmModel{} }

// Open shows the modal with a message and a command to run on confirmation.
func (c *confirmModel) Open(message string, onYes tea.Cmd) {
	c.Message = message
	c.onYes = onYes
	c.open = true
}

func (c *confirmModel) Close() {
	c.open = false
	c.onYes = nil
	c.Message = ""
}

func (c *confirmModel) IsOpen() bool { return c.open }

// Update returns (handled, cmd). Caller must close the modal when handled.
// cmd is non-nil only when the user confirmed.
func (c *confirmModel) Update(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !c.open {
		return false, nil
	}
	switch msg.String() {
	case "y", "enter":
		return true, c.onYes
	case "n", "esc":
		return true, nil
	}
	return false, nil
}

func (c *confirmModel) View(width int) string {
	boxWidth := width - 8
	if boxWidth > 60 {
		boxWidth = 60
	}
	box := modalBorderStyle.Width(boxWidth).Render(
		c.Message + "\n\n" + dimStyle.Render("y/enter: confirm   n/esc: cancel"),
	)
	return lipgloss.Place(width, 0, lipgloss.Center, lipgloss.Top, box)
}
