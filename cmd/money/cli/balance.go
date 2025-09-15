package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"os"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
	"github.com/guptarohit/asciigraph"

	"github.com/arjungandhi/money/pkg/database"
)

var Balance = &Z.Cmd{
	Name:     "balance",
	Aliases:  []string{"bal", "b"},
	Summary:  "Show current balance of all accounts and net worth with trending graph",
	Usage:    "[--days|-d <number>]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		// Parse days flag (default 30)
		days := 30
		for i, arg := range args {
			if (arg == "--days" || arg == "-d") && i+1 < len(args) {
				if parsedDays, err := strconv.Atoi(args[i+1]); err == nil && parsedDays > 0 {
					days = parsedDays
				}
				break
			}
		}

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

		// Display balance trend graph first
		err = displayBalanceTrends(db, accounts, days)
		if err != nil {
			// Don't fail the command if graph generation fails, just log a warning
			fmt.Printf("Warning: could not generate balance trend graph: %v\n", err)
		}


		// Show properly aligned current balances table
		fmt.Println("\nCurrent Account Balances")
		fmt.Println(strings.Repeat("=", 70))

		// Create tabwriter for proper alignment
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Type\tInstitution\tAccount\tBalance\n")
		fmt.Fprintf(w, "────\t───────────\t───────\t───────\n")

		var totalNetWorth int64
		for _, account := range accounts {
			accountType := "unset"
			if account.AccountType != nil {
				accountType = *account.AccountType
			}

			typeIcon := getTypeIcon(accountType)
			balanceStr := formatCurrency(account.Balance, account.Currency)

			// Get institution name
			institutionName := account.OrgID // fallback to ID
			if org, exists := orgMap[account.OrgID]; exists {
				institutionName = org.Name
			}

			// Truncate institution name if too long
			institutionName = truncateString(institutionName, 15)

			fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n",
				typeIcon, strings.Title(accountType), institutionName, account.DisplayName(), balanceStr)
			totalNetWorth += int64(account.Balance)
		}

		w.Flush()

		fmt.Println(strings.Repeat("=", 70))
		fmt.Printf("💰 Net Worth: %s\n", formatCurrency(int(totalNetWorth), "USD"))
		fmt.Println(strings.Repeat("=", 70))

		return nil
	},
}

// truncateString truncates a string to maxLength characters, adding "..." if truncated
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
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

	str := fmt.Sprintf("%d", n)

	// Work backwards in groups of 3
	var parts []string
	for i := len(str); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{str[start:i]}, parts...)
	}

	return strings.Join(parts, ",")
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

// displayBalanceTrends shows an ASCII graph of balance trends over time grouped by account type
func displayBalanceTrends(db *database.DB, accounts []database.Account, days int) error {
	fmt.Println()
	fmt.Printf("Balance Trends (Last %d Days)\n", days)
	fmt.Println(strings.Repeat("=", 30))

	// Get all balance history for the period
	history, err := db.GetAllBalanceHistory(days)
	if err != nil {
		return fmt.Errorf("failed to get balance history: %w", err)
	}

	if len(history) == 0 {
		fmt.Println("No historical balance data available. Run 'money fetch' to start collecting balance trends.")
		return nil
	}

	// Create account type lookup map
	accountTypeMap := make(map[string]string)
	for _, account := range accounts {
		accountType := "unset"
		if account.AccountType != nil {
			accountType = *account.AccountType
		}
		accountTypeMap[account.ID] = accountType
	}

	// Group history by account type and date - use latest balance per account per day
	accountDailyBalances := make(map[string]map[string]int64) // [accountID][date] = latestBalance
	typeHistoryMap := make(map[string]map[string]int64) // [accountType][date] = totalBalance
	dateSet := make(map[string]bool)

	// First, get the latest balance per account per day
	for _, bh := range history {
		// Parse the recorded_at timestamp and format as date
		recordedTime, err := time.Parse("2006-01-02 15:04:05", bh.RecordedAt)
		if err != nil {
			// Try alternative format
			recordedTime, err = time.Parse(time.RFC3339, bh.RecordedAt)
			if err != nil {
				continue // Skip this entry if we can't parse the date
			}
		}
		dateStr := recordedTime.Format("2006-01-02")

		if accountDailyBalances[bh.AccountID] == nil {
			accountDailyBalances[bh.AccountID] = make(map[string]int64)
		}

		// Store the balance - since history is ordered by recorded_at ASC,
		// later entries will overwrite earlier ones, giving us the latest balance for each day
		accountDailyBalances[bh.AccountID][dateStr] = int64(bh.Balance)
		dateSet[dateStr] = true
	}

	// Now aggregate by account type
	for accountID, dailyBalances := range accountDailyBalances {
		accountType, exists := accountTypeMap[accountID]
		if !exists {
			accountType = "unset"
		}

		if typeHistoryMap[accountType] == nil {
			typeHistoryMap[accountType] = make(map[string]int64)
		}

		for date, balance := range dailyBalances {
			typeHistoryMap[accountType][date] += balance
		}
	}

	// Convert dates to sorted slice
	var dates []string
	for date := range dateSet {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	if len(dates) < 2 {
		fmt.Println("Not enough historical data points to generate a meaningful trend graph.")
		return nil
	}

	// Create multi-line graph with different series for each account type
	typeOrder := []string{"checking", "savings", "investment", "credit", "loan", "other", "unset"}
	var allSeries [][]float64
	var seriesLabels []string
	var seriesColors []asciigraph.AnsiColor
	var activeTypes []string // Track which types actually have data

	// Define colors for each account type
	colorMap := map[string]asciigraph.AnsiColor{
		"checking":   asciigraph.Green,
		"savings":    asciigraph.Blue,
		"investment": asciigraph.Magenta,
		"credit":     asciigraph.Red,
		"loan":       asciigraph.Yellow,
		"other":      asciigraph.Cyan,
		"unset":      asciigraph.Default,
	}

	// Prepare data series for each account type
	for _, accountType := range typeOrder {
		typeHistory, exists := typeHistoryMap[accountType]
		if !exists || len(typeHistory) == 0 {
			continue
		}

		// Prepare data for this account type - only include dates with actual data
		var values []float64
		var hasData bool
		for _, date := range dates {
			if balance, dateExists := typeHistory[date]; dateExists {
				values = append(values, float64(balance)/100.0) // Convert cents to dollars
				hasData = true
			}
			// Don't pad with zeros or previous values - only use actual data points
		}

		if !hasData {
			continue
		}

		allSeries = append(allSeries, values)
		activeTypes = append(activeTypes, accountType) // Track active types
		typeDisplayName := getTypeDisplayName(accountType)
		seriesLabels = append(seriesLabels, typeDisplayName)
		if color, exists := colorMap[accountType]; exists {
			seriesColors = append(seriesColors, color)
		} else {
			seriesColors = append(seriesColors, asciigraph.Default)
		}
	}

	// Display the multi-series graph if we have data
	if len(allSeries) > 0 {
		fmt.Printf("\n📊 Balance Trends by Account Type\n")

		// Calculate appropriate y-axis bounds
		var minVal, maxVal float64
		if len(allSeries) > 0 && len(allSeries[0]) > 0 {
			minVal = allSeries[0][0]
			maxVal = allSeries[0][0]
			for _, series := range allSeries {
				for _, value := range series {
					if value < minVal {
						minVal = value
					}
					if value > maxVal {
						maxVal = value
					}
				}
			}
		}

		// Create multi-series plot with proper bounds and legends
		graph := asciigraph.PlotMany(allSeries,
			asciigraph.Height(12),
			asciigraph.Width(70),
			asciigraph.LowerBound(minVal*0.95), // Add some padding below
			asciigraph.UpperBound(maxVal*1.05), // Add some padding above
			asciigraph.SeriesColors(seriesColors...),
			asciigraph.SeriesLegends(seriesLabels...),
			asciigraph.Caption(fmt.Sprintf("Balance Trends (Last %d Days)", days)))
		fmt.Println(graph)


		// Show trend summary
		fmt.Println("\nTrend Summary:")
		for i, label := range seriesLabels {
			typeIcon := getTypeIcon(activeTypes[i])
			currentValue := allSeries[i][len(allSeries[i])-1]

			var trend string
			if len(allSeries[i]) > 1 {
				startValue := allSeries[i][0]
				change := currentValue - startValue
				changePercent := 0.0
				if startValue != 0 {
					changePercent = (change / startValue) * 100
				}

				if change > 0 {
					trend = fmt.Sprintf(" (↑ $%s, +%.1f%%)", formatWithCommas(int64(change)), changePercent)
				} else if change < 0 {
					trend = fmt.Sprintf(" (↓ $%s, %.1f%%)", formatWithCommas(int64(-change)), changePercent)
				} else {
					trend = " (→ No change)"
				}
			}

			fmt.Printf("  %s %s: %s%s\n",
				typeIcon, label, formatCurrency(int(currentValue*100), "USD"), trend)
		}

		// Calculate and show net worth
		if len(allSeries) > 1 {
			var netWorthStart, netWorthCurrent float64
			for _, series := range allSeries {
				// Include all account types in net worth calculation
				// Credit and loan balances are already negative in the data
				netWorthStart += series[0]
				netWorthCurrent += series[len(series)-1]
			}

			netWorthChange := netWorthCurrent - netWorthStart
			netWorthChangePercent := 0.0
			if netWorthStart != 0 {
				netWorthChangePercent = (netWorthChange / netWorthStart) * 100
			}

			fmt.Printf("\n💰 Net Worth: %s", formatCurrency(int(netWorthCurrent*100), "USD"))
			if netWorthChange > 0 {
				fmt.Printf(" (↑ $%s, +%.1f%% over %d days)",
					formatWithCommas(int64(netWorthChange)), netWorthChangePercent, days)
			} else if netWorthChange < 0 {
				fmt.Printf(" (↓ $%s, %.1f%% over %d days)",
					formatWithCommas(int64(-netWorthChange)), netWorthChangePercent, days)
			} else {
				fmt.Printf(" (→ No change over %d days)", days)
			}
			fmt.Println()
		}
	}

	return nil
}
