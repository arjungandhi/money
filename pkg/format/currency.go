package format

import (
	"fmt"
	"strings"
)

func Currency(cents int, currency string) string {
	symbol := currencySymbol(currency)
	var wholePart int64
	var decimalPart int
	var negative bool

	if cents < 0 {
		negative = true
		cents = -cents
	}

	wholePart = int64(cents / 100)
	decimalPart = cents % 100
	wholeStr := withCommas(wholePart)
	if negative {
		return fmt.Sprintf("-%s%s.%02d", symbol, wholeStr, decimalPart)
	} else {
		return fmt.Sprintf("%s%s.%02d", symbol, wholeStr, decimalPart)
	}
}

func WithCommas(n int64) string {
	return withCommas(n)
}

func withCommas(n int64) string {
	if n == 0 {
		return "0"
	}

	str := fmt.Sprintf("%d", n)
	var parts []string
	for i := len(str); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{str[start:i]}, parts...)
	}

	return strings.Join(parts, ",")
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "JPY":
		return "¥"
	case "CAD":
		return "C$"
	case "AUD":
		return "A$"
	default:
		return currency + " "
	}
}
