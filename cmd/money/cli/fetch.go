package cli

import (
	"fmt"

	Z "github.com/rwxrob/bonzai/z"

	"github.com/arjungandhi/money/pkg/database"
)

var Fetch = &Z.Cmd{
	Name:    "fetch",
	Summary: "Sync latest data from SimpleFIN",
	Call: func(cmd *Z.Cmd, args ...string) error {
		// TODO: Implement fetch logic
		// 1. Load stored credentials from database.DB
		// 2. Fetch data from SimpleFIN /accounts endpoint using simplefin.Client
		// 3. Store accounts, transactions, and organizations in database

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Placeholder implementation
		fmt.Println("TODO: Implement fetch command")
		return nil
	},
}
