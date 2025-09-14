package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)


var (
	grayColor  = color.New(color.FgHiBlack)
	redColor   = color.New(color.FgRed)   // For expenses (negative amounts)
	greenColor = color.New(color.FgGreen) // For income (positive amounts)
)

// colorizeCategory returns a colorized version of the category name
func colorizeCategory(category string) string {
	if category == "Transfer" {
		return grayColor.Sprint(category)
	}
	if category == "Uncategorized" {
		return redColor.Sprint(category)
	}

	// All other categories are uncolored
	return category
}

// colorizeAmount returns a colorized version of the amount based on sign
// and calculates the proper padding to account for ANSI color codes
func colorizeAmount(amount int, amountStr string, width int) string {
	coloredStr := amountStr
	if amount < 0 {
		coloredStr = redColor.Sprint(amountStr)   // Expenses in red
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
	Summary: "Manage and categorize transactions",
	Commands: []*Z.Cmd{
		help.Cmd,
		TransactionsList,
		Categorize,
		Category,
	},
}

var TransactionsList = &Z.Cmd{
	Name:    "list",
	Summary: "List transactions with optional filtering",
	Usage:   "list [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--account <account-id>]",
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

		// Display transactions
		fmt.Printf("Found %d transactions:\n\n", len(transactions))
		fmt.Printf("%-20s %-15s %-12s %-50s %s\n", "Date", "Account", "Amount", "Description", "Category")
		fmt.Println(strings.Repeat("-", 120))

		for _, t := range transactions {
			// Parse date for display
			postedTime, _ := time.Parse(time.RFC3339, t.Posted)
			dateStr := postedTime.Format("2006-01-02 15:04")

			// Format amount (convert cents to dollars)
			amountStr := fmt.Sprintf("$%.2f", float64(t.Amount)/100.0)
			coloredAmount := colorizeAmount(t.Amount, amountStr, 12)

			// Get category name if categorized
			categoryStr := "Uncategorized"
			if t.IsTransfer {
				categoryStr = "Transfer"
			} else if t.CategoryID != nil {
				category, err := db.GetCategoryByID(*t.CategoryID)
				if err == nil {
					categoryStr = category.Name
				}
			}

			// Truncate description if too long
			description := t.Description
			if len(description) > 47 {
				description = description[:47] + "..."
			}

			// Get account name for display
			accountDisplay := t.AccountID // fallback to ID if name not found
			if accountName, exists := accountMap[t.AccountID]; exists {
				accountDisplay = accountName
			}
			// Truncate account name if too long
			if len(accountDisplay) > 13 {
				accountDisplay = accountDisplay[:10] + "..."
			}

			// Apply color to category
			coloredCategory := colorizeCategory(categoryStr)

			fmt.Printf("%-20s %-15s %s %-50s %s\n",
				dateStr, accountDisplay, coloredAmount, description, coloredCategory)
		}

		return nil
	},
}

var Categorize = &Z.Cmd{
	Name:     "categorize",
	Summary:  "Manage transaction categorization",
	Commands: []*Z.Cmd{
		help.Cmd,
		CategorizeModify,
		CategorizeClear,
		CategorizeList,
		CategorizeAdd,
		CategorizeRemove,
		CategorizeSeed,
		CategorizeTransfer,
		CategorizeAuto, // LLM auto-categorization placeholder
	},
}

var CategorizeModify = &Z.Cmd{
	Name:    "modify",
	Summary: "Set or change the category of a specific transaction",
	Usage:   "modify <transaction-id> <category-name>",
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
	Name:    "clear",
	Summary: "Clear the category of a specific transaction",
	Usage:   "clear <transaction-id>",
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

var CategorizeList = &Z.Cmd{
	Name:     "list",
	Summary:  "Show all existing categories",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		categories, err := db.GetCategories()
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		if len(categories) == 0 {
			fmt.Println("No categories found. Use 'money transactions categorize add <name>' to create categories or 'money transactions categorize seed' to add common defaults.")
			return nil
		}

		for _, c := range categories {
			fmt.Println(c.Name)
		}

		return nil
	},
}

var CategorizeAdd = &Z.Cmd{
	Name:     "add",
	Summary:  "Add a new category",
	Usage:    "add <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money transactions categorize add <name>")
		}

		categoryName := strings.Join(args, " ")

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		_, err = db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to add category: %w", err)
		}

		fmt.Printf("Category '%s' added successfully\n", categoryName)
		return nil
	},
}

var CategorizeRemove = &Z.Cmd{
	Name:     "remove",
	Summary:  "Remove a category (only if not used by any transactions)",
	Usage:    "remove <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money transactions categorize remove <name>")
		}

		categoryName := strings.Join(args, " ")

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.DeleteCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to remove category: %w", err)
		}

		fmt.Printf("Category '%s' removed successfully\n", categoryName)
		return nil
	},
}

var CategorizeSeed = &Z.Cmd{
	Name:     "seed",
	Summary:  "Populate database with common default categories",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.SeedDefaultCategories()
		if err != nil {
			return fmt.Errorf("failed to seed categories: %w", err)
		}

		fmt.Println("Default categories added successfully")
		return nil
	},
}

var CategorizeTransfer = &Z.Cmd{
	Name:     "transfer",
	Summary:  "Mark transaction as a transfer (excludes from income/expense calculations)",
	Usage:    "transfer <transaction-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: money transactions categorize transfer <transaction-id>")
		}

		transactionID := args[0]

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.MarkTransactionAsTransfer(transactionID)
		if err != nil {
			return fmt.Errorf("failed to mark transaction as transfer: %w", err)
		}

		fmt.Printf("Transaction %s marked as transfer\n", transactionID)
		return nil
	},
}

var CategorizeAuto = &Z.Cmd{
	Name:     "auto",
	Summary:  "Automatically categorize uncategorized transactions using LLM (TODO)",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement LLM-based auto-categorization
		// This is a placeholder as requested by the user

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Get uncategorized transactions count for demo
		uncategorized, err := db.GetUncategorizedTransactions()
		if err != nil {
			return fmt.Errorf("failed to get uncategorized transactions: %w", err)
		}

		fmt.Printf("TODO: Implement LLM auto-categorization\n")
		fmt.Printf("Found %d uncategorized transactions that could be processed.\n", len(uncategorized))
		fmt.Println("This feature will:")
		fmt.Println("1. Load uncategorized transactions from database")
		fmt.Println("2. Use LLM to suggest categories for each transaction")
		fmt.Println("3. Present interactive prompts for category review and adjustment")
		fmt.Println("4. Save category assignments back to database")

		return nil
	},
}

var Category = &Z.Cmd{
	Name:     "category",
	Summary:  "Manage transaction categories",
	Commands: []*Z.Cmd{
		help.Cmd,
		CategoryList,
		CategoryAdd,
		CategoryRemove,
		CategorySeed,
	},
}

var CategoryList = &Z.Cmd{
	Name:     "list",
	Summary:  "Show all existing categories",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		categories, err := db.GetCategories()
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		if len(categories) == 0 {
			fmt.Println("No categories found. Use 'money transactions category add <name>' to create categories or 'money transactions category seed' to add common defaults.")
			return nil
		}

		for _, c := range categories {
			fmt.Println(c.Name)
		}

		return nil
	},
}

var CategoryAdd = &Z.Cmd{
	Name:     "add",
	Summary:  "Add a new category",
	Usage:    "add <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money transactions category add <name>")
		}

		categoryName := strings.Join(args, " ")

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		_, err = db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to add category: %w", err)
		}

		fmt.Printf("Category '%s' added successfully\n", categoryName)
		return nil
	},
}

var CategoryRemove = &Z.Cmd{
	Name:     "remove",
	Summary:  "Remove a category (only if not used by any transactions)",
	Usage:    "remove <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money transactions category remove <name>")
		}

		categoryName := strings.Join(args, " ")

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.DeleteCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to remove category: %w", err)
		}

		fmt.Printf("Category '%s' removed successfully\n", categoryName)
		return nil
	},
}

var CategorySeed = &Z.Cmd{
	Name:     "seed",
	Summary:  "Populate database with common default categories",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		err = db.SeedDefaultCategories()
		if err != nil {
			return fmt.Errorf("failed to seed categories: %w", err)
		}

		fmt.Println("Default categories added successfully")
		return nil
	},
}
