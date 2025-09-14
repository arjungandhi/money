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
	claimURLBytes, err := base64.StdEncoding.DecodeString(setupToken)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid setup token: failed to decode base64: %w", err)
	}

	claimURL := string(claimURLBytes)
	_, err = url.Parse(claimURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid claim URL: %w", err)
	}

	req, err := http.NewRequest("POST", claimURL, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create claim request: %w", err)
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read token exchange response: %w", err)
	}

	accessURLStr := strings.TrimSpace(string(body))
	parsedURL, err := url.Parse(accessURLStr)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid access URL in response: %w", err)
	}

	if parsedURL.User == nil {
		return "", "", "", fmt.Errorf("access URL does not contain credentials")
	}

	username = parsedURL.User.Username()
	password, _ = parsedURL.User.Password()

	parsedURL.User = nil
	accessURL = parsedURL.String()

	return accessURL, username, password, nil
}

type AccountsOptions struct {
	StartDate    *time.Time
	EndDate      *time.Time
	Pending      *bool
	AccountID    string
	BalancesOnly bool
}

func (c *Client) GetAccounts() (*AccountsResponse, error) {
	return c.GetAccountsWithOptions(nil)
}

func (c *Client) GetAccountsWithOptions(opts *AccountsOptions) (*AccountsResponse, error) {
	if c.accessURL == "" {
		return nil, fmt.Errorf("access URL not set - call ExchangeToken first or create client with credentials")
	}

	baseURL := strings.TrimSuffix(c.accessURL, "/") + "/accounts"

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse accounts URL: %w", err)
	}

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

	req, err := http.NewRequest("GET", parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounts request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "money-cli/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("accounts request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read accounts response: %w", err)
	}

	var accountsResponse AccountsResponse
	if err := json.Unmarshal(body, &accountsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse accounts JSON response: %w", err)
	}

	if len(accountsResponse.Errors) > 0 {
		fmt.Printf("Warning - some institutions had errors: %v\n", accountsResponse.Errors)
	}

	return &accountsResponse, nil
}

func (c *Client) GetCredentials() (accessURL, username, password string) {
	return c.accessURL, c.username, c.password
}

func (c *Client) IsConfigured() bool {
	return c.accessURL != "" && c.username != "" && c.password != ""
}

func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

type AccountsResponse struct {
	Accounts []Account `json:"accounts"`
	Errors   []string  `json:"errors"`
}

type Account struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Org              Organization           `json:"org"`
	Currency         string                 `json:"currency"`
	Balance          string                 `json:"balance"`
	AvailableBalance *string                `json:"available-balance,omitempty"`
	BalanceDate      *int64                 `json:"balance-date,omitempty"`
	Transactions     []Transaction          `json:"transactions"`
	Holdings         []Holding              `json:"holdings,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

type Transaction struct {
	ID           string                 `json:"id"`
	Posted       int64                  `json:"posted"`
	Amount       string                 `json:"amount"`
	Description  string                 `json:"description"`
	Memo         string                 `json:"memo,omitempty"`
	Payee        string                 `json:"payee,omitempty"`
	Pending      *bool                  `json:"pending,omitempty"`
	TransactedAt *int64                 `json:"transacted_at,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
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

func ParseAmountToCents(amountStr string) (int, error) {
	if amountStr == "" {
		return 0, nil
	}
	
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount '%s': %w", amountStr, err)
	}
	
	cents := amount * 100
	if cents >= 0 {
		cents += 0.5
	} else {
		cents -= 0.5
	}
	return int(cents), nil
}

func UnixTimestampToISO(unixTimestamp int64) string {
	if unixTimestamp == 0 {
		return ""
	}
	return time.Unix(unixTimestamp, 0).UTC().Format(time.RFC3339)
}
