package property

import (
	"fmt"
	"os"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/rentcast"
)

// Service handles property-related operations
type Service struct {
	db             *database.DB
	rentcastClient *rentcast.Client
}

// NewService creates a new property service
func NewService(db *database.DB) *Service {
	// Try to get API key from database first, then fallback to environment variable
	var client *rentcast.Client
	if apiKey, err := db.GetRentCastAPIKey(); err == nil {
		client = rentcast.NewClient(apiKey)
	} else if apiKey := os.Getenv("RENTCAST_API_KEY"); apiKey != "" {
		client = rentcast.NewClient(apiKey)
	}

	return &Service{
		db:             db,
		rentcastClient: client,
	}
}

// CreatePropertyAccount creates a new property account and associated property record
func (s *Service) CreatePropertyAccount(orgID, name, address, city, state, zipCode string, propertyType *string, latitude, longitude *float64) (string, error) {
	// Generate a unique account ID (you might want to use a UUID library)
	accountID := fmt.Sprintf("property_%s_%s_%s", state, city, zipCode)

	// Create the account with property type
	err := s.db.SaveAccount(accountID, orgID, name, "USD", 0, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create property account: %w", err)
	}

	// Set account type to property
	err = s.db.SetAccountType(accountID, "property")
	if err != nil {
		return "", fmt.Errorf("failed to set account type to property: %w", err)
	}

	// Save property details
	err = s.db.SaveProperty(accountID, address, city, state, zipCode, propertyType, latitude, longitude)
	if err != nil {
		return "", fmt.Errorf("failed to save property details: %w", err)
	}

	return accountID, nil
}

// UpdatePropertyValuation fetches current valuation from RentCast and updates the property
func (s *Service) UpdatePropertyValuation(accountID string) error {
	if s.rentcastClient == nil {
		return fmt.Errorf("RentCast API key not configured. Run 'money property config' to set your API key")
	}

	// Get property details
	property, err := s.db.GetProperty(accountID)
	if err != nil {
		return fmt.Errorf("failed to get property details: %w", err)
	}

	// Prepare request
	req := rentcast.ValueEstimateRequest{
		Address:      property.Address,
		City:         property.City,
		State:        property.State,
		ZipCode:      property.ZipCode,
		PropertyType: property.PropertyType,
		Latitude:     property.Latitude,
		Longitude:    property.Longitude,
	}

	// Get value estimate
	valueResp, err := s.rentcastClient.GetValueEstimate(req)
	if err != nil {
		return fmt.Errorf("failed to get value estimate: %w", err)
	}

	// Get rent estimate
	rentResp, err := s.rentcastClient.GetRentEstimate(req)
	if err != nil {
		return fmt.Errorf("failed to get rent estimate: %w", err)
	}

	// Convert to cents for storage
	var valueEstimate, rentEstimate *int
	if valueResp.Price != nil {
		value := (*valueResp.Price) * 100 // Convert dollars to cents
		valueEstimate = &value
	}
	if rentResp.Rent != nil {
		rent := (*rentResp.Rent) * 100 // Convert dollars to cents
		rentEstimate = &rent
	}

	// Update property valuation in database
	err = s.db.UpdatePropertyValuation(accountID, valueEstimate, rentEstimate)
	if err != nil {
		return fmt.Errorf("failed to update property valuation: %w", err)
	}

	// Update account balance to reflect current property value
	if valueEstimate != nil {
		err = s.db.UpdateAccountBalance(accountID, *valueEstimate)
		if err != nil {
			return fmt.Errorf("failed to update account balance: %w", err)
		}

		// Record balance history for trending
		err = s.db.SaveBalanceHistory(accountID, *valueEstimate, nil)
		if err != nil {
			return fmt.Errorf("failed to save balance history: %w", err)
		}
	}

	return nil
}

// UpdateAllPropertyValuations updates valuations for all property accounts
func (s *Service) UpdateAllPropertyValuations() error {
	if s.rentcastClient == nil {
		return fmt.Errorf("RentCast API key not configured. Run 'money property config' to set your API key")
	}

	properties, err := s.db.GetAllProperties()
	if err != nil {
		return fmt.Errorf("failed to get properties: %w", err)
	}

	var errors []string
	for _, property := range properties {
		err := s.UpdatePropertyValuation(property.AccountID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to update %s: %v", property.Address, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some property valuations failed: %v", errors)
	}

	return nil
}

// GetPropertyDetails returns property details for a given account
func (s *Service) GetPropertyDetails(accountID string) (*database.Property, error) {
	return s.db.GetProperty(accountID)
}

// ListAllProperties returns all properties
func (s *Service) ListAllProperties() ([]database.Property, error) {
	return s.db.GetAllProperties()
}

// SetPropertyValue manually sets the value for a property account
func (s *Service) SetPropertyValue(accountID string, valueInCents int) error {
	// Update the account balance
	err := s.db.UpdateAccountBalance(accountID, valueInCents)
	if err != nil {
		return fmt.Errorf("failed to update account balance: %w", err)
	}

	// Record balance history for trending
	err = s.db.SaveBalanceHistory(accountID, valueInCents, nil)
	if err != nil {
		return fmt.Errorf("failed to save balance history: %w", err)
	}

	// Also update the property valuation record
	err = s.db.UpdatePropertyValuation(accountID, &valueInCents, nil)
	if err != nil {
		return fmt.Errorf("failed to update property valuation: %w", err)
	}

	return nil
}
