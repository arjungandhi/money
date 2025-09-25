package cli

import (
	"testing"
)


func TestGetTypeIcon(t *testing.T) {
	tests := []struct {
		accountType string
		expected    string
	}{
		{"checking", "💰"},
		{"savings", "🏦"},
		{"credit", "💳"},
		{"investment", "📊"},
		{"loan", "💸"},
		{"other", "💼"},
		{"unset", "❓"},
		{"unknown", "📋"},
	}

	for _, tt := range tests {
		t.Run(tt.accountType, func(t *testing.T) {
			result := getTypeIcon(tt.accountType)
			if result != tt.expected {
				t.Errorf("getTypeIcon(%s) = %s; want %s", tt.accountType, result, tt.expected)
			}
		})
	}
}

func TestGetTypeDisplayName(t *testing.T) {
	tests := []struct {
		accountType string
		expected    string
	}{
		{"checking", "Checking Accounts"},
		{"savings", "Savings Accounts"},
		{"credit", "Credit Accounts"},
		{"investment", "Investment Accounts"},
		{"loan", "Loan Accounts"},
		{"other", "Other Accounts"},
		{"unset", "Unset Accounts"},
		{"custom", "Custom Accounts"},
	}

	for _, tt := range tests {
		t.Run(tt.accountType, func(t *testing.T) {
			result := getTypeDisplayName(tt.accountType)
			if result != tt.expected {
				t.Errorf("getTypeDisplayName(%s) = %s; want %s", tt.accountType, result, tt.expected)
			}
		})
	}
}
