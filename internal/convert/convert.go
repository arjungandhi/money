package convert

import (
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/llm"
)

// ToLLMTransactionData converts database transactions to LLM transaction format
func ToLLMTransactionData(transactions []database.Transaction) []llm.TransactionData {
	llmTransactions := make([]llm.TransactionData, len(transactions))
	for i, tx := range transactions {
		llmTransactions[i] = llm.TransactionData{
			ID:          tx.ID,
			AccountID:   tx.AccountID,
			Posted:      tx.Posted,
			Amount:      tx.Amount,
			Description: tx.Description,
			Pending:     tx.Pending,
		}
	}
	return llmTransactions
}

// ToLLMAccountData converts database accounts to LLM account format
func ToLLMAccountData(accounts []database.Account) []llm.AccountData {
	llmAccounts := make([]llm.AccountData, len(accounts))
	for i, acc := range accounts {
		nickname := ""
		if acc.Nickname != nil {
			nickname = *acc.Nickname
		}
		accountType := ""
		if acc.AccountType != nil {
			accountType = *acc.AccountType
		}
		llmAccounts[i] = llm.AccountData{
			ID:          acc.ID,
			Name:        acc.Name,
			Nickname:    nickname,
			AccountType: accountType,
		}
	}
	return llmAccounts
}

// ToCategorizedExamples converts database transactions with categories to LLM examples
func ToCategorizedExamples(transactions []database.Transaction, db *database.DB) ([]llm.CategorizedExample, error) {
	examples := make([]llm.CategorizedExample, 0, len(transactions))

	for _, tx := range transactions {
		if tx.CategoryID == nil {
			continue // Skip uncategorized transactions
		}

		// Get category name
		category, err := db.GetCategoryByID(*tx.CategoryID)
		if err != nil {
			continue // Skip if category not found
		}

		examples = append(examples, llm.CategorizedExample{
			Description: tx.Description,
			Amount:      tx.Amount,
			Category:    category.Name,
		})
	}

	return examples, nil
}