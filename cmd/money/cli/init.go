package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	Z "github.com/rwxrob/bonzai/z"

	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/simplefin"
)

var Init = &Z.Cmd{
	Name:    "init",
	Summary: "Initialize SimpleFIN credentials and setup",
	Usage:   "init",
	Description: `
Initialize the money CLI by setting up SimpleFIN credentials.

This command will:
1. Prompt you for a SimpleFIN setup token
2. Exchange the token for permanent access credentials 
3. Store the credentials securely in the local database
4. Test the connection to verify setup

You can get a setup token from your financial institution's
SimpleFIN portal or bridge service.
`,
	Call: initCommand,
}

func initCommand(cmd *Z.Cmd, args ...string) error {
	fmt.Println("Money CLI Initialization")
	fmt.Println("=======================")
	fmt.Println()

	// Initialize database connection
	fmt.Println("Initializing database...")
	db, err := database.New()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Check if credentials already exist
	hasCredentials, err := db.HasCredentials()
	if err != nil {
		return fmt.Errorf("failed to check existing credentials: %w", err)
	}

	if hasCredentials {
		fmt.Println("⚠️  Existing credentials found!")
		fmt.Println()
		
		if !confirmOverwrite() {
			fmt.Println("Setup cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Prompt for SimpleFIN setup token
	setupToken, err := promptForSetupToken()
	if err != nil {
		return fmt.Errorf("failed to get setup token: %w", err)
	}

	// Validate token format
	if err := validateSetupToken(setupToken); err != nil {
		return fmt.Errorf("invalid setup token: %w", err)
	}

	fmt.Println("Exchanging setup token for permanent credentials...")

	// Exchange token for credentials using SimpleFIN API
	client, err := simplefin.NewClientFromToken(setupToken)
	if err != nil {
		return fmt.Errorf("failed to exchange setup token: %w", err)
	}

	// Get the credentials from the client
	accessURL, username, password := client.GetCredentials()

	// Save credentials to database
	fmt.Println("Saving credentials...")
	if err := db.SaveCredentials(accessURL, username, password); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Test the connection by fetching accounts
	fmt.Println("Testing connection...")
	accounts, err := client.GetAccounts()
	if err != nil {
		return fmt.Errorf("failed to test connection - credentials may be invalid: %w", err)
	}

	// Confirm successful setup
	fmt.Println()
	fmt.Println("✅ Setup completed successfully!")
	fmt.Printf("Found %d organizations with %d total accounts\n", 
		len(accounts.Organizations), len(accounts.Accounts))
	
	if len(accounts.Organizations) > 0 {
		fmt.Println()
		fmt.Println("Connected organizations:")
		for _, org := range accounts.Organizations {
			accountCount := 0
			for _, account := range accounts.Accounts {
				if account.OrgID == org.ID {
					accountCount++
				}
			}
			fmt.Printf("  • %s (%d accounts)\n", org.Name, accountCount)
		}
	}

	fmt.Println()
	fmt.Println("You can now run other money commands:")
	fmt.Println("  money fetch    - Fetch latest account data")
	fmt.Println("  money balance  - Show account balances")
	fmt.Println("  money transactions - Show recent transactions")

	return nil
}

func confirmOverwrite() bool {
	fmt.Print("Do you want to overwrite the existing setup? (y/N): ")
	
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func promptForSetupToken() (string, error) {
	fmt.Println("Please enter your SimpleFIN setup token.")
	fmt.Println("This should be a URL starting with https://")
	fmt.Println("Example: https://bridge.simplefin.org/simplefin/claim/abc123...")
	fmt.Println()
	fmt.Print("Setup token: ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("setup token cannot be empty")
	}

	return token, nil
}

func validateSetupToken(token string) error {
	if !strings.HasPrefix(token, "https://") && !strings.HasPrefix(token, "http://") {
		return fmt.Errorf("setup token must be a URL starting with https://")
	}

	if !strings.Contains(token, "/claim/") {
		return fmt.Errorf("setup token must contain '/claim/' path")
	}

	// Basic validation - the simplefin package will do more thorough validation
	parts := strings.Split(token, "/claim/")
	if len(parts) != 2 || parts[1] == "" {
		return fmt.Errorf("setup token format invalid - missing claim token")
	}

	return nil
}
