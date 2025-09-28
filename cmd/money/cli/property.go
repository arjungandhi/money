package cli

import (
	"fmt"
	"strconv"
	"strings"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/format"
	"github.com/arjungandhi/money/pkg/property"
	"github.com/arjungandhi/money/pkg/table"
)

var Property = &Z.Cmd{
	Name:    "property",
	Aliases: []string{"prop", "p"},
	Summary: "Manage property accounts and valuations using RentCast API",
	Commands: []*Z.Cmd{
		help.Cmd,
		PropertyConfig,
		PropertyAdd,
		PropertyList,
		PropertyUpdate,
		PropertyUpdateAll,
		PropertySetValue,
		PropertyDetails,
	},
}

var PropertyConfig = &Z.Cmd{
	Name:     "config",
	Summary:  "Configure RentCast API key for property valuations",
	Usage:    "<api-key>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <api-key>", cmd.Usage)
		}

		apiKey := args[0]

		// Basic validation - RentCast API keys are typically alphanumeric
		if len(apiKey) < 10 {
			return fmt.Errorf("API key appears to be too short. Please check your RentCast API key")
		}

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		// Save the API key
		err = db.SaveRentCastAPIKey(apiKey)
		if err != nil {
			return fmt.Errorf("failed to save RentCast API key: %w", err)
		}

		fmt.Println("Successfully saved RentCast API key!")
		fmt.Println("You can now use 'money property update' and 'money property update-all' commands.")
		fmt.Println("To get a RentCast API key, visit: https://developers.rentcast.io/")

		return nil
	},
}

var PropertyAdd = &Z.Cmd{
	Name:     "add",
	Summary:  "Add a new property account",
	Usage:    "<name> <address> <city> <state> <zipcode> [property-type] [latitude] [longitude]",
	Description: `
Add a new property account for tracking real estate in your net worth.

Property types (optional, case-sensitive):
  "Single Family" - Detached single-family home
  "Condo"         - Condominium unit
  "Townhouse"     - Townhouse/rowhouse
  "Manufactured"  - Mobile/manufactured home
  "Multi-Family"  - 2-4 unit residential building
  "Apartment"     - 5+ unit apartment building
  "Land"          - Vacant land/lot

Examples:
  money property add "My House" "123 Main St" "Austin" "TX" "78701"
  money property add "My Condo" "456 Oak St" "Miami" "FL" "33101" "Condo"
  money property add "My House" "789 Pine St" "Denver" "CO" "80202" "Single Family" 39.7392 -104.9903
`,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) < 5 {
			return fmt.Errorf("usage: %s <name> <address> <city> <state> <zipcode> [property-type] [latitude] [longitude]", cmd.Usage)
		}

		name := args[0]
		address := args[1]
		city := args[2]
		state := args[3]
		zipCode := args[4]

		var propertyType *string
		var latitude, longitude *float64

		// Parse optional arguments
		if len(args) >= 6 {
			// Check if arg 5 is a valid property type
			validTypes := []string{"Single Family", "Condo", "Townhouse", "Manufactured", "Multi-Family", "Apartment", "Land"}
			isValidType := false
			for _, validType := range validTypes {
				if args[5] == validType {
					isValidType = true
					break
				}
			}
			if isValidType {
				propertyType = &args[5]
				// Parse coordinates starting from arg 6
				if len(args) >= 8 {
					if lat, err := strconv.ParseFloat(args[6], 64); err == nil {
						latitude = &lat
					}
					if lon, err := strconv.ParseFloat(args[7], 64); err == nil {
						longitude = &lon
					}
				}
			} else {
				// Assume args[5] and args[6] are latitude and longitude
				if lat, err := strconv.ParseFloat(args[5], 64); err == nil {
					latitude = &lat
				}
				if len(args) >= 7 {
					if lon, err := strconv.ParseFloat(args[6], 64); err == nil {
						longitude = &lon
					}
				}
			}
		}

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		// Use "Property" as the org ID for manual property entries
		accountID, err := propertyService.CreatePropertyAccount("Property", name, address, city, state, zipCode, propertyType, latitude, longitude)
		if err != nil {
			return fmt.Errorf("failed to create property account: %w", err)
		}

		fmt.Printf("Successfully created property account:\n")
		fmt.Printf("  Name: %s\n", name)
		fmt.Printf("  Address: %s, %s, %s %s\n", address, city, state, zipCode)
		if propertyType != nil {
			fmt.Printf("  Property Type: %s\n", *propertyType)
		}
		fmt.Printf("  Account ID: %s\n", accountID)

		if latitude != nil && longitude != nil {
			fmt.Printf("  Location: %.6f, %.6f\n", *latitude, *longitude)
		}

		fmt.Println("\nTo get current valuation, run: money property update", accountID)

		return nil
	},
}

var PropertyList = &Z.Cmd{
	Name:     "list",
	Aliases:  []string{"ls", "l"},
	Summary:  "List all property accounts with their details",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		properties, err := propertyService.ListAllProperties()
		if err != nil {
			return fmt.Errorf("failed to list properties: %w", err)
		}

		if len(properties) == 0 {
			fmt.Println("No property accounts found. Use 'money property add' to add a property.")
			return nil
		}

		config := table.DefaultConfig()
		config.Title = "Property Accounts"
		config.MaxColumnWidth = 30

		t := table.NewWithConfig(config, "Account ID", "Address", "Value", "Last Updated")

		for _, prop := range properties {
			// Get account details for the current balance
			account, err := db.GetAccountByID(prop.AccountID)
			if err != nil {
				continue
			}

			address := fmt.Sprintf("%s, %s", prop.Address, prop.City)

			valueStr := "N/A"
			if account.Balance > 0 {
				valueStr = format.Currency(account.Balance, "USD")
			}

			lastUpdated := "Never"
			if prop.LastUpdated != nil {
				lastUpdated = *prop.LastUpdated
			}

			t.AddRow(prop.AccountID, address, valueStr, lastUpdated)
		}

		if err := t.Render(); err != nil {
			return fmt.Errorf("failed to render property table: %w", err)
		}

		return nil
	},
}

var PropertyUpdate = &Z.Cmd{
	Name:     "update",
	Summary:  "Update valuation for a specific property using RentCast API",
	Usage:    "<account-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <account-id>", cmd.Usage)
		}

		accountID := args[0]

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		fmt.Printf("Updating valuation for property: %s\n", accountID)

		err = propertyService.UpdatePropertyValuation(accountID)
		if err != nil {
			return fmt.Errorf("failed to update property valuation: %w", err)
		}

		// Get updated property details
		propertyDetails, err := propertyService.GetPropertyDetails(accountID)
		if err != nil {
			return fmt.Errorf("failed to get updated property details: %w", err)
		}

		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return fmt.Errorf("failed to get account details: %w", err)
		}

		fmt.Printf("Successfully updated property valuation:\n")
		fmt.Printf("  Address: %s, %s, %s %s\n", propertyDetails.Address, propertyDetails.City, propertyDetails.State, propertyDetails.ZipCode)
		fmt.Printf("  Current Value: %s\n", format.Currency(account.Balance, "USD"))

		if propertyDetails.LastRentEstimate != nil {
			fmt.Printf("  Estimated Rent: %s/month\n", format.Currency(*propertyDetails.LastRentEstimate, "USD"))
		}

		return nil
	},
}

var PropertyUpdateAll = &Z.Cmd{
	Name:     "update-all",
	Summary:  "Update valuations for all property accounts using RentCast API",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		fmt.Println("Updating valuations for all property accounts...")

		err = propertyService.UpdateAllPropertyValuations()
		if err != nil {
			return fmt.Errorf("failed to update property valuations: %w", err)
		}

		fmt.Println("Successfully updated all property valuations.")
		fmt.Println("Run 'money balance' to see updated net worth with current property values.")

		return nil
	},
}

var PropertySetValue = &Z.Cmd{
	Name:     "set-value",
	Summary:  "Manually set the value for a property account",
	Usage:    "<account-id> <value>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 2 {
			return fmt.Errorf("usage: %s <account-id> <value>", cmd.Usage)
		}

		accountID := args[0]
		valueStr := args[1]

		// Parse the value (assume it's in dollars)
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return fmt.Errorf("invalid value '%s': must be a number", valueStr)
		}

		if value < 0 {
			return fmt.Errorf("property value cannot be negative")
		}

		// Convert to cents
		valueInCents := int(value * 100)

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		// Verify this is a property account
		_, err = propertyService.GetPropertyDetails(accountID)
		if err != nil {
			return fmt.Errorf("property not found: %w", err)
		}

		err = propertyService.SetPropertyValue(accountID, valueInCents)
		if err != nil {
			return fmt.Errorf("failed to set property value: %w", err)
		}

		fmt.Printf("Successfully set property value to %s for account: %s\n", format.Currency(valueInCents, "USD"), accountID)

		return nil
	},
}

var PropertyDetails = &Z.Cmd{
	Name:     "details",
	Aliases:  []string{"detail", "info"},
	Summary:  "Show detailed information for a specific property",
	Usage:    "<account-id>",
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(cmd *Z.Cmd, args ...string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: %s <account-id>", cmd.Usage)
		}

		accountID := args[0]

		db, err := database.New()
		if err != nil {
			return err
		}
		defer db.Close()

		propertyService := property.NewService(db)

		// Get property details
		propertyDetails, err := propertyService.GetPropertyDetails(accountID)
		if err != nil {
			return fmt.Errorf("failed to get property details: %w", err)
		}

		// Get account details
		account, err := db.GetAccountByID(accountID)
		if err != nil {
			return fmt.Errorf("failed to get account details: %w", err)
		}

		fmt.Println("Property Details")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Printf("Account ID: %s\n", accountID)
		fmt.Printf("Name: %s\n", account.DisplayName())
		fmt.Printf("Address: %s\n", propertyDetails.Address)
		fmt.Printf("City: %s\n", propertyDetails.City)
		fmt.Printf("State: %s\n", propertyDetails.State)
		fmt.Printf("ZIP Code: %s\n", propertyDetails.ZipCode)
		if propertyDetails.PropertyType != nil {
			fmt.Printf("Property Type: %s\n", *propertyDetails.PropertyType)
		}

		if propertyDetails.Latitude != nil && propertyDetails.Longitude != nil {
			fmt.Printf("Location: %.6f, %.6f\n", *propertyDetails.Latitude, *propertyDetails.Longitude)
		}

		fmt.Printf("Current Value: %s\n", format.Currency(account.Balance, "USD"))

		if propertyDetails.LastValueEstimate != nil {
			fmt.Printf("Last Value Estimate: %s\n", format.Currency(*propertyDetails.LastValueEstimate, "USD"))
		}

		if propertyDetails.LastRentEstimate != nil {
			fmt.Printf("Last Rent Estimate: %s/month\n", format.Currency(*propertyDetails.LastRentEstimate, "USD"))
		}

		if propertyDetails.LastUpdated != nil {
			fmt.Printf("Last Updated: %s\n", *propertyDetails.LastUpdated)
		} else {
			fmt.Println("Last Updated: Never")
		}

		return nil
	},
}
