package cli

import (
	"fmt"
	"strings"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/table"
)

var Categories = &Z.Cmd{
	Name:     "categories",
	Aliases:  []string{"category", "cat"},
	Summary:  "Manage transaction categories",
	Commands: []*Z.Cmd{
		help.Cmd,
		CategoriesList,
		CategoriesAdd,
		CategoriesDelete,
		CategoriesSeed,
	},
}

var CategoriesList = &Z.Cmd{
	Name:     "list",
	Aliases:  []string{"ls", "l"},
	Summary:  "List all categories",
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
			fmt.Println("No categories found. Use 'money categories seed' to create default categories or 'money categories add <name>' to add a category.")
			return nil
		}

		config := table.DefaultConfig()
		config.Title = "Transaction Categories"
		config.MaxColumnWidth = 50

		t := table.NewWithConfig(config, "ID", "Name", "Type", "Usage Count")

		for _, category := range categories {
			typeStr := "Regular"
			if category.IsInternal {
				typeStr = "Internal"
			}

			// Get usage count
			count, err := db.GetCategoryUsageCount(category.ID)
			if err != nil {
				count = 0 // fallback if error
			}

			t.AddRow(fmt.Sprintf("%d", category.ID), category.Name, typeStr, fmt.Sprintf("%d", count))
		}

		if err := t.Render(); err != nil {
			return fmt.Errorf("failed to render categories table: %w", err)
		}

		return nil
	},
}

var CategoriesAdd = &Z.Cmd{
	Name:     "add",
	Summary:  "Add a new category",
	Usage:    "<name> [--internal]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) == 0 {
			return fmt.Errorf("usage: money categories add <name> [--internal]")
		}

		name := strings.Join(args, " ")
		isInternal := false

		// Check for --internal flag
		if strings.HasSuffix(name, " --internal") {
			isInternal = true
			name = strings.TrimSuffix(name, " --internal")
		}

		if name == "" {
			return fmt.Errorf("category name cannot be empty")
		}

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		categoryID, err := db.SaveCategory(name)
		if err != nil {
			return fmt.Errorf("failed to save category: %w", err)
		}

		// If internal flag was specified, update the category
		if isInternal {
			err = db.SetCategoryInternal(categoryID, true)
			if err != nil {
				return fmt.Errorf("failed to set category as internal: %w", err)
			}
		}

		typeStr := "regular"
		if isInternal {
			typeStr = "internal"
		}

		fmt.Printf("Successfully added %s category: %s (ID: %d)\n", typeStr, name, categoryID)
		return nil
	},
}

var CategoriesDelete = &Z.Cmd{
	Name:     "delete",
	Aliases:  []string{"del", "rm"},
	Summary:  "Delete a category and uncategorize associated transactions",
	Usage:    "<category-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: money categories delete <category-id>")
		}

		categoryIDStr := args[0]

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Parse category ID
		var categoryID int
		if _, err := fmt.Sscanf(categoryIDStr, "%d", &categoryID); err != nil {
			return fmt.Errorf("invalid category ID: %s", categoryIDStr)
		}

		// Check if category exists and get its name
		category, err := db.GetCategoryByID(categoryID)
		if err != nil {
			return fmt.Errorf("category not found: %w", err)
		}

		// Get usage count to warn user
		count, err := db.GetCategoryUsageCount(categoryID)
		if err != nil {
			return fmt.Errorf("failed to check category usage: %w", err)
		}

		if count > 0 {
			fmt.Printf("⚠️  Category '%s' is used by %d transaction(s).\n", category.Name, count)
			fmt.Printf("Deleting this category will uncategorize these transactions.\n\n")
			fmt.Printf("Type 'yes' to confirm deletion: ")

			var confirmation string
			fmt.Scanln(&confirmation)

			if strings.ToLower(confirmation) != "yes" {
				fmt.Println("Category deletion cancelled.")
				return nil
			}
		}

		err = db.DeleteCategoryByID(categoryID)
		if err != nil {
			return fmt.Errorf("failed to delete category: %w", err)
		}

		fmt.Printf("Successfully deleted category: %s\n", category.Name)
		if count > 0 {
			fmt.Printf("Uncategorized %d transaction(s).\n", count)
		}

		return nil
	},
}

var CategoriesSeed = &Z.Cmd{
	Name:     "seed",
	Summary:  "Create default categories for transaction categorization",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Check if categories already exist
		categories, err := db.GetCategories()
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		if len(categories) > 0 {
			fmt.Printf("Found %d existing categories. Do you want to add default categories anyway? (y/N): ", len(categories))
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
				fmt.Println("Seed cancelled.")
				return nil
			}
		}

		// Default categories for personal finance
		defaultCategories := []struct {
			name       string
			isInternal bool
		}{
			// Regular spending categories
			{"Food & Dining", false},
			{"Groceries", false},
			{"Entertainment", false},
			{"Transportation", false},
			{"Gas & Fuel", false},
			{"Shopping", false},
			{"Health & Medical", false},
			{"Home & Garden", false},
			{"Utilities", false},
			{"Insurance", false},
			{"Phone & Internet", false},
			{"Travel", false},
			{"Education", false},
			{"Personal Care", false},
			{"Gifts & Donations", false},
			{"Professional Services", false},
			{"Taxes", false},
			{"Fees & Charges", false},
			{"Subscriptions", false},
			
			// Income categories
			{"Salary", false},
			{"Freelance Income", false},
			{"Investment Income", false},
			{"Refunds", false},
			{"Other Income", false},
			
			// Internal categories for account management
			{"Transfer", true},
			{"Credit Card Payment", true},
			{"Account Adjustment", true},
		}

		var added int
		for _, cat := range defaultCategories {
			categoryID, err := db.SaveCategory(cat.name)
			if err != nil {
				fmt.Printf("⚠️  Failed to create category '%s': %v\n", cat.name, err)
				continue
			}

			if cat.isInternal {
				err = db.SetCategoryInternal(categoryID, true)
				if err != nil {
					fmt.Printf("⚠️  Failed to set category '%s' as internal: %v\n", cat.name, err)
					continue
				}
			}

			added++
		}

		fmt.Printf("✅ Successfully created %d default categories.\n", added)
		fmt.Println("Use 'money categories list' to see all categories.")
		fmt.Println("Use 'money transactions categorize auto' to start auto-categorizing transactions.")

		return nil
	},
}
