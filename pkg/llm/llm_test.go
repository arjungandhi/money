package llm

import (
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Error("NewClient should return a non-nil client")
	}

	if client.promptCommand == "" {
		t.Error("Client should have a default command")
	}
}

func TestBuildTransferIdentificationPrompt(t *testing.T) {
	transactions := []TransactionData{
		{ID: "tx1", AccountID: "acc1", Description: "Transfer to savings", Amount: -5000},
		{ID: "tx2", AccountID: "acc2", Description: "Transfer from checking", Amount: 5000},
	}

	accounts := []AccountData{
		{ID: "acc1", Name: "Checking Account", AccountType: "checking"},
		{ID: "acc2", Name: "Savings Account", AccountType: "savings"},
	}

	prompt := buildTransferIdentificationPrompt(transactions, accounts)

	if prompt == "" {
		t.Error("buildTransferIdentificationPrompt should return non-empty prompt")
	}

	expectedElements := []string{"transactions", "accounts", "transfer", "JSON"}
	for _, element := range expectedElements {
		if !containsIgnoreCase(prompt, element) {
			t.Errorf("Prompt should contain '%s'", element)
		}
	}
}

func TestBuildCategorizationPrompt(t *testing.T) {
	transactions := []TransactionData{
		{ID: "tx1", Description: "Starbucks Coffee", Amount: -500},
		{ID: "tx2", Description: "Grocery Store", Amount: -5000},
	}

	categories := []string{"Dining Out", "Groceries", "Transportation"}

	prompt := buildCategorizationPrompt(transactions, categories)

	if prompt == "" {
		t.Error("buildCategorizationPrompt should return non-empty prompt")
	}

	for _, category := range categories {
		if !containsIgnoreCase(prompt, category) {
			t.Errorf("Prompt should contain category '%s'", category)
		}
	}
}

// Helper function to check if string contains substring (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}