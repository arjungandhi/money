package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SelectOption represents an option in a selection menu
type SelectOption struct {
	Label       string
	Value       string
	Description string
}

// Validator function type for input validation
type ValidatorFunc func(string) error

// RunConfirmation shows a yes/no prompt and returns true if user confirms
func RunConfirmation(prompt string) bool {
	fmt.Printf("%s (y/N): ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return response == "y" || response == "yes"
	}
	return false
}

// RunInput shows a simple input prompt
func RunInput(prompt string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			return defaultValue
		}
		return input
	}
	return defaultValue
}

// RunInputWithValidator shows an input prompt with validation
func RunInputWithValidator(prompt string, defaultValue string, validator ValidatorFunc) string {
	for {
		input := RunInput(prompt, defaultValue)
		if validator != nil {
			if err := validator(input); err != nil {
				fmt.Printf("⚠️  %v\nPlease try again.\n", err)
				continue
			}
		}
		return input
	}
}

// RunSelection shows a selection menu and returns the selected option
func RunSelection(prompt string, options []SelectOption) *SelectOption {
	if len(options) == 0 {
		return nil
	}

	fmt.Println(prompt)
	for i, option := range options {
		fmt.Printf("  %d) %s", i+1, option.Label)
		if option.Description != "" {
			fmt.Printf(" - %s", option.Description)
		}
		fmt.Println()
	}

	for {
		fmt.Printf("Choose an option (1-%d): ", len(options))
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			input := strings.TrimSpace(scanner.Text())
			if choice, err := strconv.Atoi(input); err == nil && choice >= 1 && choice <= len(options) {
				return &options[choice-1]
			}
		}
		fmt.Println("Invalid choice. Please try again.")
	}
}

// Validator functions
func SetupTokenValidator(token string) error {
	if token == "" {
		return fmt.Errorf("setup token cannot be empty")
	}
	if len(token) < 10 {
		return fmt.Errorf("setup token appears to be too short")
	}
	return nil
}

func APIKeyValidator(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if len(apiKey) < 10 {
		return fmt.Errorf("API key appears to be too short")
	}
	return nil
}

func DirectoryValidator(dir string) error {
	if dir == "" {
		return fmt.Errorf("directory path cannot be empty")
	}
	
	// Expand ~ to home directory
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("could not get home directory: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}
	
	// Check if parent directory exists
	parentDir := filepath.Dir(dir)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("parent directory does not exist: %s", parentDir)
	}
	
	return nil
}

func BatchSizeValidator(sizeStr string) error {
	if sizeStr == "" {
		return fmt.Errorf("batch size cannot be empty")
	}
	
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return fmt.Errorf("batch size must be a number")
	}
	
	if size < 1 {
		return fmt.Errorf("batch size must be at least 1")
	}
	
	if size > 100 {
		return fmt.Errorf("batch size cannot be more than 100")
	}
	
	return nil
}

// Interactive manual categorization using bubble tea
func runManualCategorization() error {
	fmt.Println("Manual categorization interface not yet implemented.")
	fmt.Println("Please use 'money transactions categorize auto' for now.")
	return nil
}

// Bubble tea models for more advanced UI (placeholder for future implementation)
type model struct {
	list     list.Model
	input    textinput.Model
	quitting bool
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	
	var b strings.Builder
	
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(1).
		PaddingLeft(4).
		Width(50)
	
	b.WriteString(style.Render("Money CLI"))
	b.WriteString("\n\n")
	b.WriteString("Interactive categorization coming soon!\n")
	b.WriteString("Press q to quit.\n")
	
	return b.String()
}
