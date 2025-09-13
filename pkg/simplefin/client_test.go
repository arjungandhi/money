package simplefin

import (
	"crypto/tls"
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

	// Create setup token URL pointing to our test server
	setupToken := fmt.Sprintf("%s/claim/test-token-123", server.URL)

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

	setupToken := fmt.Sprintf("%s/claim/test-token-123", server.URL)

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
	// Create test data
	testResponse := AccountsResponse{
		Accounts: []Account{
			{
				ID:       "acc1",
				OrgID:    "org1",
				Name:     "Test Checking",
				Currency: "USD",
				Balance:  150050, // $1500.50 in cents
				Transactions: []Transaction{
					{
						ID:          "txn1",
						Posted:      "2023-12-01T10:00:00Z",
						Amount:      -2500, // -$25.00 in cents
						Description: "Coffee Shop",
					},
				},
			},
		},
		Organizations: []Organization{
			{
				ID:   "org1",
				Name: "Test Bank",
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

	if len(accounts.Organizations) != 1 {
		t.Errorf("Expected 1 organization, got %d", len(accounts.Organizations))
	}

	account := accounts.Accounts[0]
	if account.ID != "acc1" || account.Name != "Test Checking" || account.Balance != 150050 {
		t.Errorf("Account data mismatch: %+v", account)
	}

	if len(account.Transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(account.Transactions))
	}

	txn := account.Transactions[0]
	if txn.ID != "txn1" || txn.Amount != -2500 || txn.Description != "Coffee Shop" {
		t.Errorf("Transaction data mismatch: %+v", txn)
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

func TestExchangeTokenInvalidFormat(t *testing.T) {
	client := &Client{}
	client.SetHTTPClient(createTestHTTPClient())
	_, _, _, err := client.ExchangeToken("https://example.com/invalid/path")
	if err == nil {
		t.Error("Expected error for invalid token format")
	}

	if !strings.Contains(err.Error(), "invalid setup token format") {
		t.Errorf("Expected 'invalid setup token format' error, got: %v", err)
	}
}
