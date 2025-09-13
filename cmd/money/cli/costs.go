package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"

	"github.com/arjungandhi/money/pkg/database"
)

var Costs = &Z.Cmd{
	Name:    "costs",
	Summary: "Show breakdown of costs by category for time period",
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement costs logic
		// 1. Parse time period (default: this month)
		// 2. Query transactions for negative amounts using database.DB.GetTransactions()
		// 3. Categorize and display breakdown

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Placeholder implementation
		fmt.Println("TODO: Implement costs command")
		return nil
	},
}
