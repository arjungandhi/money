package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/internal/convert"
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/llm"
	"github.com/arjungandhi/money/pkg/table"
)

var (
	grayColor  = color.New(color.FgHiBlack)
	redColor   = color.New(color.FgRed)   // For expenses (negative amounts)
	greenColor = color.New(color.FgGreen) // For income (positive amounts)
)

// colorizeCategory returns a colorized version of the category name
func colorizeCategory(category string) string {
	if category == "Uncategorized" {
		return redColor.Sprint(category)
	}
	if strings.Contains(category, "(internal)") {
		return grayColor.Sprint(category)
	}

	// All other categories are uncolored
	return category
}

// colorizeAmount returns a colorized version of the amount based on sign
// and calculates the proper padding to account for ANSI color codes
func colorizeAmount(amount int, amountStr string, width int) string {
	coloredStr := amountStr
	if amount < 0 {
		coloredStr = redColor.Sprint(amountStr) // Expenses in red
	} else if amount > 0 {
		coloredStr = greenColor.Sprint(amountStr) // Income in green
	}

	// Calculate padding to account for invisible ANSI codes
	// The visible length is the original string length
	visibleLen := len(amountStr)
	totalLen := len(coloredStr)
	invisibleLen := totalLen - visibleLen

	// Adjust width to account for invisible characters
	adjustedWidth := width + invisibleLen

	// Right-align the colored string with adjusted width
	return fmt.Sprintf("%*s", adjustedWidth, coloredStr)
}

var Transactions = &Z.Cmd{
	Name:    "transactions",
	Aliases: []string{"transaction", "tx", "t"},
	Summary: "Manage and categorize transactions",
	Commands: []*Z.Cmd{
		help.Cmd,
		TransactionsList,
		Categorize,
	},
	Call: func(cmd *Z.Cmd, args ...string) error {
		// If no arguments provided, run manual categorization
		if len(args) == 0 {
			return runManualCategorization()
		}
		// Otherwise show help
		return help.Cmd.Call(cmd, args...)
	},
}

var TransactionsList = &Z.Cmd{
	Name:     "list",
	Aliases:  []string{"ls", "l"},
	Summary:  "List transactions with optional filtering",
	Usage:    "list [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--account <account-id>]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Parse command line arguments
		var startDate, endDate, accountID string
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--start":
				if i+1 < len(args) {
					startDate = args[i+1]
					i++
				}
			case "--end":
				if i+1 < len(args) {
					endDate = args[i+1]
					i++
				}
			case "--account":
				if i+1 < len(args) {
					accountID = args[i+1]
					i++
				}
			}
		}

		// Validate date format if provided
		if startDate != "" {
			if _, err := time.Parse("2006-01-02", startDate); err != nil {
				return fmt.Errorf("invalid start date format. Use YYYY-MM-DD: %w", err)
			}
		}
		if endDate != "" {
			if _, err := time.Parse("2006-01-02", endDate); err != nil {
				return fmt.Errorf("invalid end date format. Use YYYY-MM-DD: %w", err)
			}
		}

		// Get transactions from database
		transactions, err := db.GetTransactions(accountID, startDate, endDate)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}

		if len(transactions) == 0 {
			fmt.Println("No transactions found.")
			return nil
		}

		// Get accounts for name lookup
		accounts, err := db.GetAccounts()
		if err != nil {
			return fmt.Errorf("failed to get accounts: %w", err)
		}

		// Create account ID to display name mapping
		accountMap := make(map[string]string)
		for _, account := range accounts {
			accountMap[account.ID] = account.DisplayName()
		}

		// Create and populate transactions table
		config := table.DefaultConfig()
		config.Title = fmt.Sprintf("Found %d transactions", len(transactions))
		config.MaxColumnWidth = 50

		t := table.NewWithConfig(config, "Date", "Account", "Amount", "Description", "Category")

		for _, txn := range transactions {
			// Parse date for display
			postedTime, _ := time.Parse(time.RFC3339, txn.Posted)
			dateStr := postedTime.Format("2006-01-02 15:04")

			// Format amount (convert cents to dollars)
			amountStr := fmt.Sprintf("$%.2f", float64(txn.Amount)/100.0)
			coloredAmount := colorizeAmount(txn.Amount, amountStr, 12)

			// Get category name if categorized
			categoryStr := "Uncategorized"
			if txn.CategoryID != nil {
				category, err := db.GetCategoryByID(*txn.CategoryID)
				if err == nil {
					categoryStr = category.Name
					if category.IsInternal {
						categoryStr += " (internal)"
					}
				}
			}

			// Get account name for display
			accountDisplay := txn.AccountID // fallback to ID if name not found
			if accountName, exists := accountMap[txn.AccountID]; exists {
				accountDisplay = accountName
			}

			// Apply color to category
			coloredCategory := colorizeCategory(categoryStr)

			t.AddRow(dateStr, accountDisplay, coloredAmount, txn.Description, coloredCategory)
		}

		if err := t.Render(); err != nil {
			return fmt.Errorf("failed to render transactions table: %w", err)
		}

		return nil
	},
}

var Categorize = &Z.Cmd{
	Name:    "categorize",
	Aliases: []string{"cat", "c"},
	Summary: "Manage transaction categorization",
	Commands: []*Z.Cmd{
		help.Cmd,
		CategorizeModify,
		CategorizeClear,
		CategorizeAuto,
		CategorizeManual,
	},
}

var CategorizeModify = &Z.Cmd{
	Name:     "modify",
	Summary:  "Set or change the category of a specific transaction",
	Usage:    "modify <transaction-id> <category-name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 2 {
			return fmt.Errorf("usage: money transactions categorize modify <transaction-id> <category-name>")
		}

		transactionID := args[0]
		categoryName := strings.Join(args[1:], " ")

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Determine category type based on transaction amount
		transactions, err := db.GetTransactions("", "", "")
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}

		var transaction *database.Transaction
		for _, t := range transactions {
			if t.ID == transactionID {
				transaction = &t
				break
			}
		}

		if transaction == nil {
			return fmt.Errorf("transaction not found: %s", transactionID)
		}

		// Save or get category (no type needed now)
		categoryID, err := db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to save category: %w", err)
		}

		// Update transaction
		err = db.UpdateTransactionCategory(transactionID, categoryID)
		if err != nil {
			return fmt.Errorf("failed to update transaction category: %w", err)
		}

		fmt.Printf("Transaction %s categorized as '%s'\n", transactionID, categoryName)
		return nil
	},
}

var CategorizeClear = &Z.Cmd{
	Name:     "clear",
	Summary:  "Clear the category of a specific transaction",
	Usage:    "clear <transaction-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: money transactions categorize clear <transaction-id>")
		}

		transactionID := args[0]

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.ClearTransactionCategory(transactionID)
		if err != nil {
			return fmt.Errorf("failed to clear transaction category: %w", err)
		}

		fmt.Printf("Category cleared for transaction %s\n", transactionID)
		return nil
	},
}

var CategorizeAuto = &Z.Cmd{
	Name:     "auto",
	Summary:  "Automatically categorize transactions using LLM",
	Usage:    "auto [--all]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		processAll := false
		for _, arg := range args {
			if arg == "--all" {
				processAll = true
				break
			}
		}

		if processAll {
			return recategorizeAllTransactions()
		} else {
			return autoCategorizeTransactions()
		}
	},
}

var CategorizeManual = &Z.Cmd{
	Name:     "manual",
	Summary:  "Interactive manual categorization using spreadsheet-style interface",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		return runManualCategorization()
	},
}

// autoCategorizeTransactions implements the LLM-based auto-categorization logic
func autoCategorizeTransactions() error {
	db, err := database.New()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Get uncategorized transactions (not marked as transfers)
	transactions, err := db.GetUncategorizedTransactions()
	if err != nil {
		return fmt.Errorf("failed to get uncategorized transactions: %w", err)
	}

	if len(transactions) == 0 {
		fmt.Println("No uncategorized transactions found.")
		return nil
	}

	fmt.Printf("Found %d uncategorized transactions.\n\n", len(transactions))

	// Get all accounts for context (helps LLM identify transfers and account-specific patterns)
	accounts, err := db.GetAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// Get user's existing categories for categorization
	categories, err := db.GetCategories()
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}

	if len(categories) == 0 {
		fmt.Println("No categories found. Please run 'money transactions category seed' first to create default categories, or add categories manually using 'money transactions category add <name>'.")
		return nil
	}

	// Separate regular and internal categories for the LLM prompt
	var regularCategories []string
	var internalCategories []string
	allCategoryNames := make([]string, len(categories))

	for i, cat := range categories {
		allCategoryNames[i] = cat.Name
		if cat.IsInternal {
			internalCategories = append(internalCategories, cat.Name)
		} else {
			regularCategories = append(regularCategories, cat.Name)
		}
	}

	fmt.Printf("Using %d categories total: %d regular + %d internal\n",
		len(categories), len(regularCategories), len(internalCategories))
	fmt.Printf("Regular: %s\n", strings.Join(regularCategories, ", "))
	if len(internalCategories) > 0 {
		fmt.Printf("Internal: %s\n", strings.Join(internalCategories, ", "))
	}

	// Initialize LLM client
	llmClient := llm.NewClient()
	ctx := context.Background()

	// Convert database types to LLM types
	llmTransactions := convert.ToLLMTransactionData(transactions)
	llmAccounts := convert.ToLLMAccountData(accounts)

	// Get examples from previously categorized transactions
	categorizedExamples, err := db.GetCategorizedExamples(10) // Get up to 10 examples
	if err != nil {
		return fmt.Errorf("failed to get categorized examples: %w", err)
	}

	examples, err := convert.ToCategorizedExamples(categorizedExamples, db)
	if err != nil {
		return fmt.Errorf("failed to convert categorized examples: %w", err)
	}

	if len(examples) > 0 {
		fmt.Printf("📚 Using %d examples from previously categorized transactions\n", len(examples))
	}

	// Categorize transactions using user's existing categories
	fmt.Printf("📝 Categorizing %d transactions using your existing categories...\n", len(llmTransactions))
	categoryResult, err := llmClient.CategorizeTransactionsWithExamples(ctx, llmTransactions, categories, llmAccounts, examples)
	if err != nil {
		return fmt.Errorf("failed to categorize transactions: %w", err)
	}

	// Apply category suggestions with user approval
	categoryCount := 0
	for _, suggestion := range categoryResult.Suggestions {
		// Find the transaction to show details
		var transaction *database.Transaction
		for _, tx := range transactions {
			if tx.ID == suggestion.TransactionID {
				transaction = &tx
				break
			}
		}

		if transaction == nil {
			continue
		}

		// Get category ID (this will find the existing category since we're using user's categories)
		categoryID, err := db.SaveCategory(suggestion.Category)
		if err != nil {
			return fmt.Errorf("failed to get category ID: %w", err)
		}

		// Update transaction category
		err = db.UpdateTransactionCategory(suggestion.TransactionID, categoryID)
		if err != nil {
			return fmt.Errorf("failed to update transaction category: %w", err)
		}
		fmt.Printf("💸 %s → %s\n", transaction.Description, suggestion.Category)
		categoryCount++
	}

	fmt.Printf("\n🎉 Auto-categorization complete!\n")
	fmt.Printf("   Transactions categorized: %d\n", categoryCount)

	return nil
}

// recategorizeAllTransactions recategorizes ALL transactions using LLM
func recategorizeAllTransactions() error {
	// TODO: This function needs to be updated to work with internal categories instead of transfer flags
	fmt.Println("⚠️  Recategorize all functionality temporarily disabled during refactor")
	fmt.Println("Please use 'money transactions categorize auto' for new categorization")
	return nil
}
