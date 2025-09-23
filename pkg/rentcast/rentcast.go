package rentcast

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	BaseURL = "https://api.rentcast.io/v1"
)

// Client represents a RentCast API client
type Client struct {
	APIKey     string
	HTTPClient *http.Client
}

// NewClient creates a new RentCast API client
func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ValueEstimateRequest represents the parameters for a value estimate request
type ValueEstimateRequest struct {
	Address      string
	City         string
	State        string
	ZipCode      string
	Latitude     *float64
	Longitude    *float64
	PropertyType *string // e.g., "Single Family", "Condo", "Townhouse", "Multi Family"
}

// ValueEstimateResponse represents the response from the value estimate API
type ValueEstimateResponse struct {
	Price     *int    `json:"price"`
	PriceLow  *int    `json:"priceRangeLow"`
	PriceHigh *int    `json:"priceRangeHigh"`
	Accuracy  *string `json:"accuracy"`
	Error     *string `json:"error"`
}

// RentEstimateResponse represents the response from the rent estimate API
type RentEstimateResponse struct {
	Rent     *int    `json:"rent"`
	RentLow  *int    `json:"rentRangeLow"`
	RentHigh *int    `json:"rentRangeHigh"`
	Accuracy *string `json:"accuracy"`
	Error    *string `json:"error"`
}

// GetValueEstimate gets a property value estimate using the RentCast API
func (c *Client) GetValueEstimate(req ValueEstimateRequest) (*ValueEstimateResponse, error) {
	params := url.Values{}
	params.Set("address", req.Address)
	params.Set("city", req.City)
	params.Set("state", req.State)
	params.Set("zipCode", req.ZipCode)

	if req.Latitude != nil {
		params.Set("latitude", strconv.FormatFloat(*req.Latitude, 'f', -1, 64))
	}
	if req.Longitude != nil {
		params.Set("longitude", strconv.FormatFloat(*req.Longitude, 'f', -1, 64))
	}
	if req.PropertyType != nil {
		params.Set("propertyType", *req.PropertyType)
	}

	url := fmt.Sprintf("%s/avm/value?%s", BaseURL, params.Encode())

	resp, err := c.makeRequest("GET", url)
	if err != nil {
		return nil, fmt.Errorf("failed to make value estimate request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var valueResp ValueEstimateResponse
	if err := json.Unmarshal(body, &valueResp); err != nil {
		return nil, fmt.Errorf("failed to parse value estimate response: %w", err)
	}

	return &valueResp, nil
}

// GetRentEstimate gets a property rent estimate using the RentCast API
func (c *Client) GetRentEstimate(req ValueEstimateRequest) (*RentEstimateResponse, error) {
	params := url.Values{}
	params.Set("address", req.Address)
	params.Set("city", req.City)
	params.Set("state", req.State)
	params.Set("zipCode", req.ZipCode)

	if req.Latitude != nil {
		params.Set("latitude", strconv.FormatFloat(*req.Latitude, 'f', -1, 64))
	}
	if req.Longitude != nil {
		params.Set("longitude", strconv.FormatFloat(*req.Longitude, 'f', -1, 64))
	}
	if req.PropertyType != nil {
		params.Set("propertyType", *req.PropertyType)
	}

	url := fmt.Sprintf("%s/avm/rent/long-term?%s", BaseURL, params.Encode())

	resp, err := c.makeRequest("GET", url)
	if err != nil {
		return nil, fmt.Errorf("failed to make rent estimate request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rentResp RentEstimateResponse
	if err := json.Unmarshal(body, &rentResp); err != nil {
		return nil, fmt.Errorf("failed to parse rent estimate response: %w", err)
	}

	return &rentResp, nil
}

// makeRequest makes an HTTP request with the API key header
func (c *Client) makeRequest(method, url string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "money-cli/1.0")

	return c.HTTPClient.Do(req)
}
