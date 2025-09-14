package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)

var Balance = &Z.Cmd{
	Name:     "balance",
	Summary:  "Show current balance of all accounts and net worth",
	Commands: []*Z.Cmd{help.Cmd},
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

		// Group accounts by account type, then by organization
		accountsByTypeAndOrg := make(map[string]map[string][]database.Account)
		for _, account := range accounts {
			accountType := "unset"
			if account.AccountType != nil {
				accountType = *account.AccountType
			}
			
			if accountsByTypeAndOrg[accountType] == nil {
				accountsByTypeAndOrg[accountType] = make(map[string][]database.Account)
			}
			accountsByTypeAndOrg[accountType][account.OrgID] = append(accountsByTypeAndOrg[accountType][account.OrgID], account)
		}

		// Define account type order (unset at the end)
		typeOrder := []string{"checking", "savings", "credit", "investment", "loan", "other", "unset"}
		var accountTypes []string
		
		// Add types in preferred order if they exist
		for _, accountType := range typeOrder {
			if _, exists := accountsByTypeAndOrg[accountType]; exists {
				accountTypes = append(accountTypes, accountType)
			}
		}

		// Display balances grouped by account type, then organization
		var totalNetWorth int64
		currencyTotals := make(map[string]int64)
		
		fmt.Println("Account Balances by Type")
		fmt.Println(strings.Repeat("=", 24))
		fmt.Println()

		for _, accountType := range accountTypes {
			accountsByOrg := accountsByTypeAndOrg[accountType]
			
			// Display account type header with appropriate emoji
			typeIcon := getTypeIcon(accountType)
			typeDisplayName := getTypeDisplayName(accountType)
			fmt.Printf("%s %s\n", typeIcon, typeDisplayName)
			fmt.Println(strings.Repeat("-", len(typeDisplayName)+4))
			
			// Sort organization IDs within this account type
			var orgIDs []string
			for orgID := range accountsByOrg {
				orgIDs = append(orgIDs, orgID)
			}
			sort.Strings(orgIDs)

			typeTotalsByCurrency := make(map[string]int64)
			
			for _, orgID := range orgIDs {
				accounts := accountsByOrg[orgID]
				org, exists := orgMap[orgID]
				
				// Display organization name
				if exists {
					fmt.Printf("📍 %s\n", org.Name)
				} else {
					fmt.Printf("📍 Organization %s\n", orgID)
				}

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
					typeTotalsByCurrency[account.Currency] += int64(account.Balance)
					currencyTotals[account.Currency] += int64(account.Balance)
				}

				fmt.Println()
			}

			// Display account type totals by currency  
			if len(typeTotalsByCurrency) == 1 {
				// Single currency - show simple total
				for currency, total := range typeTotalsByCurrency {
					fmt.Printf("%s Total: %s\n", typeDisplayName, formatCurrency(int(total), currency))
				}
			} else {
				// Multiple currencies - show breakdown
				fmt.Printf("%s Totals:\n", typeDisplayName)
				for currency, total := range typeTotalsByCurrency {
					fmt.Printf("  %s: %s\n", currency, formatCurrency(int(total), currency))
				}
			}
			fmt.Println()

			// For net worth calculation, assume USD for simplicity (in a real app, we'd convert currencies)
			if len(typeTotalsByCurrency) == 1 {
				for _, total := range typeTotalsByCurrency {
					totalNetWorth += total
				}
			} else {
				// If mixed currencies, only add USD amounts to net worth
				if usdTotal, exists := typeTotalsByCurrency["USD"]; exists {
					totalNetWorth += usdTotal
				}
			}
		}

		// Display net worth
		fmt.Println(strings.Repeat("=", 24))
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
		fmt.Println(strings.Repeat("=", 24))

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

// getTypeIcon returns the appropriate emoji for the account type
func getTypeIcon(accountType string) string {
	switch accountType {
	case "checking":
		return "💰"
	case "savings":
		return "🏦"
	case "credit":
		return "💳"
	case "investment":
		return "📊"
	case "loan":
		return "🏠"
	case "other":
		return "💼"
	case "unset":
		return "❓"
	default:
		return "📋"
	}
}

// getTypeDisplayName returns the formatted display name for the account type
func getTypeDisplayName(accountType string) string {
	switch accountType {
	case "checking":
		return "Checking Accounts"
	case "savings":
		return "Savings Accounts"
	case "credit":
		return "Credit Accounts"
	case "investment":
		return "Investment Accounts"
	case "loan":
		return "Loan Accounts"
	case "other":
		return "Other Accounts"
	case "unset":
		return "Unset Accounts"
	default:
		return strings.Title(accountType) + " Accounts"
	}
}
