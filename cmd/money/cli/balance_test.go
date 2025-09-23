package cli

import (
	"testing"
)

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		cents    int
		currency string
		expected string
	}{
		{
			name:     "positive USD amount",
			cents:    123456,
			currency: "USD",
			expected: "$1,234.56",
		},
		{
			name:     "negative USD amount",
			cents:    -123456,
			currency: "USD",
			expected: "-$1,234.56",
		},
		{
			name:     "zero amount",
			cents:    0,
			currency: "USD",
			expected: "$0.00",
		},
		{
			name:     "large amount",
			cents:    123456789,
			currency: "USD",
			expected: "$1,234,567.89",
		},
		{
			name:     "EUR currency",
			cents:    50000,
			currency: "EUR",
			expected: "€500.00",
		},
		{
			name:     "unknown currency",
			cents:    10000,
			currency: "XYZ",
			expected: "XYZ 100.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCurrency(tt.cents, tt.currency)
			if result != tt.expected {
				t.Errorf("formatCurrency(%d, %s) = %s; want %s", tt.cents, tt.currency, result, tt.expected)
			}
		})
	}
}

func TestFormatWithCommas(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "small number",
			input:    123,
			expected: "123",
		},
		{
			name:     "thousands",
			input:    1234,
			expected: "1,234",
		},
		{
			name:     "millions",
			input:    1234567,
			expected: "1,234,567",
		},
		{
			name:     "billions",
			input:    1234567890,
			expected: "1,234,567,890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWithCommas(tt.input)
			if result != tt.expected {
				t.Errorf("formatWithCommas(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetCurrencySymbol(t *testing.T) {
	tests := []struct {
		currency string
		expected string
	}{
		{"USD", "$"},
		{"usd", "$"},
		{"EUR", "€"},
		{"GBP", "£"},
		{"JPY", "¥"},
		{"CAD", "C$"},
		{"AUD", "A$"},
		{"XYZ", "XYZ "},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			result := getCurrencySymbol(tt.currency)
			if result != tt.expected {
				t.Errorf("getCurrencySymbol(%s) = %s; want %s", tt.currency, result, tt.expected)
			}
		})
	}
}

func TestGetTypeIcon(t *testing.T) {
	tests := []struct {
		accountType string
		expected    string
	}{
		{"checking", "💰"},
		{"savings", "🏦"},
		{"credit", "💳"},
		{"investment", "📊"},
		{"loan", "🏠"},
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
