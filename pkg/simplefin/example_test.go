package simplefin_test

import (
	"fmt"
	"log"

	"github.com/arjungandhi/money/pkg/simplefin"
)

func ExampleClient_ExchangeToken() {
	// Exchange a SimpleFIN setup token for access credentials
	client := &simplefin.Client{}
	accessURL, username, password, err := client.ExchangeToken("https://bridge.simplefin.org/claim/abcd1234")
	if err != nil {
		log.Fatalf("Token exchange failed: %v", err)
	}

	fmt.Printf("Access URL: %s\n", accessURL)
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Password: %s\n", password)
}

func ExampleNewClientFromToken() {
	// Create a client directly from a setup token
	client, err := simplefin.NewClientFromToken("https://bridge.simplefin.org/claim/abcd1234")
	if err != nil {
		log.Fatalf("Client creation failed: %v", err)
	}

	// Check if client is properly configured
	if client.IsConfigured() {
		fmt.Println("Client successfully configured")
	}
}

func ExampleClient_GetAccounts() {
	// Create client with existing credentials
	client := simplefin.NewClient("https://bridge.simplefin.org/api", "myuser", "mypass")

	// Fetch all accounts and transactions
	response, err := client.GetAccounts()
	if err != nil {
		log.Fatalf("Failed to fetch accounts: %v", err)
	}

	// Process accounts
	for _, account := range response.Accounts {
		fmt.Printf("Account: %s (%s)\n", account.Name, account.Currency)
		fmt.Printf("Balance: $%.2f\n", float64(account.Balance)/100)
		fmt.Printf("Transactions: %d\n", len(account.Transactions))

		// Process recent transactions
		for i, txn := range account.Transactions {
			if i >= 3 { // Show only first 3 transactions
				break
			}
			fmt.Printf("  %s: $%.2f - %s\n", 
				txn.Posted, float64(txn.Amount)/100, txn.Description)
		}
	}

	// Process organizations
	for _, org := range response.Organizations {
		fmt.Printf("Organization: %s\n", org.Name)
	}
}