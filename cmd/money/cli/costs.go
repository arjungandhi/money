package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/format"
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
		startDate, endDate := format.DateRange(args)

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

		fmt.Printf("\n💸 Costs Breakdown (%s to %s)\n", format.DateForDisplay(startDate), format.DateForDisplay(endDate))
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
				format.Currency(int(cat.cost), "USD"),
				percentage)
		}

		w.Flush()

		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("💰 Total Costs: %s\n", format.Currency(int(totalCosts), "USD"))
		fmt.Println(strings.Repeat("=", 60))

		return nil
	},
}

