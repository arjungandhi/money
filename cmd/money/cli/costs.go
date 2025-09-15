package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
)

var Costs = &Z.Cmd{
	Name:     "costs",
	Summary:  "Show breakdown of costs by category for time period",
	Usage:    "[--start YYYY-MM-DD] [--end YYYY-MM-DD] [--month YYYY-MM]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Parse date arguments
		startDate, endDate := parseDateRange(args)

		// Get categorized transactions (exclude transfers)
		categoryTransactions, err := db.GetTransactionsByCategory(startDate, endDate, true)
		if err != nil {
			return fmt.Errorf("failed to get categorized transactions: %w", err)
		}

		// Calculate costs (negative amounts only)
		categoryCosts := make(map[string]int64)
		totalCosts := int64(0)

		for categoryName, transactions := range categoryTransactions {
			categoryTotal := int64(0)
			for _, t := range transactions {
				// Only include negative amounts (costs/expenses)
				if t.Amount < 0 {
					categoryTotal += int64(-t.Amount) // Make positive for display
				}
			}
			if categoryTotal > 0 {
				categoryCosts[categoryName] = categoryTotal
				totalCosts += categoryTotal
			}
		}

		// Display results
		if len(categoryCosts) == 0 {
			fmt.Printf("No costs found for period %s to %s\n", startDate, endDate)
			return nil
		}

		fmt.Printf("\n💸 Costs Breakdown (%s to %s)\n", formatDateForDisplay(startDate), formatDateForDisplay(endDate))
		fmt.Println(strings.Repeat("=", 60))

		// Sort categories by cost amount (descending)
		type categoryData struct {
			name string
			cost int64
		}

		var sortedCategories []categoryData
		for name, cost := range categoryCosts {
			sortedCategories = append(sortedCategories, categoryData{name: name, cost: cost})
		}

		sort.Slice(sortedCategories, func(i, j int) bool {
			return sortedCategories[i].cost > sortedCategories[j].cost
		})

		// Create tabwriter for proper alignment
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Category\tAmount\tPercentage\n")
		fmt.Fprintf(w, "────────\t──────\t──────────\n")

		for _, cat := range sortedCategories {
			percentage := float64(cat.cost) / float64(totalCosts) * 100
			fmt.Fprintf(w, "%s\t%s\t%.1f%%\n",
				cat.name,
				formatCurrency(int(cat.cost), "USD"),
				percentage)
		}

		w.Flush()

		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("💰 Total Costs: %s\n", formatCurrency(int(totalCosts), "USD"))
		fmt.Println(strings.Repeat("=", 60))

		return nil
	},
}

// parseDateRange parses command line arguments for date range
func parseDateRange(args []string) (startDate, endDate string) {
	// Default to current month
	now := time.Now()

	// Check for explicit start/end dates
	for i, arg := range args {
		if (arg == "--start" || arg == "-s") && i+1 < len(args) {
			startDate = args[i+1]
		} else if (arg == "--end" || arg == "-e") && i+1 < len(args) {
			endDate = args[i+1]
		} else if (arg == "--month" || arg == "-m") && i+1 < len(args) {
			// Parse month in YYYY-MM format
			monthStr := args[i+1]
			if monthTime, err := time.Parse("2006-01", monthStr); err == nil {
				startDate = monthTime.Format("2006-01-02")
				endDate = monthTime.AddDate(0, 1, -1).Format("2006-01-02") // Last day of month
			}
		}
	}

	// If no dates specified, use current month
	if startDate == "" && endDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Format("2006-01-02")
	} else if startDate != "" && endDate == "" {
		// If only start date provided, use rest of current month
		endDate = now.Format("2006-01-02")
	} else if startDate == "" && endDate != "" {
		// If only end date provided, use beginning of month
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}

	return startDate, endDate
}

// formatDateForDisplay formats a date string for user-friendly display
func formatDateForDisplay(dateStr string) string {
	if dateStr == "" {
		return "unknown"
	}

	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.Format("Jan 2, 2006")
	}

	return dateStr
}
