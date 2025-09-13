package cli

import (
	"fmt"
	"time"

	Z "github.com/rwxrob/bonzai/z"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/simplefin"
)

var Fetch = &Z.Cmd{
	Name:    "fetch",
	Summary: "Sync latest data from SimpleFIN",
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

		// 4. Process and store organizations
		fmt.Printf("Processing %d organizations...\n", len(accountsData.Organizations))
		for _, org := range accountsData.Organizations {
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
			balanceDate := ""
			if account.BalanceDate != nil {
				balanceDate = *account.BalanceDate
			}

			if err := db.SaveAccount(
				account.ID,
				account.OrgID,
				account.Name,
				account.Currency,
				account.Balance,
				account.AvailableBalance,
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

				pending := false
				if transaction.Pending != nil {
					pending = *transaction.Pending
				}

				if err := db.SaveTransaction(
					transaction.ID,
					account.ID,
					transaction.Posted,
					transaction.Amount,
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
