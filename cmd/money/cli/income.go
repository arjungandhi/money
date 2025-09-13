package cli

import (
	"fmt"
	
	Z "github.com/rwxrob/bonzai/z"
	
	"github.com/arjungandhi/money/pkg/database"
)

var Income = &Z.Cmd{
	Name:    "income",
	Summary: "Show breakdown of income by category for time period",
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement income logic
		// 1. Parse time period (default: this month)
		// 2. Query transactions for positive amounts using database.DB.GetTransactions()
		// 3. Categorize and display breakdown
		
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()
		
		// Placeholder implementation
		fmt.Println("TODO: Implement income command")
		return nil
	},
}