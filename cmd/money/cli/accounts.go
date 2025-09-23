package cli

import (
	"fmt"
	"strings"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)

var Accounts = &Z.Cmd{
	Name:    "accounts",
	Aliases: []string{"account", "acc", "a", "act"},
	Summary: "Manage user accounts and account types",
	Commands: []*Z.Cmd{
		help.Cmd,
		AccountsList,
		AccountsType,
		AccountsNickname,
		AccountsDelete,
	},
}

var AccountsType = &Z.Cmd{
	Name:    "type",
	Summary: "Manage account types for better balance organization",
	Commands: []*Z.Cmd{
		help.Cmd,
		AccountsTypeSet,
		AccountsTypeClear,
	},
}

var AccountsNickname = &Z.Cmd{
	Name:    "nickname",
	Summary: "Manage custom account nicknames",
	Commands: []*Z.Cmd{
		help.Cmd,
		AccountsNicknameSet,
		AccountsNicknameClear,
	},
}

var AccountsList = &Z.Cmd{
	Name:     "list",
	Aliases:  []string{"ls", "l"},
	Summary:  "Show all accounts with their current types",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		accounts, err := db.GetAccounts()
		if err != nil {
			return fmt.Errorf("failed to get accounts: %w", err)
		}

		if len(accounts) == 0 {
			fmt.Println("No accounts found. Run 'money fetch' to sync your financial data.")
			return nil
		}

		// Get organizations for display
		orgs, err := db.GetOrganizations()
		if err != nil {
			return fmt.Errorf("failed to get organizations: %w", err)
		}

		orgMap := make(map[string]database.Organization)
		for _, org := range orgs {
			orgMap[org.ID] = org
		}

		// Display accounts with types
		fmt.Println("Account Types")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("%-12s %-25s %-30s %s\n", "Type", "Organization", "Account Name", "Account ID")
		fmt.Println(strings.Repeat("-", 80))

		for _, account := range accounts {
			accountType := "unset"
			if account.AccountType != nil {
				accountType = *account.AccountType
			}

			orgName := account.OrgID
			if org, exists := orgMap[account.OrgID]; exists {
				orgName = org.Name
			}

			// Use DisplayName method to get nickname or original name
			displayName := account.DisplayName()

			// Truncate long names for better display
			if len(orgName) > 25 {
				orgName = orgName[:22] + "..."
			}
			if len(displayName) > 30 {
				displayName = displayName[:27] + "..."
			}

			fmt.Printf("%-12s %-25s %-30s %s\n", accountType, orgName, displayName, account.ID)
		}

		fmt.Println()
		fmt.Println("Available account types: checking, savings, credit, investment, loan, property, other")
		fmt.Println("Use 'money accounts type set <account-id> <type>' to set an account type")

		return nil
	},
}

var AccountsTypeSet = &Z.Cmd{
	Name:     "set",
	Summary:  "Set account type for an account",
	Usage:    "<account-id> <type>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 2 {
			return fmt.Errorf("usage: %s <account-id> <type>", cmd.Usage)
		}

		accountID := args[0]
		accountType := args[1]

		// Validate account type
		validTypes := []string{"checking", "savings", "credit", "investment", "loan", "property", "other"}
		isValid := false
		for _, validType := range validTypes {
			if accountType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid account type: %s. Valid types are: %v", accountType, validTypes)
		}

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Check if account exists
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return err
		}

		// Set the account type
		err = db.SetAccountType(accountID, accountType)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully set account type '%s' for account: %s (%s)\n", accountType, account.Name, accountID)

		return nil
	},
}

var AccountsTypeClear = &Z.Cmd{
	Name:     "clear",
	Summary:  "Clear account type for an account (set to unset)",
	Usage:    "<account-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <account-id>", cmd.Usage)
		}

		accountID := args[0]

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Check if account exists
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return err
		}

		// Clear the account type
		err = db.ClearAccountType(accountID)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully cleared account type for account: %s (%s)\n", account.Name, accountID)

		return nil
	},
}

var AccountsNicknameSet = &Z.Cmd{
	Name:     "set",
	Summary:  "Set a custom nickname for an account",
	Usage:    "<account-id> <nickname>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 2 {
			return fmt.Errorf("usage: %s <account-id> <nickname>", cmd.Usage)
		}

		accountID := args[0]
		// Join remaining args as nickname to support multi-word nicknames
		nickname := strings.Join(args[1:], " ")

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Check if account exists
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return err
		}

		// Set the account nickname
		err = db.SetAccountNickname(accountID, nickname)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully set nickname '%s' for account: %s (%s)\n", nickname, account.Name, accountID)

		return nil
	},
}

var AccountsNicknameClear = &Z.Cmd{
	Name:     "clear",
	Summary:  "Remove custom nickname for an account (revert to original name)",
	Usage:    "<account-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <account-id>", cmd.Usage)
		}

		accountID := args[0]

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Check if account exists
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return err
		}

		// Clear the account nickname
		err = db.ClearAccountNickname(accountID)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully cleared nickname for account: %s (%s)\n", account.Name, accountID)

		return nil
	},
}

var AccountsDelete = &Z.Cmd{
	Name:     "delete",
	Aliases:  []string{"del", "rm"},
	Summary:  "Delete an account and all associated data",
	Usage:    "<account-id>",
	Description: `
Delete an account and all its associated data including:
- Transaction history
- Balance history
- Property details (if property account)

WARNING: This action cannot be undone!

Use 'money accounts list' to see account IDs.
`,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <account-id>", cmd.Usage)
		}

		accountID := args[0]

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Check if account exists and get details
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return err
		}

		// Confirm deletion
		fmt.Printf("Are you sure you want to delete account '%s' (%s)?\n", account.DisplayName(), accountID)
		fmt.Printf("This will permanently delete:\n")
		fmt.Printf("- All transaction history\n")
		fmt.Printf("- All balance history\n")
		if account.AccountType != nil && *account.AccountType == "property" {
			fmt.Printf("- Property details and valuations\n")
		}
		fmt.Printf("\nType 'yes' to confirm: ")

		var confirmation string
		fmt.Scanln(&confirmation)

		if strings.ToLower(confirmation) != "yes" {
			fmt.Println("Account deletion cancelled.")
			return nil
		}

		// Delete the account
		err = db.DeleteAccount(accountID)
		if err != nil {
			return fmt.Errorf("failed to delete account: %w", err)
		}

		fmt.Printf("Successfully deleted account '%s' (%s)\n", account.DisplayName(), accountID)

		return nil
	},
}
