package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// promptModel is a one-line text prompt with a submit callback.
// It's modal-adjacent: when open it owns all key input until enter/esc.
type promptModel struct {
	input    textinput.Model
	label    string
	onSubmit func(string) tea.Cmd
	open     bool
}

func newPrompt() *promptModel {
	ti := textinput.New()
	ti.CharLimit = 256
	return &promptModel{input: ti}
}

func (p *promptModel) Open(label, defaultValue string, onSubmit func(string) tea.Cmd) {
	p.label = label
	p.input.SetValue(defaultValue)
	p.input.CursorEnd()
	p.input.Focus()
	p.onSubmit = onSubmit
	p.open = true
}

func (p *promptModel) Close() {
	p.open = false
	p.onSubmit = nil
	p.input.Blur()
	p.input.SetValue("")
}

func (p *promptModel) IsOpen() bool { return p.open }

// Update returns (handled, cmd). On enter the caller should Close().
// cmd is non-nil only when the user submitted.
func (p *promptModel) Update(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !p.open {
		return false, nil
	}
	switch msg.String() {
	case "enter":
		v := p.input.Value()
		if p.onSubmit != nil && v != "" {
			return true, p.onSubmit(v)
		}
		return true, nil
	case "esc":
		return true, nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return false, cmd
}

func (p *promptModel) View() string {
	return "  " + dimStyle.Render(p.label+" ") + p.input.View()
}
