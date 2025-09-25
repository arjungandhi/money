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

var Income = &Z.Cmd{
	Name:     "income",
	Summary:  "Show breakdown of income by category for time period",
	Usage:    "[--start YYYY-MM-DD] [--end YYYY-MM-DD] [--month YYYY-MM]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Parse date arguments (reuse same function from costs)
		startDate, endDate := format.DateRange(args)

		// Get categorized transactions (exclude transfers)
		categoryTransactions, err := db.GetTransactionsByCategory(startDate, endDate, true)
		if err != nil {
			return fmt.Errorf("failed to get categorized transactions: %w", err)
		}

		// Calculate income (positive amounts only)
		categoryIncome := make(map[string]int64)
		totalIncome := int64(0)

		for categoryName, transactions := range categoryTransactions {
			categoryTotal := int64(0)
			for _, t := range transactions {
				// Only include positive amounts (income)
				if t.Amount > 0 {
					categoryTotal += int64(t.Amount)
				}
			}
			if categoryTotal > 0 {
				categoryIncome[categoryName] = categoryTotal
				totalIncome += categoryTotal
			}
		}

		// Display results
		if len(categoryIncome) == 0 {
			fmt.Printf("No income found for period %s to %s\n", startDate, endDate)
			return nil
		}

		fmt.Printf("\n💰 Income Breakdown (%s to %s)\n", format.DateForDisplay(startDate), format.DateForDisplay(endDate))
		fmt.Println(strings.Repeat("=", 60))

		// Sort categories by income amount (descending)
		type categoryData struct {
			name   string
			income int64
		}

		var sortedCategories []categoryData
		for name, income := range categoryIncome {
			sortedCategories = append(sortedCategories, categoryData{name: name, income: income})
		}

		sort.Slice(sortedCategories, func(i, j int) bool {
			return sortedCategories[i].income > sortedCategories[j].income
		})

		// Create tabwriter for proper alignment
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "Category\tAmount\tPercentage\n")
		fmt.Fprintf(w, "────────\t──────\t──────────\n")

		for _, cat := range sortedCategories {
			percentage := float64(cat.income) / float64(totalIncome) * 100
			fmt.Fprintf(w, "%s\t%s\t%.1f%%\n",
				cat.name,
				format.Currency(int(cat.income), "USD"),
				percentage)
		}

		w.Flush()

		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("💵 Total Income: %s\n", format.Currency(int(totalIncome), "USD"))
		fmt.Println(strings.Repeat("=", 60))

		return nil
	},
}

