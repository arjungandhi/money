package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// Simple confirmation model
type confirmModel struct {
	question string
	selected int
	done     bool
	result   bool
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "left", "h":
			m.selected = 0
		case "right", "l":
			m.selected = 1
		case "enter":
			m.result = m.selected == 0
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		return ""
	}

	s := fmt.Sprintf("%s\n\n", m.question)

	if m.selected == 0 {
		s += "[Yes] No"
	} else {
		s += "Yes [No]"
	}

	s += "\n\n← → to choose, Enter to confirm, q to quit"
	return s
}

// Simple input model using textinput
type inputModel struct {
	textInput textinput.Model
	question  string
	done      bool
	validator func(string) error
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.validator != nil {
				if err := m.validator(m.textInput.Value()); err != nil {
					return m, nil
				}
			}
			m.done = true
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	if m.done {
		return ""
	}

	s := fmt.Sprintf("%s\n\n", m.question)
	s += m.textInput.View()

	if m.validator != nil && m.textInput.Value() != "" {
		if err := m.validator(m.textInput.Value()); err != nil {
			s += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: "+err.Error())
		}
	}

	s += "\n\nEnter to confirm, Esc to cancel"
	return s
}

// List item for selections
type listItem struct {
	title, desc string
	value       string
}

func (i listItem) FilterValue() string { return i.title }
func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }

// Simple list model
type listModel struct {
	list   list.Model
	done   bool
	result *listItem
}

func (m listModel) Init() tea.Cmd {
	return nil
}

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "enter":
			if selected := m.list.SelectedItem(); selected != nil {
				item := selected.(listItem)
				m.result = &item
				m.done = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string {
	if m.done {
		return ""
	}
	return m.list.View()
}


// Helper functions using standard components

func RunConfirmation(question string) bool {
	model := confirmModel{
		question: question,
		selected: 1, // Default to No
	}

	p := tea.NewProgram(model)
	finalModel, err := p.StartReturningModel()
	if err != nil {
		return false
	}

	result := finalModel.(confirmModel)
	return result.result
}

func RunInput(prompt, placeholder string) string {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()

	model := inputModel{
		textInput: ti,
		question:  prompt,
	}

	p := tea.NewProgram(model)
	finalModel, err := p.StartReturningModel()
	if err != nil {
		return ""
	}

	result := finalModel.(inputModel)
	return result.textInput.Value()
}

func RunInputWithValidator(prompt, placeholder string, validator func(string) error) string {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()

	model := inputModel{
		textInput: ti,
		question:  prompt,
		validator: validator,
	}

	p := tea.NewProgram(model)
	finalModel, err := p.StartReturningModel()
	if err != nil {
		return ""
	}

	result := finalModel.(inputModel)
	return result.textInput.Value()
}

func RunMaskedInput(prompt, placeholder string) string {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.EchoMode = textinput.EchoPassword
	ti.Focus()

	model := inputModel{
		textInput: ti,
		question:  prompt,
	}

	p := tea.NewProgram(model)
	finalModel, err := p.StartReturningModel()
	if err != nil {
		return ""
	}

	result := finalModel.(inputModel)
	return result.textInput.Value()
}

type SelectOption struct {
	Label       string
	Value       string
	Description string
}

func RunSelection(title string, options []SelectOption) *SelectOption {
	items := make([]list.Item, len(options))
	for i, opt := range options {
		items[i] = listItem{
			title: opt.Label,
			desc:  opt.Description,
			value: opt.Value,
		}
	}

	l := list.New(items, list.NewDefaultDelegate(), 80, 14)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	model := listModel{list: l}

	p := tea.NewProgram(model)
	finalModel, err := p.StartReturningModel()
	if err != nil {
		return nil
	}

	result := finalModel.(listModel)
	if result.result == nil {
		return nil
	}

	return &SelectOption{
		Label:       result.result.title,
		Value:       result.result.value,
		Description: result.result.desc,
	}
}

// Validators

func DirectoryValidator(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Check if directory exists or can be created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to create the directory
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("cannot create directory: %w", err)
		}
	}

	return nil
}

func SetupTokenValidator(token string) error {
	if token == "" {
		return fmt.Errorf("setup token cannot be empty")
	}
	if len(token) < 10 {
		return fmt.Errorf("setup token appears too short")
	}
	return nil
}

func APIKeyValidator(key string) error {
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if len(key) < 10 {
		return fmt.Errorf("API key appears too short")
	}
	return nil
}

func BatchSizeValidator(input string) error {
	if input == "" {
		return nil // Allow empty for default
	}

	size, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("must be a number")
	}

	if size < 1 || size > 100 {
		return fmt.Errorf("batch size must be between 1 and 100")
	}

	return nil
}