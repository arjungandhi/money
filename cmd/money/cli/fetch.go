package cli

import (
	"fmt"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/simplefin"
)

var Fetch = &Z.Cmd{
	Name:     "fetch",
	Summary:  "Sync latest data from SimpleFIN",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		fmt.Println("Fetching data from SimpleFIN...")
		
		// 1. Initialize database connection
		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// 2. Load stored credentials
		accessURL, username, password, err := db.GetCredentials()
		if err != nil {
			return fmt.Errorf("failed to load credentials: %w", err)
		}

		// 3. Create SimpleFIN client and fetch data
		client := simplefin.NewClient(accessURL, username, password)
		
		fmt.Println("Connecting to SimpleFIN API...")
		accountsData, err := client.GetAccounts()
		if err != nil {
			return fmt.Errorf("failed to fetch account data from SimpleFIN: %w", err)
		}

		// Track sync statistics
		var stats syncStats
		stats.startTime = time.Now()

		// 4. Extract and store organizations from accounts
		// Since organizations are now embedded in accounts, we need to collect unique organizations
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

		// 5. Process and store accounts
		fmt.Printf("Processing %d accounts...\n", len(accountsData.Accounts))
		for _, account := range accountsData.Accounts {
			// Convert string balance to cents
			balance, err := simplefin.ParseAmountToCents(account.Balance)
			if err != nil {
				return fmt.Errorf("failed to parse balance for account %s: %w", account.Name, err)
			}

			// Convert available balance if present
			var availableBalance *int
			if account.AvailableBalance != nil {
				availBalCents, err := simplefin.ParseAmountToCents(*account.AvailableBalance)
				if err != nil {
					return fmt.Errorf("failed to parse available balance for account %s: %w", account.Name, err)
				}
				availableBalance = &availBalCents
			}

			// Convert unix timestamp to ISO string if present
			balanceDate := ""
			if account.BalanceDate != nil {
				balanceDate = simplefin.UnixTimestampToISO(*account.BalanceDate)
			}

			// Normalize currency - default empty currencies to USD
			currency := account.Currency
			if currency == "" {
				currency = "USD"
			}

			if err := db.SaveAccount(
				account.ID,
				account.Org.ID, // Use embedded organization ID
				account.Name,
				currency,
				balance,
				availableBalance,
				balanceDate,
			); err != nil {
				return fmt.Errorf("failed to save account %s: %w", account.Name, err)
			}
			stats.accountsProcessed++
		}

		// 6. Process and store transactions
		fmt.Printf("Processing transactions...\n")
		for _, account := range accountsData.Accounts {
			for _, transaction := range account.Transactions {
				// Check if transaction already exists to track new vs existing
				exists, err := db.TransactionExists(transaction.ID)
				if err != nil {
					return fmt.Errorf("failed to check transaction existence: %w", err)
				}

				// Convert string amount to cents
				amount, err := simplefin.ParseAmountToCents(transaction.Amount)
				if err != nil {
					return fmt.Errorf("failed to parse amount for transaction %s: %w", transaction.ID, err)
				}

				// Convert unix timestamp to ISO string
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

		// 7. Provide summary
		stats.duration = time.Since(stats.startTime)
		printSyncSummary(stats)

		return nil
	},
}

// syncStats tracks synchronization statistics
type syncStats struct {
	startTime             time.Time
	duration              time.Duration
	orgsProcessed         int
	accountsProcessed     int
	transactionsProcessed int
	newTransactions       int
}

// printSyncSummary displays a summary of the synchronization results
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
