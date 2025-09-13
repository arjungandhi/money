package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)

var Transactions = &Z.Cmd{
	Name:    "transactions",
	Summary: "Manage and categorize transactions",
	Commands: []*Z.Cmd{
		help.Cmd,
		Categorize,
	},
}

var Categorize = &Z.Cmd{
	Name:    "categorize",
	Summary: "Interactively categorize uncategorized transactions via LLM",
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement categorization logic
		// 1. Load uncategorized transactions from database
		// 2. Use LLM to suggest categories for each transaction
		// 3. Present interactive prompts for category review and adjustment
		// 4. Save category assignments back to database

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Placeholder implementation
		fmt.Println("TODO: Implement transactions categorize command")
		return nil
	},
}
