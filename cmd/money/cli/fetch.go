package cli

import (
	"fmt"
	"strconv"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/simplefin"
)

var Fetch = &Z.Cmd{
	Name:     "fetch",
	Aliases:  []string{"f", "sync"},
	Summary:  "Sync latest data from SimpleFIN",
	Usage:    "[--days|-d <number>] [--all|-a]",
	Description: `
Sync account and transaction data from SimpleFIN.

By default, fetches complete transaction history. Use --days to limit
to a specific number of recent days.

Examples:
  money fetch           # Complete history (default)
  money fetch -d 7      # Last 7 days only
  money fetch --days 30 # Last 30 days only
  money fetch --all     # Complete history (explicit)
`,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Println("Fetching data from SimpleFIN...")

		days := 30
		fetchAll := true
		for i, arg := range args {
			switch {
			case (arg == "--days" || arg == "-d") && i+1 < len(args):
				if parsedDays, err := strconv.Atoi(args[i+1]); err == nil && parsedDays > 0 {
					days = parsedDays
					fetchAll = false
				}
			case arg == "--all" || arg == "-a":
				fetchAll = true
			}
		}

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		accessURL, username, password, err := db.GetCredentials()
		if err != nil {
			return fmt.Errorf("failed to load credentials: %w", err)
		}

		client := simplefin.NewClient(accessURL, username, password)

		fmt.Println("Connecting to SimpleFIN API...")

		var options *simplefin.AccountsOptions
		if fetchAll {
			fmt.Println("Fetching complete transaction history...")
			options = nil
		} else {
			startDate := time.Now().AddDate(0, 0, -days)
			fmt.Printf("Fetching transactions from the last %d days...\n", days)
			options = &simplefin.AccountsOptions{
				StartDate: &startDate,
			}
		}

		accountsData, err := client.GetAccountsWithOptions(options)
		if err != nil {
			return fmt.Errorf("failed to fetch account data from SimpleFIN: %w", err)
		}

		var stats syncStats
		stats.startTime = time.Now()

		orgMap := make(map[string]simplefin.Organization)
		for _, account := range accountsData.Accounts {
			orgMap[account.Org.ID] = account.Org
		}

		fmt.Printf("Processing %d organizations...\n", len(orgMap))
		for _, org := range orgMap {
			url := ""
			if org.URL != nil {
				url = *org.URL
			}
			
			if err := db.SaveOrganization(org.ID, org.Name, url); err != nil {
				return fmt.Errorf("failed to save organization %s: %w", org.Name, err)
			}
			stats.orgsProcessed++
		}

		fmt.Printf("Processing %d accounts...\n", len(accountsData.Accounts))
		for _, account := range accountsData.Accounts {
			balance, err := simplefin.ParseAmountToCents(account.Balance)
			if err != nil {
				return fmt.Errorf("failed to parse balance for account %s: %w", account.Name, err)
			}

			var availableBalance *int
			if account.AvailableBalance != nil {
				availBalCents, err := simplefin.ParseAmountToCents(*account.AvailableBalance)
				if err != nil {
					return fmt.Errorf("failed to parse available balance for account %s: %w", account.Name, err)
				}
				availableBalance = &availBalCents
			}

			balanceDate := ""
			if account.BalanceDate != nil {
				balanceDate = simplefin.UnixTimestampToISO(*account.BalanceDate)
			}

			currency := account.Currency
			if currency == "" {
				currency = "USD"
			}

			if err := db.SaveAccount(
				account.ID,
				account.Org.ID,
				account.Name,
				currency,
				balance,
				availableBalance,
				balanceDate,
			); err != nil {
				return fmt.Errorf("failed to save account %s: %w", account.Name, err)
			}

			if err := db.SaveBalanceHistory(account.ID, balance, availableBalance); err != nil {
				return fmt.Errorf("failed to save balance history for account %s: %w", account.Name, err)
			}

			stats.accountsProcessed++
		}

		fmt.Printf("Processing transactions...\n")
		for _, account := range accountsData.Accounts {
			for _, transaction := range account.Transactions {
				exists, err := db.TransactionExists(transaction.ID)
				if err != nil {
					return fmt.Errorf("failed to check transaction existence: %w", err)
				}

				amount, err := simplefin.ParseAmountToCents(transaction.Amount)
				if err != nil {
					return fmt.Errorf("failed to parse amount for transaction %s: %w", transaction.ID, err)
				}

				postedDate := simplefin.UnixTimestampToISO(transaction.Posted)

				pending := false
				if transaction.Pending != nil {
					pending = *transaction.Pending
				}

				if err := db.SaveTransaction(
					transaction.ID,
					account.ID,
					postedDate,
					amount,
					transaction.Description,
					pending,
				); err != nil {
					return fmt.Errorf("failed to save transaction %s: %w", transaction.ID, err)
				}

				if !exists {
					stats.newTransactions++
				}
				stats.transactionsProcessed++
			}
		}

		stats.duration = time.Since(stats.startTime)
		printSyncSummary(stats)

		return nil
	},
}

type syncStats struct {
	startTime             time.Time
	duration              time.Duration
	orgsProcessed         int
	accountsProcessed     int
	transactionsProcessed int
	newTransactions       int
}

func printSyncSummary(stats syncStats) {
	fmt.Printf("\nSync Summary:\n")
	fmt.Printf("  Duration: %v\n", stats.duration.Round(time.Millisecond))
	fmt.Printf("  Organizations: %d processed\n", stats.orgsProcessed)
	fmt.Printf("  Accounts: %d processed\n", stats.accountsProcessed)
	fmt.Printf("  Transactions: %d processed (%d new)\n", stats.transactionsProcessed, stats.newTransactions)
	
	if stats.newTransactions > 0 {
		fmt.Printf("\nFetch completed successfully! %d new transactions were added.\n", stats.newTransactions)
	} else {
		fmt.Printf("\nFetch completed successfully! All data is up to date.\n")
	}
}
