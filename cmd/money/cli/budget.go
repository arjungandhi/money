package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/internal/dbutil"
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/format"
	"github.com/arjungandhi/money/pkg/table"
)

var Budget = &Z.Cmd{
	Name:     "budget",
	Summary:  "Show comprehensive budget view with income, expenses, and net cash flow by category",
	Usage:    "[--days|-d <number>] [--income-only] [--expenses-only] [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--month YYYY-MM]",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		return dbutil.WithDatabase(func(db *database.DB) error {
			// Parse flags
			var startDate, endDate string
			var incomeOnly, expensesOnly bool
			days := 0

			for i, arg := range args {
				switch arg {
				case "--income-only":
					incomeOnly = true
				case "--expenses-only":
					expensesOnly = true
				case "--days", "-d":
					if i+1 < len(args) {
						if parsedDays, err := strconv.Atoi(args[i+1]); err == nil && parsedDays > 0 {
							days = parsedDays
						}
					}
				case "--start", "-s":
					if i+1 < len(args) {
						startDate = args[i+1]
					}
				case "--end", "-e":
					if i+1 < len(args) {
						endDate = args[i+1]
					}
				case "--month", "-m":
					if i+1 < len(args) {
						monthStr := args[i+1]
						if monthTime, err := time.Parse("2006-01", monthStr); err == nil {
							startDate = monthTime.Format("2006-01-02")
							endDate = monthTime.AddDate(0, 1, -1).Format("2006-01-02")
						}
					}
				}
			}

			// Handle --days flag (overrides other date options)
			if days > 0 {
				now := time.Now()
				endDate = now.Format("2006-01-02")
				startDate = now.AddDate(0, 0, -days).Format("2006-01-02")
			} else if startDate == "" && endDate == "" {
				// Default to current month if no date range specified
				now := time.Now()
				startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
				endDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Format("2006-01-02")
			} else if startDate != "" && endDate == "" {
				endDate = time.Now().Format("2006-01-02")
			} else if startDate == "" && endDate != "" {
				now := time.Now()
				startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
			}

			// Get categorized transactions (exclude internal categories)
			categoryTransactions, err := db.GetTransactionsByCategory(startDate, endDate, true)
			if err != nil {
				return fmt.Errorf("failed to get categorized transactions: %w", err)
			}

			// Calculate income and expenses by category
			categoryIncome := make(map[string]int64)
			categoryExpenses := make(map[string]int64)
			totalIncome := int64(0)
			totalExpenses := int64(0)

			for categoryName, transactions := range categoryTransactions {
				incomeTotal := int64(0)
				expenseTotal := int64(0)

				for _, t := range transactions {
					if t.Amount > 0 {
						// Positive amounts are income
						incomeTotal += int64(t.Amount)
					} else if t.Amount < 0 {
						// Negative amounts are expenses (make positive for display)
						expenseTotal += int64(-t.Amount)
					}
				}

				if incomeTotal > 0 {
					categoryIncome[categoryName] = incomeTotal
					totalIncome += incomeTotal
				}
				if expenseTotal > 0 {
					categoryExpenses[categoryName] = expenseTotal
					totalExpenses += expenseTotal
				}
			}

			// Display results
			if len(categoryIncome) == 0 && len(categoryExpenses) == 0 {
				fmt.Printf("No transactions found for period %s to %s\n", startDate, endDate)
				return nil
			}

			periodLabel := generatePeriodLabel(startDate, endDate, days)

			// Show Income section (unless expenses-only)
			if !expensesOnly && len(categoryIncome) > 0 {
				displayBudgetSection("💰 Income", categoryIncome, totalIncome, periodLabel)
			}

			// Show Expenses section (unless income-only)
			if !incomeOnly && len(categoryExpenses) > 0 {
				displayBudgetSection("💸 Expenses", categoryExpenses, totalExpenses, periodLabel)
			}

			// Show Net Cash Flow summary (unless showing only one section)
			if !incomeOnly && !expensesOnly {
				netCashFlow := totalIncome - totalExpenses

				var flowIcon string
				var flowLabel string
				var cashFlowDisplay string
				if netCashFlow > 0 {
					flowIcon = "📈"
					flowLabel = "Net Cash Flow"
					green := color.New(color.FgGreen).SprintFunc()
					cashFlowDisplay = green(fmt.Sprintf("+%s", format.Currency(int(netCashFlow), "USD")))
				} else if netCashFlow < 0 {
					flowIcon = "📉"
					flowLabel = "Net Cash Flow"
					red := color.New(color.FgRed).SprintFunc()
					cashFlowDisplay = red(format.Currency(int(netCashFlow), "USD"))
				} else {
					flowIcon = "⚖️"
					flowLabel = "Net Cash Flow"
					cashFlowDisplay = format.Currency(int(netCashFlow), "USD")
				}

				config := table.DefaultConfig()
				config.Title = fmt.Sprintf("📊 Net Cash Flow (%s)", periodLabel)
				config.ShowHeaders = false

				cashFlowTable := table.NewWithConfig(config, "", "")
				cashFlowTable.AddRow("Total Income", format.Currency(int(totalIncome), "USD"))
				cashFlowTable.AddRow("Total Expenses", format.Currency(int(totalExpenses), "USD"))
				cashFlowTable.AddRow("────────────", "──────────────")
				cashFlowTable.AddRow(fmt.Sprintf("%s %s", flowIcon, flowLabel), cashFlowDisplay)

				if err := cashFlowTable.Render(); err != nil {
					fmt.Printf("Error rendering cash flow table: %v\n", err)
					return err
				}
			}

			return nil
		})
	},
}

func displayBudgetSection(title string, categoryAmounts map[string]int64, total int64, periodLabel string) {
	// Sort categories by amount (descending)
	type categoryData struct {
		name   string
		amount int64
	}

	var sortedCategories []categoryData
	for name, amount := range categoryAmounts {
		sortedCategories = append(sortedCategories, categoryData{name: name, amount: amount})
	}

	sort.Slice(sortedCategories, func(i, j int) bool {
		return sortedCategories[i].amount > sortedCategories[j].amount
	})

	// Create budget section table
	config := table.DefaultConfig()
	config.Title = fmt.Sprintf("%s (%s)", title, periodLabel)
	config.MaxColumnWidth = 30

	budgetTable := table.NewWithConfig(config, "Category", "Amount", "Percentage")

	for _, cat := range sortedCategories {
		percentage := float64(cat.amount) / float64(total) * 100
		budgetTable.AddRow(
			cat.name,
			format.Currency(int(cat.amount), "USD"),
			fmt.Sprintf("%.1f%%", percentage),
		)
	}

	if err := budgetTable.Render(); err != nil {
		fmt.Printf("Error rendering budget table: %v\n", err)
		return
	}

	fmt.Printf("💵 Total: %s\n", format.Currency(int(total), "USD"))
	fmt.Println(strings.Repeat("=", 60))
}

func generatePeriodLabel(startDate, endDate string, days int) string {
	if days > 0 {
		return fmt.Sprintf("Last %d Days", days)
	}
	return fmt.Sprintf("%s to %s", format.DateForDisplay(startDate), format.DateForDisplay(endDate))
}
