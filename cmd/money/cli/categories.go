package cli

import (
	"fmt"
	"strings"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/internal/dbutil"
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/table"
)

var Categories = &Z.Cmd{
	Name:    "categories",
	Aliases: []string{"category", "cat"},
	Summary: "Manage transaction categories",
	Commands: []*Z.Cmd{
		help.Cmd,
		CategoriesList,
		CategoriesAdd,
		CategoriesRemove,
		CategoriesSetInternal,
		CategoriesClearInternal,
		CategoriesSeed,
	},
}

var CategoriesList = &Z.Cmd{
	Name:     "list",
	Summary:  "Show all existing categories with their internal status",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		return dbutil.WithDatabase(func(db *database.DB) error {
			categories, err := db.GetCategories()
			if err != nil {
				return fmt.Errorf("failed to get categories: %w", err)
			}

			if len(categories) == 0 {
				fmt.Println("No categories found. Use 'money categories add <name>' to create categories or 'money categories seed' to add common defaults.")
				return nil
			}

			t := table.New("Category", "Internal")
			for _, c := range categories {
				internal := "No"
				if c.IsInternal {
					internal = "Yes"
				}
				t.AddRow(c.Name, internal)
			}

			if err := t.Render(); err != nil {
				return fmt.Errorf("failed to render categories table: %w", err)
			}

			return nil
		})
	},
}

var CategoriesAdd = &Z.Cmd{
	Name:     "add",
	Summary:  "Add a new category, optionally marking it as internal",
	Usage:    "<name> [--internal]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money categories add <name> [--internal]")
		}

		// Parse flags and non-flag arguments
		var nonFlags []string
		isInternal := false
		for _, arg := range args {
			if arg == "--internal" {
				isInternal = true
			} else {
				nonFlags = append(nonFlags, arg)
			}
		}

		if len(nonFlags) < 1 {
			return fmt.Errorf("usage: money categories add <name> [--internal]")
		}

		categoryName := strings.Join(nonFlags, " ")

		return dbutil.WithDatabase(func(db *database.DB) error {
			_, err := db.SaveCategoryWithInternal(categoryName, isInternal)
			if err != nil {
				return fmt.Errorf("failed to add category: %w", err)
			}

			internalStatus := ""
			if isInternal {
				internalStatus = " (internal)"
			}
			fmt.Printf("Category '%s'%s added successfully\n", categoryName, internalStatus)
			return nil
		})
	},
}

var CategoriesRemove = &Z.Cmd{
	Name:     "remove",
	Summary:  "Remove a category (only if not used by any transactions)",
	Usage:    "remove <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money categories remove <name>")
		}

		categoryName := strings.Join(args, " ")

		return dbutil.WithDatabase(func(db *database.DB) error {
			err := db.DeleteCategory(categoryName)
			if err != nil {
				return fmt.Errorf("failed to remove category: %w", err)
			}

			fmt.Printf("Category '%s' removed successfully\n", categoryName)
			return nil
		})
	},
}

var CategoriesSeed = &Z.Cmd{
	Name:     "seed",
	Summary:  "Populate database with common default categories",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		return dbutil.WithDatabase(func(db *database.DB) error {
			err := db.SeedDefaultCategories()
			if err != nil {
				return fmt.Errorf("failed to seed categories: %w", err)
			}

			fmt.Println("Default categories added successfully")
			return nil
		})
	},
}

var CategoriesSetInternal = &Z.Cmd{
	Name:     "set-internal",
	Summary:  "Mark a category as internal (excludes from budget calculations)",
	Usage:    "set-internal <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money categories set-internal <name>")
		}

		categoryName := strings.Join(args, " ")

		return dbutil.WithDatabase(func(db *database.DB) error {
			err := db.SetCategoryInternalByName(categoryName, true)
			if err != nil {
				return fmt.Errorf("failed to set category as internal: %w", err)
			}

			fmt.Printf("Category '%s' marked as internal\n", categoryName)
			return nil
		})
	},
}

var CategoriesClearInternal = &Z.Cmd{
	Name:     "clear-internal",
	Summary:  "Remove internal flag from a category",
	Usage:    "clear-internal <name>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: money categories clear-internal <name>")
		}

		categoryName := strings.Join(args, " ")

		return dbutil.WithDatabase(func(db *database.DB) error {
			err := db.SetCategoryInternalByName(categoryName, false)
			if err != nil {
				return fmt.Errorf("failed to clear internal flag: %w", err)
			}

			fmt.Printf("Internal flag removed from category '%s'\n", categoryName)
			return nil
		})
	},
}
