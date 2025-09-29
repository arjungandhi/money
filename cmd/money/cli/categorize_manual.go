package cli

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/evertras/bubble-table/table"

	"github.com/arjungandhi/money/internal/dbutil"
	"github.com/arjungandhi/money/pkg/database"
)

const (
	columnKeyID              = "id"
	columnKeyDate            = "date"
	columnKeyAccount         = "account"
	columnKeyAmount          = "amount"
	columnKeyDescription     = "description"
	columnKeyCategory        = "category"
	columnKeyTransactionData = "transaction_data"
)

type columnWidths struct {
	date        int
	account     int
	amount      int
	description int
	category    int
}

type CategorizationModel struct {
	table         table.Model
	categories    []database.Category
	categoryInput string
	inputMode     bool
	selectedTxID  string
	message       string
	// Remove db field - we'll create connections as needed
	transactions []database.Transaction
	accounts     map[string]string // account ID to display name mapping
	width        int
	height       int
	// Visual selection mode
	visualMode   bool
	visualStart  int
	selectedRows map[int]bool
	currentIndex int
	// Search mode
	searchMode    bool
	searchInput   string
	searchMatches []int // indices of matching transactions
	searchIndex   int   // current position in searchMatches
}

func calculateOptimalColumnWidths(transactions []database.Transaction, accountMap map[string]string, categories []database.Category, db *database.DB) columnWidths {
	widths := columnWidths{
		date:        10, // "2006-01-02" + header
		account:     7,  // "Account" header length
		amount:      8,  // "$-999.99" typical
		description: 11, // "Description" header length
		category:    8,  // "Category" header length
	}

	// Check actual content lengths
	for _, tx := range transactions {
		// Date is always "2006-01-02" format (10 chars)

		// Account name
		accountName := tx.AccountID
		if name, exists := accountMap[tx.AccountID]; exists {
			accountName = name
		}
		if len(accountName) > widths.account {
			widths.account = len(accountName)
		}

		// Amount formatting
		amountStr := fmt.Sprintf("$%.2f", float64(tx.Amount)/100.0)
		if len(amountStr) > widths.amount {
			widths.amount = len(amountStr)
		}

		// Description
		if len(tx.Description) > widths.description {
			widths.description = len(tx.Description)
		}

		// Category
		categoryStr := "Uncategorized"
		if tx.CategoryID != nil && db != nil {
			if category, err := db.GetCategoryByID(*tx.CategoryID); err == nil {
				categoryStr = category.Name
				if category.IsInternal {
					categoryStr += " (internal)"
				}
			}
		}
		if len(categoryStr) > widths.category {
			widths.category = len(categoryStr)
		}
	}

	// Add some padding and set reasonable limits
	widths.account += 2
	widths.description += 2
	widths.category += 2

	// Set max limits to prevent super wide columns
	if widths.account > 25 {
		widths.account = 25
	}
	if widths.description > 60 {
		widths.description = 60
	}
	if widths.category > 20 {
		widths.category = 20
	}

	return widths
}

func NewCategorizationModel() (*CategorizationModel, error) {
	var model *CategorizationModel
	err := dbutil.WithDatabase(func(db *database.DB) error {
		// Get all transactions
		transactions, err := db.GetTransactions("", "", "")
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}

		// Get all categories
		categories, err := db.GetCategories()
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		// Get accounts for name lookup
		accounts, err := db.GetAccounts()
		if err != nil {
			return fmt.Errorf("failed to get accounts: %w", err)
		}

		// Create account mapping
		accountMap := make(map[string]string)
		for _, account := range accounts {
			accountMap[account.ID] = account.DisplayName()
		}

		// Calculate optimal column widths based on content
		colWidths := calculateOptimalColumnWidths(transactions, accountMap, categories, db)

		// Create table rows
		rows := []table.Row{}
		for _, tx := range transactions {
			row := transactionToRowWithDB(tx, accountMap, db)
			rows = append(rows, row)
		}

		// Create table with columns and highlighting using calculated widths
		tableModel := table.New([]table.Column{
			table.NewColumn(columnKeyDate, "Date", colWidths.date),
			table.NewColumn(columnKeyAccount, "Account", colWidths.account),
			table.NewColumn(columnKeyAmount, "Amount", colWidths.amount),
			table.NewColumn(columnKeyDescription, "Description", colWidths.description),
			table.NewColumn(columnKeyCategory, "Category", colWidths.category),
		}).WithRows(rows).
			BorderRounded().
			WithPageSize(25).
			Focused(true).
			WithBaseStyle(lipgloss.NewStyle().
				BorderForeground(lipgloss.Color("#00d7ff")).
				Align(lipgloss.Left)).
			WithRowStyleFunc(func(input table.RowStyleFuncInput) lipgloss.Style {
				// Current row highlighting (basic for now)
				if input.IsHighlighted {
					return lipgloss.NewStyle().
						Background(lipgloss.Color("#555"))
				}
				return lipgloss.NewStyle()
			})

		model = &CategorizationModel{
			table:      tableModel,
			categories: categories,
			// db field removed
			transactions: transactions,
			accounts:     accountMap,
			message:      fmt.Sprintf("Found %d transactions. Use j/k to navigate, e to categorize, q to quit.", len(transactions)),
			selectedRows: make(map[int]bool),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return model, nil
}

func transactionToRow(tx database.Transaction, accountMap map[string]string) table.Row {
	return transactionToRowWithDB(tx, accountMap, nil)
}

func transactionToRowWithDB(tx database.Transaction, accountMap map[string]string, db *database.DB) table.Row {
	// Parse date for display
	postedTime, _ := time.Parse(time.RFC3339, tx.Posted)
	dateStr := postedTime.Format("2006-01-02")

	// Format amount
	amountStr := fmt.Sprintf("$%.2f", float64(tx.Amount)/100.0)
	var styledAmount table.StyledCell
	if tx.Amount < 0 {
		styledAmount = table.NewStyledCell(amountStr, lipgloss.NewStyle().Foreground(lipgloss.Color("#f64")))
	} else {
		styledAmount = table.NewStyledCell(amountStr, lipgloss.NewStyle().Foreground(lipgloss.Color("#8c8")))
	}

	// Get account name
	accountDisplay := tx.AccountID
	if accountName, exists := accountMap[tx.AccountID]; exists {
		accountDisplay = accountName
	}
	// Don't truncate - let the table handle column width
	description := tx.Description

	// Category display
	categoryStr := "Uncategorized"
	categoryColor := "#f64" // red for uncategorized

	if tx.CategoryID != nil && db != nil {
		if category, err := db.GetCategoryByID(*tx.CategoryID); err == nil {
			categoryStr = category.Name
			if category.IsInternal {
				categoryStr += " (internal)"
				categoryColor = "#888" // gray for internal categories
			} else {
				categoryColor = "#8c8" // green for categorized
			}
		}
	}

	styledCategory := table.NewStyledCell(categoryStr, lipgloss.NewStyle().Foreground(lipgloss.Color(categoryColor)))

	return table.NewRow(table.RowData{
		columnKeyDate:            dateStr,
		columnKeyAccount:         accountDisplay,
		columnKeyAmount:          styledAmount,
		columnKeyDescription:     description,
		columnKeyCategory:        styledCategory,
		columnKeyTransactionData: tx,
	})
}

func (m CategorizationModel) Init() tea.Cmd {
	return nil
}

func (m CategorizationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle input modes first - highest priority
	if m.inputMode {
		return m.updateInputMode(msg)
	}
	if m.searchMode {
		return m.updateSearchMode(msg)
	}

	// Handle window resize
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		return m.handleWindowResize(windowMsg)
	}

	// Handle key messages
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		handled, newModel, cmd := m.handleKeyMessage(keyMsg)
		if handled {
			return newModel, cmd
		}
	}

	// Pass unhandled messages to table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// handleWindowResize handles terminal resize events
func (m CategorizationModel) handleWindowResize(windowMsg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	if m.width == 0 || m.height == 0 {
		// First time getting terminal size, recalculate table dimensions
		m.width = windowMsg.Width
		m.height = windowMsg.Height

		// Calculate page size based on terminal height
		overhead := 11
		pageSize := windowMsg.Height - overhead
		if pageSize < 5 {
			pageSize = 5
		}
		if pageSize > 50 {
			pageSize = 50
		}

		// Calculate dynamic column widths
		remainingWidth := windowMsg.Width - 24 - 10
		accountWidth := remainingWidth * 20 / 100
		categoryWidth := remainingWidth * 25 / 100
		descriptionWidth := remainingWidth * 55 / 100

		// Set minimum widths
		if accountWidth < 15 {
			accountWidth = 15
		}
		if categoryWidth < 18 {
			categoryWidth = 18
		}
		if descriptionWidth < 30 {
			descriptionWidth = 30
		}

		// Update table with calculated dimensions
		m.table = table.New([]table.Column{
			table.NewColumn(columnKeyDate, "Date", 12),
			table.NewColumn(columnKeyAccount, "Account", accountWidth),
			table.NewColumn(columnKeyAmount, "Amount", 12),
			table.NewColumn(columnKeyDescription, "Description", descriptionWidth),
			table.NewColumn(columnKeyCategory, "Category", categoryWidth),
		}).WithRows(m.getRebuildRows()).
			BorderRounded().
			WithPageSize(pageSize).
			Focused(true).
			WithBaseStyle(lipgloss.NewStyle().
				BorderForeground(lipgloss.Color("#00d7ff")).
				Align(lipgloss.Left)).
			WithRowStyleFunc(func(input table.RowStyleFuncInput) lipgloss.Style {
				if input.IsHighlighted {
					return lipgloss.NewStyle().Background(lipgloss.Color("#555"))
				}
				return lipgloss.NewStyle()
			})
	}
	return m, nil
}

// handleKeyMessage handles all key events and returns (handled, model, cmd)
func (m CategorizationModel) handleKeyMessage(keyMsg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	key := keyMsg.String()

	// Global keys (work in all modes)
	if key == "ctrl+c" || key == "q" {
		return true, m, tea.Quit
	}

	// Handle navigation keys (j/k) - in visual mode we handle completely, otherwise let table handle
	if key == "j" || key == "down" {
		if m.visualMode {
			// In visual mode, we handle navigation completely
			if m.currentIndex < len(m.transactions)-1 {
				m.currentIndex++
				m.updateVisualSelection()
			}
			return true, m, nil // Don't pass to table in visual mode
		}
		return false, m, nil // Let table handle navigation in normal mode
	}
	if key == "k" || key == "up" {
		if m.visualMode {
			// In visual mode, we handle navigation completely
			if m.currentIndex > 0 {
				m.currentIndex--
				m.updateVisualSelection()
			}
			return true, m, nil // Don't pass to table in visual mode
		}
		return false, m, nil // Let table handle navigation in normal mode
	}

	// Mode-specific keys
	if m.visualMode {
		return m.handleVisualModeKeys(key)
	} else {
		return m.handleNormalModeKeys(key)
	}
}

// handleNormalModeKeys handles keys in normal mode
func (m CategorizationModel) handleNormalModeKeys(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "v":
		// Enter visual mode
		m.visualMode = true
		// Get the actual highlighted row index from the table
		m.visualStart = m.table.GetHighlightedRowIndex()
		m.currentIndex = m.visualStart // Keep our tracking in sync
		m.selectedRows = make(map[int]bool)
		m.selectedRows[m.visualStart] = true
		m.message = "Visual mode - use j/k to select range, e to categorize, u to uncategorize"
		m.updateTableStyling()
		return true, m, nil

	case "e":
		// Enter edit mode (categorization)
		selected := m.getSelectedTransactions()
		if len(selected) > 0 {
			m.selectedTxID = selected[0].ID // For single transaction tracking in input mode
			m.inputMode = true
			m.categoryInput = ""
			if len(selected) == 1 {
				m.message = fmt.Sprintf("Enter category for: %s (or press Esc to cancel)", selected[0].Description)
			} else {
				m.message = fmt.Sprintf("Enter category for %d transactions (or press Esc to cancel)", len(selected))
			}
		}
		return true, m, nil

	case "u":
		// Uncategorize transactions
		selected := m.getSelectedTransactions()
		if len(selected) > 0 {
			err := m.uncategorizeTransactions(selected)
			if err != nil {
				m.message = fmt.Sprintf("Error uncategorizing: %v", err)
			} else {
				if len(selected) == 1 {
					m.message = fmt.Sprintf("Uncategorized '%s'", selected[0].Description)
				} else {
					m.message = fmt.Sprintf("Uncategorized %d transactions", len(selected))
				}
				m.refreshTransactionView()
			}
		}
		return true, m, nil

	case "/":
		// Enter search mode
		m.searchMode = true
		m.searchInput = ""
		m.message = "Search: (press Enter to search, Esc to cancel)"
		return true, m, nil

	case "n":
		// Next search result
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex + 1) % len(m.searchMatches)
			m.navigateToSearchResult()
		} else {
			m.message = "No search results"
		}
		return true, m, nil

	case "N":
		// Previous search result
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex - 1 + len(m.searchMatches)) % len(m.searchMatches)
			m.navigateToSearchResult()
		} else {
			m.message = "No search results"
		}
		return true, m, nil
	}

	return false, m, nil
}

// handleVisualModeKeys handles keys in visual mode
func (m CategorizationModel) handleVisualModeKeys(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "escape", "esc", tea.KeyEscape.String(), "v":
		// Exit visual mode
		m.visualMode = false
		m.selectedRows = make(map[int]bool)
		m.message = "Visual mode cancelled"
		m.updateTableStyling()
		return true, m, nil

	case "e":
		// Categorization (works for both single and multiple)
		selected := m.getSelectedTransactions()
		if len(selected) > 0 {
			m.inputMode = true
			m.categoryInput = ""
			m.message = fmt.Sprintf("Enter category for %d selected transactions (or press Esc to cancel)", len(selected))
		}
		return true, m, nil

	case "u":
		// Uncategorize (works for both single and multiple)
		selected := m.getSelectedTransactions()
		if len(selected) > 0 {
			err := m.uncategorizeTransactions(selected)
			if err != nil {
				m.message = fmt.Sprintf("Error uncategorizing: %v", err)
			} else {
				m.message = fmt.Sprintf("Uncategorized %d transactions", len(selected))
				m.visualMode = false
				m.selectedRows = make(map[int]bool)
				m.refreshTransactionView()
			}
		}
		return true, m, nil

	case "/":
		// Enter search mode (exit visual mode first)
		m.visualMode = false
		m.selectedRows = make(map[int]bool)
		m.searchMode = true
		m.searchInput = ""
		m.message = "Search: (press Enter to search, Esc to cancel)"
		m.updateTableStyling()
		return true, m, nil

	case "n":
		// Next search result
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex + 1) % len(m.searchMatches)
			m.navigateToSearchResult()
		} else {
			m.message = "No search results"
		}
		return true, m, nil

	case "N":
		// Previous search result
		if len(m.searchMatches) > 0 {
			m.searchIndex = (m.searchIndex - 1 + len(m.searchMatches)) % len(m.searchMatches)
			m.navigateToSearchResult()
		} else {
			m.message = "No search results"
		}
		return true, m, nil
	}

	return false, m, nil
}

// getSelectedTransactions returns the currently selected transactions
// In normal mode: returns the highlighted transaction
// In visual mode: returns all selected transactions
func (m *CategorizationModel) getSelectedTransactions() []database.Transaction {
	if m.visualMode && len(m.selectedRows) > 0 {
		// Visual mode: return all selected transactions
		var selected []database.Transaction
		for index := range m.selectedRows {
			if index < len(m.transactions) {
				selected = append(selected, m.transactions[index])
			}
		}
		return selected
	} else {
		// Normal mode: return highlighted transaction
		if len(m.transactions) > 0 {
			highlightedRow := m.table.HighlightedRow()
			if highlightedRow.Data != nil {
				tx := highlightedRow.Data[columnKeyTransactionData].(database.Transaction)
				return []database.Transaction{tx}
			}
		}
		return []database.Transaction{}
	}
}

// categorizeTransactions applies a category to a list of transactions
func (m *CategorizationModel) categorizeTransactions(transactions []database.Transaction, categoryName string) error {
	for _, tx := range transactions {
		err := m.categorizeTransaction(tx.ID, categoryName)
		if err != nil {
			return err
		}
	}
	return nil
}

// uncategorizeTransactions removes categories from a list of transactions
func (m *CategorizationModel) uncategorizeTransactions(transactions []database.Transaction) error {
	return dbutil.WithDatabase(func(db *database.DB) error {
		for _, tx := range transactions {
			err := db.ClearTransactionCategory(tx.ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// updateSearchMode handles search input
func (m *CategorizationModel) updateSearchMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "escape", "esc", tea.KeyEscape.String():
			m.searchMode = false
			m.searchInput = ""
			m.message = "Search cancelled"
		case "enter":
			// Perform search
			if m.searchInput != "" {
				m.performSearch(m.searchInput)
				if len(m.searchMatches) > 0 {
					m.searchIndex = 0
					m.navigateToSearchResult()
					m.message = fmt.Sprintf("Found %d matches (n/N to navigate)", len(m.searchMatches))
				} else {
					m.message = fmt.Sprintf("No matches found for '%s'", m.searchInput)
				}
			}
			m.searchMode = false
		case "backspace":
			if len(m.searchInput) > 0 {
				m.searchInput = m.searchInput[:len(m.searchInput)-1]
				m.message = fmt.Sprintf("Search: %s", m.searchInput)
			}
		default:
			// Add typed character to search input
			if len(msg.String()) == 1 {
				m.searchInput += msg.String()
				m.message = fmt.Sprintf("Search: %s", m.searchInput)
			}
		}
	}
	return m, nil
}

// performSearch finds all transactions matching the search term
func (m *CategorizationModel) performSearch(searchTerm string) {
	m.searchMatches = nil
	searchTerm = strings.ToLower(searchTerm)

	for i, tx := range m.transactions {
		// Search in description, account name, and category
		if m.transactionMatches(tx, searchTerm) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
}

// transactionMatches checks if a transaction matches the search term
func (m *CategorizationModel) transactionMatches(tx database.Transaction, searchTerm string) bool {
	// Search in description
	if strings.Contains(strings.ToLower(tx.Description), searchTerm) {
		return true
	}

	// Search in account name
	accountName := tx.AccountID
	if name, exists := m.accounts[tx.AccountID]; exists {
		accountName = name
	}
	if strings.Contains(strings.ToLower(accountName), searchTerm) {
		return true
	}

	// Search in category
	if tx.CategoryID != nil {
		for _, cat := range m.categories {
			if cat.ID == *tx.CategoryID {
				if strings.Contains(strings.ToLower(cat.Name), searchTerm) {
					return true
				}
				break
			}
		}
	}

	// Search in amount (as string)
	amountStr := fmt.Sprintf("%.2f", float64(tx.Amount)/100.0)
	if strings.Contains(amountStr, searchTerm) {
		return true
	}

	return false
}

// navigateToSearchResult moves to the current search result
func (m *CategorizationModel) navigateToSearchResult() {
	if len(m.searchMatches) == 0 {
		return
	}

	targetIndex := m.searchMatches[m.searchIndex]
	m.currentIndex = targetIndex
	m.table = m.table.WithHighlightedRow(targetIndex)

	// Update message with current position
	m.message = fmt.Sprintf("Match %d of %d", m.searchIndex+1, len(m.searchMatches))
}

func (m *CategorizationModel) updateInputMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "escape", "esc", tea.KeyEscape.String():
			m.inputMode = false
			m.categoryInput = ""
			m.message = "Categorization cancelled"
		case "enter":
			// Apply categorization with best match if input doesn't exactly match
			if m.categoryInput != "" {
				// Find best matching category
				bestMatch := m.findBestCategoryMatch(m.categoryInput)
				if bestMatch != "" {
					// Get currently selected transactions (works for both modes)
					selected := m.getSelectedTransactions()
					if len(selected) > 0 {
						err := m.categorizeTransactions(selected, bestMatch)
						if err != nil {
							m.message = fmt.Sprintf("Error categorizing: %v", err)
						} else {
							if len(selected) == 1 {
								m.message = fmt.Sprintf("Categorized '%s' as '%s'", selected[0].Description, bestMatch)
							} else {
								m.message = fmt.Sprintf("Categorized %d transactions as '%s'", len(selected), bestMatch)
							}
							// Exit visual mode after operation
							if m.visualMode {
								m.visualMode = false
								m.selectedRows = make(map[int]bool)
							}
							m.refreshTransactionView()
						}
					}
				} else {
					m.message = fmt.Sprintf("No matching category found for '%s'", m.categoryInput)
				}
			}
			m.inputMode = false
			m.categoryInput = ""
		case "backspace":
			if len(m.categoryInput) > 0 {
				m.categoryInput = m.categoryInput[:len(m.categoryInput)-1]
			}
		default:
			// Add character to input
			if len(msg.String()) == 1 {
				m.categoryInput += msg.String()
			}
		}
	}

	return m, nil
}

func (m *CategorizationModel) categorizeTransaction(txID, categoryName string) error {
	return dbutil.WithDatabase(func(db *database.DB) error {
		// Save or get category
		categoryID, err := db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to save category: %w", err)
		}

		// Update transaction
		err = db.UpdateTransactionCategory(txID, categoryID)
		if err != nil {
			return fmt.Errorf("failed to update transaction category: %w", err)
		}

		return nil
	})
}

func (m *CategorizationModel) getRebuildRows() []table.Row {
	rows := []table.Row{}
	err := dbutil.WithDatabase(func(db *database.DB) error {
		for _, tx := range m.transactions {
			row := transactionToRowWithDB(tx, m.accounts, db)
			rows = append(rows, row)
		}
		return nil
	})
	if err != nil {
		// Fallback to nil db if there's an error
		for _, tx := range m.transactions {
			row := transactionToRowWithDB(tx, m.accounts, nil)
			rows = append(rows, row)
		}
	}
	return rows
}

func (m *CategorizationModel) findBestCategoryMatch(input string) string {
	if input == "" {
		return ""
	}

	inputLower := strings.ToLower(input)

	// First check for exact match (case insensitive)
	for _, cat := range m.categories {
		if strings.ToLower(cat.Name) == inputLower {
			return cat.Name
		}
	}

	// Then check for categories that start with the input
	for _, cat := range m.categories {
		if strings.HasPrefix(strings.ToLower(cat.Name), inputLower) {
			return cat.Name
		}
	}

	// Finally check for categories that contain the input
	for _, cat := range m.categories {
		if strings.Contains(strings.ToLower(cat.Name), inputLower) {
			return cat.Name
		}
	}

	// If no match found, return empty string (don't create new category)
	return ""
}

func (m *CategorizationModel) getCurrentRowIndex() int {
	return m.currentIndex
}

func (m *CategorizationModel) updateVisualSelection() {
	if !m.visualMode {
		return
	}

	// Use our current index and update table to match
	currentIndex := m.currentIndex

	// Move table highlight to match our position
	m.table = m.table.WithHighlightedRow(currentIndex)

	// Clear previous selection
	m.selectedRows = make(map[int]bool)

	// Select range from visualStart to current
	start := m.visualStart
	end := currentIndex
	if start > end {
		start, end = end, start
	}

	for i := start; i <= end; i++ {
		if i >= 0 && i < len(m.transactions) {
			m.selectedRows[i] = true
		}
	}

	// Update table styling to reflect new selection
	m.updateTableStyling()

	// Update message to show current selection count
	selectionCount := len(m.selectedRows)
	if selectionCount == 1 {
		m.message = fmt.Sprintf("Visual mode: %d row selected", selectionCount)
	} else {
		m.message = fmt.Sprintf("Visual mode: %d rows selected", selectionCount)
	}
}

func (m *CategorizationModel) refreshTransactionView() {
	err := dbutil.WithDatabase(func(db *database.DB) error {
		// Refresh transactions from database to get updated categories/transfer status
		transactions, err := db.GetTransactions("", "", "")
		if err != nil {
			return err
		}

		m.transactions = transactions
		return nil
	})

	if err != nil {
		m.message = fmt.Sprintf("Error refreshing transactions: %v", err)
		return
	}

	// Update rows while preserving the table styling
	m.table = m.table.WithRows(m.getRebuildRows())

	// Update visual styling if needed
	m.updateTableStyling()
}

func (m *CategorizationModel) updateTableStyling() {
	// Create a closure that captures the current model state for styling
	m.table = m.table.WithRowStyleFunc(func(input table.RowStyleFuncInput) lipgloss.Style {
		// Check if this row is selected in visual mode
		isSelected := m.selectedRows[input.Index]

		// Visual selection styling (takes priority)
		if isSelected {
			return lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#555")) // Use same color as normal highlighting
		}

		// Current row highlighting
		if input.IsHighlighted {
			if m.visualMode {
				// In visual mode, show lighter highlight for current row
				return lipgloss.NewStyle().
					Background(lipgloss.Color("#666"))
			} else {
				// Normal highlighting
				return lipgloss.NewStyle().
					Background(lipgloss.Color("#555"))
			}
		}
		return lipgloss.NewStyle()
	})
}

func (m CategorizationModel) View() string {
	style := lipgloss.NewStyle().Margin(1)

	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00d7ff")).
		Bold(true).
		Render("Manual Transaction Categorization")

	var instructions string
	if m.visualMode {
		selectedCount := len(m.selectedRows)
		instructions = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Render(fmt.Sprintf("VISUAL MODE (%d selected)  |  j/k: extend selection  |  e: bulk categorize  |  u: bulk uncategorize  |  v/Esc: exit", selectedCount))
	} else {
		instructions = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Render("Navigation: j/k or ↑↓  |  e: categorize  |  u: uncategorize  |  v: visual mode  |  /: search  |  q: quit")
	}

	var content string
	if len(m.transactions) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8c8")).
			Render("✅ No transactions found!")
	} else {
		content = m.table.View()
	}

	var input string
	if m.inputMode {
		categories := make([]string, len(m.categories))
		for i, cat := range m.categories {
			categories[i] = cat.Name
		}

		suggestions := ""
		if len(categories) > 0 {
			// Show category suggestions
			matchingCats := []string{}
			for _, cat := range categories {
				if m.categoryInput == "" || strings.Contains(strings.ToLower(cat), strings.ToLower(m.categoryInput)) {
					matchingCats = append(matchingCats, cat)
				}
			}
			if len(matchingCats) > 0 {
				suggestions = "\nSuggestions: " + strings.Join(matchingCats[:min(5, len(matchingCats))], ", ")
			}
		}

		inputStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00d7ff")).
			Background(lipgloss.Color("#333"))

		input = "\n" + inputStyle.Render(fmt.Sprintf("Category: %s_", m.categoryInput)) + suggestions
	}

	status := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0")).
		Render(m.message)

	return style.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			instructions,
			"",
			content,
			input,
			"",
			status,
		),
	)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runManualCategorization() error {
	model, err := NewCategorizationModel()
	if err != nil {
		return err
	}

	if len(model.transactions) == 0 {
		fmt.Println("No transactions found.")
		return nil
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		return err
	}
	return nil
}
