package simplefin

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// createTestHTTPClient creates an HTTP client that accepts self-signed certificates for testing
func createTestHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Allow self-signed certificates in tests
			},
		},
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("https://example.com", "user", "pass")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	if !client.IsConfigured() {
		t.Error("Client should be configured with provided credentials")
	}

	accessURL, username, password := client.GetCredentials()
	if accessURL != "https://example.com" || username != "user" || password != "pass" {
		t.Errorf("Credentials not stored correctly: got %s, %s, %s", accessURL, username, password)
	}
}

func TestExchangeToken(t *testing.T) {
	// Create a test server to simulate SimpleFIN Bridge
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/claim" {
			t.Errorf("Expected POST /claim, got %s %s", r.Method, r.URL.Path)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Return a mock access URL with credentials
		fmt.Fprintf(w, "https://testuser:testpass@%s/api", r.Host)
	}))
	defer server.Close()

	// Create setup token as base64 encoded URL pointing to our test server
	claimURL := fmt.Sprintf("%s/claim", server.URL)
	setupToken := base64.StdEncoding.EncodeToString([]byte(claimURL))

	client := &Client{httpClient: createTestHTTPClient()}
	accessURL, username, password, err := client.ExchangeToken(setupToken)
	if err != nil {
		t.Fatalf("ExchangeToken failed: %v", err)
	}

	expectedURL := fmt.Sprintf("https://%s/api", strings.TrimPrefix(server.URL, "https://"))
	if accessURL != expectedURL {
		t.Errorf("Expected access URL %s, got %s", expectedURL, accessURL)
	}

	if username != "testuser" || password != "testpass" {
		t.Errorf("Expected credentials testuser/testpass, got %s/%s", username, password)
	}
}

func TestNewClientFromToken(t *testing.T) {
	// Create a test server to simulate SimpleFIN Bridge
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "https://testuser:testpass@%s/api", r.Host)
	}))
	defer server.Close()

	claimURL := fmt.Sprintf("%s/claim", server.URL)
	setupToken := base64.StdEncoding.EncodeToString([]byte(claimURL))

	// For testing, we need to create a client that uses test HTTP client
	tempClient := &Client{httpClient: createTestHTTPClient()}
	accessURL, username, password, err := tempClient.ExchangeToken(setupToken)
	if err != nil {
		t.Fatalf("Token exchange failed: %v", err)
	}

	client := NewClient(accessURL, username, password)

	if !client.IsConfigured() {
		t.Error("Client should be configured after token exchange")
	}
}

func TestGetAccounts(t *testing.T) {
	// Create test data with the new API structure
	testResponse := AccountsResponse{
		Accounts: []Account{
			{
				ID:       "acc1",
				Name:     "Test Checking",
				Currency: "USD",
				Balance:  "1500.50", // Amount as string
				Org: Organization{
					ID:   "org1",
					Name: "Test Bank",
				},
				Transactions: []Transaction{
					{
						ID:          "txn1",
						Posted:      1609459200, // Unix timestamp for 2021-01-01T00:00:00Z
						Amount:      "-25.00",   // Amount as string
						Description: "Coffee Shop",
					},
				},
			},
		},
	}

	// Create a test server to simulate SimpleFIN API
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/accounts" {
			t.Errorf("Expected GET /accounts, got %s %s", r.Method, r.URL.Path)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Check basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "testuser" || password != "testpass" {
			t.Errorf("Expected basic auth testuser/testpass, got %s/%s", username, password)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testResponse)
	}))
	defer server.Close()

	// Create client with test server URL and test HTTP client
	accessURL, _ := url.Parse(server.URL)
	client := NewClient(accessURL.String(), "testuser", "testpass")
	client.SetHTTPClient(createTestHTTPClient()) // Override with test client

	accounts, err := client.GetAccounts()
	if err != nil {
		t.Fatalf("GetAccounts failed: %v", err)
	}

	if len(accounts.Accounts) != 1 {
		t.Errorf("Expected 1 account, got %d", len(accounts.Accounts))
	}

	account := accounts.Accounts[0]
	if account.ID != "acc1" || account.Name != "Test Checking" || account.Balance != "1500.50" {
		t.Errorf("Account data mismatch: %+v", account)
	}

	if account.Org.ID != "org1" || account.Org.Name != "Test Bank" {
		t.Errorf("Embedded organization data mismatch: %+v", account.Org)
	}

	if len(account.Transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(account.Transactions))
	}

	txn := account.Transactions[0]
	if txn.ID != "txn1" || txn.Amount != "-25.00" || txn.Description != "Coffee Shop" {
		t.Errorf("Transaction data mismatch: %+v", txn)
	}

	if txn.Posted != 1609459200 {
		t.Errorf("Transaction posted timestamp mismatch: expected 1609459200, got %d", txn.Posted)
	}
}

func TestGetAccountsNotConfigured(t *testing.T) {
	client := &Client{}
	client.SetHTTPClient(createTestHTTPClient())
	_, err := client.GetAccounts()
	if err == nil {
		t.Error("Expected error when client is not configured")
	}

	if !strings.Contains(err.Error(), "access URL not set") {
		t.Errorf("Expected 'access URL not set' error, got: %v", err)
	}
}

func TestExchangeTokenInvalidURL(t *testing.T) {
	client := &Client{}
	client.SetHTTPClient(createTestHTTPClient())
	_, _, _, err := client.ExchangeToken("not-a-url")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestExchangeTokenInvalidBase64(t *testing.T) {
	client := &Client{}
	client.SetHTTPClient(createTestHTTPClient())
	_, _, _, err := client.ExchangeToken("not-valid-base64!")
	if err == nil {
		t.Error("Expected error for invalid base64")
	}

	if !strings.Contains(err.Error(), "invalid setup token") {
		t.Errorf("Expected 'invalid setup token' error, got: %v", err)
	}
}

func TestParseAmountToCents(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123.45", 12345, false},
		{"0.99", 99, false},
		{"1000.00", 100000, false},
		{"-25.50", -2550, false},
		{"0", 0, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"123.456", 12346, false}, // Should round to nearest cent
	}

	for _, tc := range testCases {
		result, err := ParseAmountToCents(tc.input)

		if tc.hasError {
			if err == nil {
				t.Errorf("Expected error for input '%s', but got none", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
			}
			if result != tc.expected {
				t.Errorf("For input '%s', expected %d cents, got %d", tc.input, tc.expected, result)
			}
		}
	}
}

func TestUnixTimestampToISO(t *testing.T) {
	testCases := []struct {
		input    int64
		expected string
	}{
		{1609459200, "2021-01-01T00:00:00Z"}, // Jan 1, 2021 midnight UTC
		{0, ""},                              // Zero timestamp should return empty string
		{1640995200, "2022-01-01T00:00:00Z"}, // Jan 1, 2022 midnight UTC
	}

	for _, tc := range testCases {
		result := UnixTimestampToISO(tc.input)
		if result != tc.expected {
			t.Errorf("For input %d, expected '%s', got '%s'", tc.input, tc.expected, result)
		}
	}
}
