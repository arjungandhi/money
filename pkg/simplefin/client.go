package simplefin

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	accessURL  string
	username   string
	password   string
}

func NewClient(accessURL, username, password string) *Client {
	return &Client{
		httpClient: createHTTPClient(),
		accessURL:  accessURL,
		username:   username,
		password:   password,
	}
}

// NewClientFromToken creates a new client by exchanging a setup token
func NewClientFromToken(setupToken string) (*Client, error) {
	tempClient := &Client{
		httpClient: createHTTPClient(),
	}

	accessURL, username, password, err := tempClient.ExchangeToken(setupToken)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	return NewClient(accessURL, username, password), nil
}

// createHTTPClient creates a configured HTTP client for SimpleFIN API requests
func createHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Require SSL certificate verification
			},
		},
	}
}

func (c *Client) ExchangeToken(setupToken string) (accessURL, username, password string, err error) {
	// Decode the base64 setup token to get the claim URL
	claimURLBytes, err := base64.StdEncoding.DecodeString(setupToken)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid setup token: failed to decode base64: %w", err)
	}
	
	claimURL := string(claimURLBytes)
	
	// Validate that the claim URL is valid
	_, err = url.Parse(claimURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid claim URL: %w", err)
	}

	// Create the claim request - POST to the claim URL
	req, err := http.NewRequest("POST", claimURL, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create claim request: %w", err)
	}

	// No special headers needed for SimpleFIN token exchange

	// Use the client's HTTP client for token exchange, or create one if not set
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = createHTTPClient()
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body which contains the access URL with embedded credentials
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read token exchange response: %w", err)
	}

	// Parse the access URL from the response
	accessURLStr := strings.TrimSpace(string(body))
	parsedURL, err := url.Parse(accessURLStr)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid access URL in response: %w", err)
	}

	// Extract credentials from the URL
	if parsedURL.User == nil {
		return "", "", "", fmt.Errorf("access URL does not contain credentials")
	}

	username = parsedURL.User.Username()
	password, _ = parsedURL.User.Password()

	// Remove credentials from URL to get the clean access URL
	parsedURL.User = nil
	accessURL = parsedURL.String()

	return accessURL, username, password, nil
}

// AccountsOptions contains optional parameters for GetAccounts requests
type AccountsOptions struct {
	StartDate    *time.Time // Start date for transaction filtering
	EndDate      *time.Time // End date for transaction filtering
	Pending      *bool      // Include pending transactions
	AccountID    string     // Filter specific account
	BalancesOnly bool       // Fetch balances only (faster)
}

func (c *Client) GetAccounts() (*AccountsResponse, error) {
	return c.GetAccountsWithOptions(nil)
}

func (c *Client) GetAccountsWithOptions(opts *AccountsOptions) (*AccountsResponse, error) {
	if c.accessURL == "" {
		return nil, fmt.Errorf("access URL not set - call ExchangeToken first or create client with credentials")
	}

	// Build the accounts endpoint URL
	baseURL := strings.TrimSuffix(c.accessURL, "/") + "/accounts"

	// Parse base URL to add query parameters
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse accounts URL: %w", err)
	}

	// Add query parameters if options provided
	if opts != nil {
		query := parsedURL.Query()

		if opts.StartDate != nil {
			query.Set("start-date", fmt.Sprintf("%d", opts.StartDate.Unix()))
		}
		if opts.EndDate != nil {
			query.Set("end-date", fmt.Sprintf("%d", opts.EndDate.Unix()))
		}
		if opts.Pending != nil {
			if *opts.Pending {
				query.Set("pending", "1")
			} else {
				query.Set("pending", "0")
			}
		}
		if opts.AccountID != "" {
			query.Set("account", opts.AccountID)
		}
		if opts.BalancesOnly {
			query.Set("balances-only", "1")
		}

		parsedURL.RawQuery = query.Encode()
	}

	// Create the request
	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}

	// Set basic authentication
	req.SetBasicAuth(c.username, c.password)

	// Set appropriate headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "money-cli/1.0")

	// Make the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("accounts request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read accounts response: %w", err)
	}

	// Parse JSON response
	var accountsResponse AccountsResponse
	if err := json.Unmarshal(body, &accountsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse accounts JSON response: %w", err)
	}

	// Log API errors but don't fail - some institutions may timeout but others work
	if len(accountsResponse.Errors) > 0 {
		fmt.Printf("Warning - some institutions had errors: %v\n", accountsResponse.Errors)
	}

	return &accountsResponse, nil
}

// GetCredentials returns the stored access credentials
func (c *Client) GetCredentials() (accessURL, username, password string) {
	return c.accessURL, c.username, c.password
}

// IsConfigured returns true if the client has valid credentials
func (c *Client) IsConfigured() bool {
	return c.accessURL != "" && c.username != "" && c.password != ""
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

// Response types based on SimpleFIN API
type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
	Errors   []string  `json:"errors"`
}

type Account struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Org              Organization           `json:"org"`
	Currency         string                 `json:"currency"`
	Balance          string                 `json:"balance"`           // Amount as string
	AvailableBalance *string                `json:"available-balance,omitempty"`
	BalanceDate      *int64                 `json:"balance-date,omitempty"` // Unix timestamp
	Transactions     []Transaction          `json:"transactions"`
	Holdings         []Holding              `json:"holdings,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"` // Additional fields
}

type Transaction struct {
	ID           string                 `json:"id"`
	Posted       int64                  `json:"posted"`       // Unix timestamp
	Amount       string                 `json:"amount"`       // Amount as string
	Description  string                 `json:"description"`
	Memo         string                 `json:"memo,omitempty"`
	Payee        string                 `json:"payee,omitempty"`
	Pending      *bool                  `json:"pending,omitempty"`
	TransactedAt *int64                 `json:"transacted_at,omitempty"` // Unix timestamp
	Extra        map[string]interface{} `json:"extra,omitempty"`         // Additional fields
}

type Organization struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Domain   string  `json:"domain,omitempty"`
	URL      *string `json:"url,omitempty"`
	SfinURL  *string `json:"sfin-url,omitempty"`
}

type Holding struct {
	ID            string `json:"id"`
	Symbol        string `json:"symbol,omitempty"`
	Description   string `json:"description,omitempty"`
	Shares        string `json:"shares,omitempty"`
	Currency      string `json:"currency,omitempty"`
	MarketValue   string `json:"market_value,omitempty"`
	PurchasePrice string `json:"purchase_price,omitempty"`
	CostBasis     string `json:"cost_basis,omitempty"`
	Created       *int64 `json:"created,omitempty"`
}

// Utility functions for data conversion

// ParseAmountToCents converts a string amount (e.g., "123.45") to cents (e.g., 12345)
func ParseAmountToCents(amountStr string) (int, error) {
	if amountStr == "" {
		return 0, nil
	}
	
	// Parse the amount as a float
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount '%s': %w", amountStr, err)
	}
	
	// Convert to cents with proper rounding for both positive and negative numbers
	cents := amount * 100
	if cents >= 0 {
		cents += 0.5
	} else {
		cents -= 0.5
	}
	return int(cents), nil
}

// UnixTimestampToISO converts a unix timestamp to ISO 8601 string format
func UnixTimestampToISO(unixTimestamp int64) string {
	if unixTimestamp == 0 {
		return ""
	}
	return time.Unix(unixTimestamp, 0).UTC().Format(time.RFC3339)
}
