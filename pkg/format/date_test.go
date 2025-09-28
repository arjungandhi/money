package format

import (
	"testing"
)

func TestDateRange(t *testing.T) {
	// Test current month default
	startDate, endDate := DateRange([]string{})
	if startDate == "" || endDate == "" {
		t.Error("DateRange should return current month dates when no args provided")
	}

	// Test with start and end dates
	args := []string{"--start", "2023-01-15", "--end", "2023-01-31"}
	startDate, endDate = DateRange(args)
	if startDate != "2023-01-15" {
		t.Errorf("Expected start date '2023-01-15', got '%s'", startDate)
	}
	if endDate != "2023-01-31" {
		t.Errorf("Expected end date '2023-01-31', got '%s'", endDate)
	}

	// Test with month flag
	args = []string{"--month", "2023-03"}
	startDate, endDate = DateRange(args)
	if startDate != "2023-03-01" {
		t.Errorf("Expected start date '2023-03-01', got '%s'", startDate)
	}
	if endDate != "2023-03-31" {
		t.Errorf("Expected end date '2023-03-31', got '%s'", endDate)
	}

	// Test with short flags
	args = []string{"-s", "2023-02-01", "-e", "2023-02-28"}
	startDate, endDate = DateRange(args)
	if startDate != "2023-02-01" {
		t.Errorf("Expected start date '2023-02-01', got '%s'", startDate)
	}
	if endDate != "2023-02-28" {
		t.Errorf("Expected end date '2023-02-28', got '%s'", endDate)
	}
}

func TestDateForDisplay(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "2023-01-15",
			expected: "January 15, 2023",
		},
		{
			input:    "2023-12-01",
			expected: "December 1, 2023",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "invalid-date",
			expected: "invalid-date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := DateForDisplay(tt.input)
			if result != tt.expected {
				t.Errorf("DateForDisplay(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}
