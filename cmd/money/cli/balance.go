package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/internal/dbutil"
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/format"
	"github.com/arjungandhi/money/pkg/property"
	"github.com/arjungandhi/money/pkg/table"
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

		return dbutil.WithDatabase(func(db *database.DB) error {

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
			typeOrder := []string{"checking", "savings", "credit", "investment", "loan", "property", "other", "unset"}
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

			// Add spacing between graph and account balances table
			fmt.Println()

			// Create account balances table
			config := table.DefaultConfig()
			config.Title = "💰 Account Balances"
			config.MaxColumnWidth = 30

			balancesTable := table.NewWithConfig(config, "Account", "Institution", "Balance")

			// Initialize property service for property account details
			propertyService := property.NewService(db)

			var totalNetWorth int64
			for _, account := range accounts {
				accountType := "unset"
				if account.AccountType != nil {
					accountType = *account.AccountType
				}

				typeIcon := getTypeIcon(accountType)
				balanceStr := format.Currency(account.Balance, account.Currency)

				// Get institution name
				institutionName := account.OrgID // fallback to ID
				if org, exists := orgMap[account.OrgID]; exists {
					institutionName = org.Name
				}

				// For property accounts, show address instead of institution
				displayName := account.DisplayName()
				if accountType == "property" {
					if propertyDetails, err := propertyService.GetPropertyDetails(account.ID); err == nil {
						institutionName = "Property"
						displayName = fmt.Sprintf("%s, %s", propertyDetails.Address, propertyDetails.City)
					}
				}

				accountDisplayName := fmt.Sprintf("%s %s", typeIcon, displayName)
				balancesTable.AddRow(accountDisplayName, institutionName, balanceStr)
				totalNetWorth += int64(account.Balance)
			}

			if err := balancesTable.Render(); err != nil {
				return fmt.Errorf("failed to render balances table: %w", err)
			}

			// Show totals by account type
			fmt.Println("\n📊 Summary by Type")
			fmt.Println(strings.Repeat("─", 50))

			// Calculate totals by account type
			accountTypeTotals := make(map[string]int64)
			accountTypeCounts := make(map[string]int)

			for _, account := range accounts {
				accountType := "unset"
				if account.AccountType != nil {
					accountType = *account.AccountType
				}
				accountTypeTotals[accountType] += int64(account.Balance)
				accountTypeCounts[accountType]++
			}

			// Create summary table
			summaryTable := table.New("Type", "Total", "Accounts")

			// Display totals in the same order as main table
			for _, accountType := range typeOrder {
				if total, exists := accountTypeTotals[accountType]; exists {
					typeIcon := getTypeIcon(accountType)
					count := accountTypeCounts[accountType]
					totalStr := format.Currency(int(total), "USD")

					// Use consistent formatting for account type names
					accountTypeName := strings.Title(accountType)
					displayName := fmt.Sprintf("%s %s", typeIcon, accountTypeName)

					summaryTable.AddRow(displayName, totalStr, fmt.Sprintf("%d", count))
				}
			}

			if err := summaryTable.Render(); err != nil {
				return fmt.Errorf("failed to render summary table: %w", err)
			}

			return nil
		})
	},
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
		return "💸"
	case "property":
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
	case "property":
		return "Property Accounts"
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
	typeHistoryMap := make(map[string]map[string]int64)       // [accountType][date] = totalBalance
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
	typeOrder := []string{"checking", "savings", "investment", "credit", "loan", "property", "other", "unset"}
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
		"property":   asciigraph.White,
		"other":      asciigraph.Cyan,
		"unset":      asciigraph.Default,
	}

	// Prepare data series for each account type
	for _, accountType := range typeOrder {
		typeHistory, exists := typeHistoryMap[accountType]
		if !exists || len(typeHistory) == 0 {
			continue
		}

		// Prepare data for this account type - maintain alignment with dates array
		var values []float64
		var hasData bool
		var lastKnownBalance float64

		for _, date := range dates {
			if balance, dateExists := typeHistory[date]; dateExists {
				lastKnownBalance = float64(balance) / 100.0 // Convert cents to dollars
				hasData = true
			}
			// Use the last known balance for dates without data to maintain proper alignment
			values = append(values, lastKnownBalance)
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

	// Create three separate charts: Non-Cash, Cash, and Net Worth
	fmt.Printf("📊 Trends (Last %d Days)\n", days)

	// Define account categories
	cashAccountTypes := map[string]bool{
		"checking": true,
		"savings":  true,
		"credit":   true,
	}

	nonCashAccountTypes := map[string]bool{
		"investment": true,
		"property":   true,
		"loan":       true,
		"other":      true,
	}

	// 1. NON-CASH ACCOUNTS CHART (sum all non-cash account types)
	var nonCashSumSeries []float64
	for dateIdx := range dates {
		var dailyNonCashSum float64
		for i, accountType := range activeTypes {
			if nonCashAccountTypes[accountType] && dateIdx < len(allSeries[i]) {
				dailyNonCashSum += allSeries[i][dateIdx]
			}
		}
		nonCashSumSeries = append(nonCashSumSeries, dailyNonCashSum)
	}

	if len(nonCashSumSeries) > 0 {
		displaySingleChart("💰 Non-Cash", nonCashSumSeries, asciigraph.Blue, days)
	}

	// 2. CASH ACCOUNTS CHART (sum all cash account types)
	var cashSumSeries []float64
	for dateIdx := range dates {
		var dailyCashSum float64
		for i, accountType := range activeTypes {
			if cashAccountTypes[accountType] && dateIdx < len(allSeries[i]) {
				dailyCashSum += allSeries[i][dateIdx]
			}
		}
		cashSumSeries = append(cashSumSeries, dailyCashSum)
	}

	if len(cashSumSeries) > 0 {
		displaySingleChart("💵 Cash", cashSumSeries, asciigraph.Green, days)
	}

	// 3. NET WORTH CHART
	// Calculate daily net worth across all account types
	var netWorthSeries []float64
	for dateIdx := range dates {
		var dailyNetWorth float64
		for _, series := range allSeries {
			if dateIdx < len(series) {
				dailyNetWorth += series[dateIdx]
			}
		}
		netWorthSeries = append(netWorthSeries, dailyNetWorth)
	}

	if len(netWorthSeries) > 1 {
		// Check for meaningful variation in net worth
		minNetWorth := netWorthSeries[0]
		maxNetWorth := netWorthSeries[0]
		for _, val := range netWorthSeries {
			if val < minNetWorth {
				minNetWorth = val
			}
			if val > maxNetWorth {
				maxNetWorth = val
			}
		}

		variation := maxNetWorth - minNetWorth
		if variation > 10.0 {
			// Show net worth trend summary
			netWorthChange := netWorthSeries[len(netWorthSeries)-1] - netWorthSeries[0]
			netWorthChangePercent := 0.0
			if netWorthSeries[0] != 0 {
				netWorthChangePercent = (netWorthChange / netWorthSeries[0]) * 100
			}

			var trend string
			if netWorthChange > 0 {
				trend = fmt.Sprintf(" (↑ $%s, +%.1f%%)", format.WithCommas(int64(netWorthChange)), netWorthChangePercent)
			} else if netWorthChange < 0 {
				trend = fmt.Sprintf(" (↓ $%s, %.1f%%)", format.WithCommas(int64(-netWorthChange)), netWorthChangePercent)
			} else {
				trend = " (→ No change)"
			}

			currentNetWorth := format.Currency(int(netWorthSeries[len(netWorthSeries)-1]*100), "USD")
			fmt.Printf("\n🏆 Net Worth: %s%s\n", currentNetWorth, trend)

			// Use tight bounds for net worth graph that don't start from 0
			padding := variation * 0.05 // 5% padding on each side
			lowerBound := minNetWorth - padding
			upperBound := maxNetWorth + padding

			netWorthGraph := asciigraph.Plot(netWorthSeries,
				asciigraph.Height(8),
				asciigraph.Width(70),
				asciigraph.LowerBound(lowerBound),
				asciigraph.UpperBound(upperBound),
				asciigraph.SeriesColors(asciigraph.Green))
			fmt.Println(netWorthGraph)
		}
	}

	return nil
}

// displaySingleChart shows a chart for a single summed category
func displaySingleChart(title string, series []float64, color asciigraph.AnsiColor, days int) {
	if len(series) <= 1 {
		fmt.Printf("\n%s:\n  Not enough data points\n", title)
		return
	}

	// Check for meaningful variation
	minVal := series[0]
	maxVal := series[0]
	for _, val := range series {
		if val < minVal {
			minVal = val
		}
		if val > maxVal {
			maxVal = val
		}
	}

	variation := maxVal - minVal
	relativeVariation := 0.0
	if minVal != 0 {
		relativeVariation = variation / minVal * 100
	}

	if variation <= 10.0 && relativeVariation <= 0.1 {
		fmt.Printf("\n%s:\n  No significant variations detected in the last %d days\n", title, days)
		return
	}

	// Show trend summary
	change := series[len(series)-1] - series[0]
	changePercent := 0.0
	if series[0] != 0 {
		changePercent = (change / series[0]) * 100
	}

	var trend string
	if change > 0 {
		trend = fmt.Sprintf(" (↑ $%s, +%.1f%%)", format.WithCommas(int64(change)), changePercent)
	} else if change < 0 {
		trend = fmt.Sprintf(" (↓ $%s, %.1f%%)", format.WithCommas(int64(-change)), changePercent)
	} else {
		trend = " (→ No change)"
	}

	// Include current total in title
	currentTotal := format.Currency(int(series[len(series)-1]*100), "USD")
	fmt.Printf("\n%s: %s%s\n", title, currentTotal, trend)

	// Use tight bounds that don't start from 0
	padding := variation * 0.05 // 5% padding on each side
	lowerBound := minVal - padding
	upperBound := maxVal + padding

	graph := asciigraph.Plot(series,
		asciigraph.Height(8),
		asciigraph.Width(70),
		asciigraph.LowerBound(lowerBound),
		asciigraph.UpperBound(upperBound),
		asciigraph.SeriesColors(color))
	fmt.Println(graph)
}
