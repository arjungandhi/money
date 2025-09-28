package format

import "testing"

func TestCurrency(t *testing.T) {
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
			result := Currency(tt.cents, tt.currency)
			if result != tt.expected {
				t.Errorf("Currency(%d, %s) = %s; want %s", tt.cents, tt.currency, result, tt.expected)
			}
		})
	}
}

func TestWithCommas(t *testing.T) {
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
			result := WithCommas(tt.input)
			if result != tt.expected {
				t.Errorf("WithCommas(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}
