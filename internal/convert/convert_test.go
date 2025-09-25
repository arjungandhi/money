package convert

import (
	"testing"

	"github.com/arjungandhi/money/pkg/database"
)

func TestToLLMTransactionData(t *testing.T) {
	dbTransactions := []database.Transaction{
		{
			ID:          "tx1",
			AccountID:   "acc1",
			Posted:      "2023-01-15T10:00:00Z",
			Amount:      -1500,
			Description: "Coffee Shop",
			Pending:     false,
		},
		{
			ID:          "tx2",
			AccountID:   "acc2",
			Posted:      "2023-01-16T14:30:00Z",
			Amount:      5000,
			Description: "Salary Deposit",
			Pending:     true,
		},
	}

	llmTransactions := ToLLMTransactionData(dbTransactions)

	if len(llmTransactions) != len(dbTransactions) {
		t.Errorf("Expected %d transactions, got %d", len(dbTransactions), len(llmTransactions))
	}

	if llmTransactions[0].ID != dbTransactions[0].ID {
		t.Errorf("Expected ID '%s', got '%s'", dbTransactions[0].ID, llmTransactions[0].ID)
	}

	if llmTransactions[0].Amount != dbTransactions[0].Amount {
		t.Errorf("Expected amount %d, got %d", dbTransactions[0].Amount, llmTransactions[0].Amount)
	}

	if llmTransactions[0].Pending != dbTransactions[0].Pending {
		t.Errorf("Expected pending %t, got %t", dbTransactions[0].Pending, llmTransactions[0].Pending)
	}
}

func TestToLLMAccountData(t *testing.T) {
	nickname := "My Checking"
	accountType := "checking"
	dbAccounts := []database.Account{
		{
			ID:          "acc1",
			Name:        "Checking Account",
			Nickname:    &nickname,
			AccountType: &accountType,
		},
		{
			ID:   "acc2",
			Name: "Savings Account",
		},
	}

	llmAccounts := ToLLMAccountData(dbAccounts)

	if len(llmAccounts) != len(dbAccounts) {
		t.Errorf("Expected %d accounts, got %d", len(dbAccounts), len(llmAccounts))
	}

	if llmAccounts[0].ID != dbAccounts[0].ID {
		t.Errorf("Expected ID '%s', got '%s'", dbAccounts[0].ID, llmAccounts[0].ID)
	}

	if llmAccounts[0].Nickname != "My Checking" {
		t.Errorf("Expected nickname 'My Checking', got '%s'", llmAccounts[0].Nickname)
	}

	if llmAccounts[0].AccountType != "checking" {
		t.Errorf("Expected account type 'checking', got '%s'", llmAccounts[0].AccountType)
	}

	if llmAccounts[1].Nickname != "" {
		t.Errorf("Expected empty nickname, got '%s'", llmAccounts[1].Nickname)
	}

	if llmAccounts[1].AccountType != "" {
		t.Errorf("Expected empty account type, got '%s'", llmAccounts[1].AccountType)
	}
}