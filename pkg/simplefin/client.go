package simplefin

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *Client) ExchangeToken(token string) (accessURL, username, password string, err error) {
	// Parse the setup token URL to extract the bridge URL and claim token
	tokenURL, err := url.Parse(token)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid setup token URL: %w", err)
	}

	// Extract the claim token from the URL path (after /claim/)
	pathParts := strings.Split(tokenURL.Path, "/")
	if len(pathParts) < 3 || pathParts[1] != "claim" {
		return "", "", "", fmt.Errorf("invalid setup token format: expected /claim/{token}")
	}
	claimToken := pathParts[2]

	// Build the bridge URL (base URL without the /claim/{token} path)
	bridgeURL := fmt.Sprintf("%s://%s", tokenURL.Scheme, tokenURL.Host)

	// Create the claim request
	claimURL := fmt.Sprintf("%s/claim", bridgeURL)
	requestBody := bytes.NewBufferString(claimToken)

	req, err := http.NewRequest("POST", claimURL, requestBody)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create claim request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

func (c *Client) GetAccounts() (*AccountsResponse, error) {
	if c.accessURL == "" {
		return nil, fmt.Errorf("access URL not set - call ExchangeToken first or create client with credentials")
	}

	// Build the accounts endpoint URL
	accountsURL := strings.TrimSuffix(c.accessURL, "/") + "/accounts"

	// Create the request
	req, err := http.NewRequest("GET", accountsURL, nil)
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
	Accounts      []Account      `json:"accounts"`
	Organizations []Organization `json:"organizations"`
}

type Account struct {
	ID               string        `json:"id"`
	OrgID            string        `json:"org"`
	Name             string        `json:"name"`
	Currency         string        `json:"currency"`
	Balance          int           `json:"balance"` // Amount in cents
	AvailableBalance *int          `json:"available-balance,omitempty"`
	BalanceDate      *string       `json:"balance-date,omitempty"`
	Transactions     []Transaction `json:"transactions"`
}

type Transaction struct {
	ID          string `json:"id"`
	Posted      string `json:"posted"` // ISO 8601 timestamp
	Amount      int    `json:"amount"` // Amount in cents
	Description string `json:"description"`
	Pending     *bool  `json:"pending,omitempty"`
}

type Organization struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	URL  *string `json:"url,omitempty"`
}
