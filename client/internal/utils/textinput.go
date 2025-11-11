package utils

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type TextModel struct {
	input textinput.Model
	err   error

	Value  string
	Prompt string
}

func NewTextModel(prompt string, limit int) TextModel {
	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = limit
	if limit != 0 {
		ti.Width = 20
	}

	return TextModel{
		input:  ti,
		Value:  "",
		Prompt: prompt,
	}
}

func (m TextModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *TextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	case error:
		m.err = msg
		return m, nil
	}

	m.input, cmd = m.input.Update(msg)
	m.Value = m.input.Value()
	return m, cmd
}

func (m TextModel) View() string {
	return fmt.Sprintf(
		"%s\n\n%s",
		m.Prompt,
		m.input.View(),
	) + "\n"
}

func GetTextInput(prompt string, limit int) string {
	m := NewTextModel(prompt, limit)
	p := tea.NewProgram(&m)

	if _, err := p.Run(); err != nil {
		panic(fmt.Errorf("tea: %w", err))
	}

	return m.Value
}
