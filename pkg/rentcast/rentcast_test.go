package rentcast

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	apiKey := "test-api-key"
	client := NewClient(apiKey)

	if client == nil {
		t.Error("NewClient should return a non-nil client")
	}

	if client.APIKey != apiKey {
		t.Errorf("Expected API key '%s', got '%s'", apiKey, client.APIKey)
	}

	if client.HTTPClient == nil {
		t.Error("Client should have an HTTP client")
	}
}

func TestValueEstimateRequest(t *testing.T) {
	lat := 37.7749
	lng := -122.4194
	propType := "Single Family"

	request := ValueEstimateRequest{
		Address:      "123 Main St",
		City:         "San Francisco",
		State:        "CA",
		ZipCode:      "94102",
		Latitude:     &lat,
		Longitude:    &lng,
		PropertyType: &propType,
	}

	if request.Address != "123 Main St" {
		t.Error("Address field should be set correctly")
	}

	if *request.PropertyType != "Single Family" {
		t.Error("PropertyType field should be set correctly")
	}
}

func TestValueEstimateResponse(t *testing.T) {
	price := 1500000
	response := ValueEstimateResponse{
		Price: &price,
	}

	if *response.Price != 1500000 {
		t.Error("Price field should be set correctly")
	}
}

func TestRentEstimateResponse(t *testing.T) {
	rent := 5000
	response := RentEstimateResponse{
		Rent: &rent,
	}

	if *response.Rent != 5000 {
		t.Error("Rent field should be set correctly")
	}
}
