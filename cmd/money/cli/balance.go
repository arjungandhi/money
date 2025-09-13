package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	Z "github.com/rwxrob/bonzai/z"

	"github.com/arjungandhi/money/pkg/database"
)

var Balance = &Z.Cmd{
	Name:    "balance",
	Summary: "Show current balance of all accounts and net worth",
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Get all accounts
		accounts, err := db.GetAccounts()
		if err != nil {
			return fmt.Errorf("failed to get accounts: %w", err)
		}

		if len(accounts) == 0 {
			fmt.Println("No accounts found. Run 'money fetch' to sync your financial data.")
			return nil
		}

		// Get all organizations
		orgs, err := db.GetOrganizations()
		if err != nil {
			return fmt.Errorf("failed to get organizations: %w", err)
		}

		// Create organization lookup map
		orgMap := make(map[string]database.Organization)
		for _, org := range orgs {
			orgMap[org.ID] = org
		}

		// Group accounts by organization
		accountsByOrg := make(map[string][]database.Account)
		for _, account := range accounts {
			accountsByOrg[account.OrgID] = append(accountsByOrg[account.OrgID], account)
		}

		// Sort organization IDs for consistent output
		var orgIDs []string
		for orgID := range accountsByOrg {
			orgIDs = append(orgIDs, orgID)
		}
		sort.Strings(orgIDs)

		// Display balances grouped by organization
		var totalNetWorth int64
		currencyTotals := make(map[string]int64)
		
		fmt.Println("Account Balances")
		fmt.Println(strings.Repeat("=", 70))
		fmt.Println()

		for _, orgID := range orgIDs {
			accounts := accountsByOrg[orgID]
			org, exists := orgMap[orgID]
			
			// Display organization name
			if exists {
				fmt.Printf("📍 %s\n", org.Name)
			} else {
				fmt.Printf("📍 Organization %s\n", orgID)
			}
			fmt.Println(strings.Repeat("-", 50))

			orgTotalsByCurrency := make(map[string]int64)
			for _, account := range accounts {
				// Format balance
				balanceStr := formatCurrency(account.Balance, account.Currency)
				
				// Format balance date if available
				dateStr := ""
				if account.BalanceDate != nil {
					if parsedDate, err := time.Parse(time.RFC3339, *account.BalanceDate); err == nil {
						dateStr = fmt.Sprintf(" (as of %s)", parsedDate.Format("Jan 2, 2006"))
					} else {
						// Try to parse other common date formats
						dateStr = fmt.Sprintf(" (as of %s)", *account.BalanceDate)
					}
				}

				// Display account info with better formatting
				fmt.Printf("  %-35s %15s%s\n", account.Name, balanceStr, dateStr)
				
				// Add to currency-specific totals
				orgTotalsByCurrency[account.Currency] += int64(account.Balance)
				currencyTotals[account.Currency] += int64(account.Balance)
			}

			// Display organization totals by currency
			fmt.Println(strings.Repeat("-", 50))
			if len(orgTotalsByCurrency) == 1 {
				// Single currency - show simple total
				for currency, total := range orgTotalsByCurrency {
					fmt.Printf("  %-35s %15s\n", "Organization Total:", formatCurrency(int(total), currency))
				}
			} else {
				// Multiple currencies - show breakdown
				fmt.Printf("  %-35s\n", "Organization Totals:")
				for currency, total := range orgTotalsByCurrency {
					fmt.Printf("    %-33s %15s\n", fmt.Sprintf("%s:", currency), formatCurrency(int(total), currency))
				}
			}
			fmt.Println()

			// For net worth calculation, assume USD for simplicity (in a real app, we'd convert currencies)
			if len(orgTotalsByCurrency) == 1 {
				for _, total := range orgTotalsByCurrency {
					totalNetWorth += total
				}
			} else {
				// If mixed currencies, only add USD amounts to net worth
				if usdTotal, exists := orgTotalsByCurrency["USD"]; exists {
					totalNetWorth += usdTotal
				}
			}
		}

		// Display net worth
		fmt.Println(strings.Repeat("=", 70))
		if len(currencyTotals) == 1 {
			// Single currency across all accounts
			for currency, total := range currencyTotals {
				fmt.Printf("💰 Net Worth: %s\n", formatCurrency(int(total), currency))
			}
		} else {
			// Multiple currencies - show breakdown
			fmt.Printf("💰 Net Worth by Currency:\n")
			for currency, total := range currencyTotals {
				fmt.Printf("   %s: %s\n", currency, formatCurrency(int(total), currency))
			}
			if usdTotal, exists := currencyTotals["USD"]; exists {
				fmt.Printf("\n   Primary (USD): %s\n", formatCurrency(int(usdTotal), "USD"))
			}
		}
		fmt.Println(strings.Repeat("=", 70))

		return nil
	},
}

// formatCurrency converts cents to dollars and formats with currency symbol and thousands separators
func formatCurrency(cents int, currency string) string {
	// Get currency symbol
	symbol := getCurrencySymbol(currency)
	
	// Use integer arithmetic to avoid floating point precision issues
	var wholePart int64
	var decimalPart int
	var negative bool
	
	if cents < 0 {
		negative = true
		cents = -cents
	}
	
	wholePart = int64(cents / 100)
	decimalPart = cents % 100
	
	// Format whole part with commas
	wholeStr := formatWithCommas(wholePart)
	
	// Combine parts
	if negative {
		return fmt.Sprintf("-%s%s.%02d", symbol, wholeStr, decimalPart)
	} else {
		return fmt.Sprintf("%s%s.%02d", symbol, wholeStr, decimalPart)
	}
}

// formatWithCommas adds thousands separators to a number
func formatWithCommas(n int64) string {
	if n == 0 {
		return "0"
	}
	
	// Convert to string and reverse for easier processing
	str := fmt.Sprintf("%d", n)
	result := ""
	
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(char)
	}
	
	return result
}

// getCurrencySymbol returns the appropriate symbol for the currency
func getCurrencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "CAD":
		return "C$"
	case "AUD":
		return "A$"
	default:
		return currency + " "
	}
}
