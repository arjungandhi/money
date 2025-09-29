package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/format"
	"github.com/arjungandhi/money/pkg/table"
)

var Budget = &Z.Cmd{
	Name:     "budget",
	Aliases:  []string{"b"},
	Summary:  "Show spending by category with budget analysis",
	Usage:    "[--month|-m YYYY-MM] [--days|-d <number>]",
	Commands: []*Z.Cmd{help.Cmd},
	Description: `
Show spending breakdown by category for budget analysis.

By default shows current month. Use --month to specify a different month,
or --days to show spending for the last N days.

Examples:
  money budget                    # Current month
  money budget -m 2023-11         # November 2023
  money budget --days 30          # Last 30 days
`,
	Call: func(cmd *Z.Cmd, args ...string) error {
		var startDate, endDate time.Time
		var periodLabel string

		// Parse arguments
		useMonth := true
		targetMonth := time.Now().Format("2006-01")
		days := 0

		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--month", "-m":
				if i+1 < len(args) {
					targetMonth = args[i+1]
					i++
				}
			case "--days", "-d":
				if i+1 < len(args) {
					if parsedDays, err := strconv.Atoi(args[i+1]); err == nil && parsedDays > 0 {
						days = parsedDays
						useMonth = false
						i++
					}
				}
			}
		}

		if useMonth {
			// Parse month (YYYY-MM)
			monthTime, err := time.Parse("2006-01", targetMonth)
			if err != nil {
				return fmt.Errorf("invalid month format. Use YYYY-MM: %w", err)
			}

			// Set start and end dates for the month
			startDate = time.Date(monthTime.Year(), monthTime.Month(), 1, 0, 0, 0, 0, time.UTC)
			endDate = startDate.AddDate(0, 1, 0).Add(-time.Second) // Last second of the month
			periodLabel = monthTime.Format("January 2006")
		} else {
			// Last N days
			endDate = time.Now()
			startDate = endDate.AddDate(0, 0, -days)
			periodLabel = fmt.Sprintf("Last %d days", days)
		}

		db, err := database.New()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Get transactions for the period
		startDateStr := startDate.Format("2006-01-02")
		endDateStr := endDate.Format("2006-01-02")

		transactions, err := db.GetTransactions("", startDateStr, endDateStr)
		if err != nil {
			return fmt.Errorf("failed to get transactions: %w", err)
		}

		if len(transactions) == 0 {
			fmt.Printf("No transactions found for %s.\n", periodLabel)
			return nil
		}

		// Get all categories for lookup
		categories, err := db.GetCategories()
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		categoryMap := make(map[int]database.Category)
		for _, cat := range categories {
			categoryMap[cat.ID] = cat
		}

		// Calculate spending by category
		type CategorySpending struct {
			Name      string
			Amount    int64 // Total spending (positive for expenses, negative for income)
			Count     int
			IsInternal bool
		}

		categorySpending := make(map[string]*CategorySpending)
		uncategorizedSpending := &CategorySpending{Name: "Uncategorized", Amount: 0, Count: 0}

		var totalIncome, totalExpenses int64

		for _, txn := range transactions {
			if txn.CategoryID != nil {
				category := categoryMap[*txn.CategoryID]
				categoryName := category.Name
				if category.IsInternal {
					categoryName += " (internal)"
				}

				if _, exists := categorySpending[categoryName]; !exists {
					categorySpending[categoryName] = &CategorySpending{
						Name:      categoryName,
						Amount:    0,
						Count:     0,
						IsInternal: category.IsInternal,
					}
				}

				categorySpending[categoryName].Amount += int64(txn.Amount)
				categorySpending[categoryName].Count++

				// Don't include internal transactions in income/expense totals
				if !category.IsInternal {
					if txn.Amount < 0 {
						totalExpenses += int64(-txn.Amount)
					} else {
						totalIncome += int64(txn.Amount)
					}
				}
			} else {
				uncategorizedSpending.Amount += int64(txn.Amount)
				uncategorizedSpending.Count++

				// Include uncategorized in totals
				if txn.Amount < 0 {
					totalExpenses += int64(-txn.Amount)
				} else {
					totalIncome += int64(txn.Amount)
				}
			}
		}

		// Add uncategorized if it has transactions
		if uncategorizedSpending.Count > 0 {
			categorySpending["Uncategorized"] = uncategorizedSpending
		}

		// Convert to slice and sort by amount (largest expenses first)
		var spendingList []*CategorySpending
		for _, spending := range categorySpending {
			spendingList = append(spendingList, spending)
		}

		sort.Slice(spendingList, func(i, j int) bool {
			// Internal categories go to the end
			if spendingList[i].IsInternal != spendingList[j].IsInternal {
				return !spendingList[i].IsInternal
			}
			// Sort by absolute amount (largest first)
			absI := spendingList[i].Amount
			if absI < 0 {
				absI = -absI
			}
			absJ := spendingList[j].Amount
			if absJ < 0 {
				absJ = -absJ
			}
			return absI > absJ
		})

		// Display results
		fmt.Printf("💰 Budget Analysis - %s\n", periodLabel)
		fmt.Printf("📊 Period: %s to %s\n", startDateStr, endDateStr)
		fmt.Println()

		// Summary
		netCashFlow := totalIncome - totalExpenses
		fmt.Printf("💵 Total Income:  %s\n", format.Currency(int(totalIncome), "USD"))
		fmt.Printf("💸 Total Expenses: %s\n", format.Currency(int(totalExpenses), "USD"))
		if netCashFlow >= 0 {
			fmt.Printf("✅ Net Cash Flow: +%s\n", format.Currency(int(netCashFlow), "USD"))
		} else {
			fmt.Printf("⚠️  Net Cash Flow: %s\n", format.Currency(int(netCashFlow), "USD"))
		}
		fmt.Println()

		// Category breakdown table
		config := table.DefaultConfig()
		config.Title = "Spending by Category"
		config.MaxColumnWidth = 30

		t := table.NewWithConfig(config, "Category", "Amount", "Transactions", "Avg per Transaction")

		for _, spending := range spendingList {
			var amountStr string
			if spending.Amount < 0 {
				// Expense (negative amount in DB)
				amountStr = format.Currency(int(-spending.Amount), "USD")
			} else {
				// Income (positive amount in DB)
				amountStr = "+" + format.Currency(int(spending.Amount), "USD")
			}

			avgAmount := float64(spending.Amount) / float64(spending.Count)
			var avgStr string
			if avgAmount < 0 {
				avgStr = format.Currency(int(-avgAmount), "USD")
			} else {
				avgStr = "+" + format.Currency(int(avgAmount), "USD")
			}

			categoryName := spending.Name
			if spending.Name == "Uncategorized" {
				categoryName = "❓ " + spending.Name
			} else if spending.IsInternal {
				categoryName = "🔄 " + strings.TrimSuffix(spending.Name, " (internal)")
			} else if spending.Amount < 0 {
				categoryName = "💸 " + spending.Name
			} else {
				categoryName = "💰 " + spending.Name
			}

			t.AddRow(
				categoryName,
				amountStr,
				fmt.Sprintf("%d", spending.Count),
				avgStr,
			)
		}

		if err := t.Render(); err != nil {
			return fmt.Errorf("failed to render budget table: %w", err)
		}

		// Tips
		fmt.Println("\n💡 Tips:")
		fmt.Println("   • Use 'money transactions categorize auto' to categorize uncategorized transactions")
		fmt.Println("   • Use 'money categories list' to see all available categories")
		fmt.Println("   • Use 'money transactions list --start YYYY-MM-DD --end YYYY-MM-DD' to see detailed transactions")

		return nil
	},
}
