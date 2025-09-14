package cli

import (
	"fmt"
	"strings"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)

var Transactions = &Z.Cmd{
	Name:    "transactions",
	Summary: "Manage and categorize transactions",
	Commands: []*Z.Cmd{
		help.Cmd,
		TransactionsList,
		Categorize,
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

			// Get category name if categorized
			categoryStr := "Uncategorized"
			if t.CategoryID != nil {
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

			// Truncate account ID for display
			accountDisplay := t.AccountID
			if len(accountDisplay) > 12 {
				accountDisplay = accountDisplay[:12] + "..."
			}

			fmt.Printf("%-20s %-15s %12s %-50s %s\n",
				dateStr, accountDisplay, amountStr, description, categoryStr)
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

		// Determine category type: negative amounts are expenses, positive are income
		categoryType := "expense"
		if transaction.Amount > 0 {
			categoryType = "income"
		}

		// Save or get category
		categoryID, err := db.SaveCategory(categoryName, categoryType)
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
