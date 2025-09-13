package simplefin

import (
	"crypto/tls"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	accessURL  string
	username   string
	password   string
}

func NewClient(accessURL, username, password string) *Client {
	// Configure HTTP client with SSL verification
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Require SSL certificate verification
			},
		},
	}

	return &Client{
		httpClient: client,
		accessURL:  accessURL,
		username:   username,
		password:   password,
	}
}

func (c *Client) ExchangeToken(token string) (accessURL, username, password string, err error) {
	// TODO: Implement token exchange with SimpleFIN Bridge
	// 1. POST token to SimpleFIN Bridge
	// 2. Receive Access URL and credentials
	return "", "", "", nil
}

func (c *Client) GetAccounts() (*AccountsResponse, error) {
	// TODO: Implement GET /accounts endpoint
	// 1. Make authenticated request to Access URL + /accounts
	// 2. Parse JSON response
	// 3. Return structured data
	return nil, nil
}

// Response types based on SimpleFIN API
type AccountsResponse struct {
	Accounts      []Account      `json:"accounts"`
	Organizations []Organization `json:"organizations"`
}

type Account struct {
	ID               string  `json:"id"`
	OrgID           string  `json:"org"`
	Name            string  `json:"name"`
	Currency        string  `json:"currency"`
	Balance         int     `json:"balance"`          // Amount in cents
	AvailableBalance *int    `json:"available-balance,omitempty"`
	BalanceDate     *string `json:"balance-date,omitempty"`
	Transactions    []Transaction `json:"transactions"`
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