package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"

	"github.com/arjungandhi/money/pkg/config"
	"github.com/arjungandhi/money/pkg/database"
	"github.com/arjungandhi/money/pkg/simplefin"
)

var Init = &Z.Cmd{
	Name:     "init",
	Summary:  "Interactive setup tutorial for money CLI",
	Commands: []*Z.Cmd{
		help.Cmd,
		InitSimpleFIN,
		InitRentCast,
	},
	Description: `
Interactive setup tutorial for the money CLI.

This command will guide you through:
1. Configuring where to store your data
2. Setting up SimpleFIN for bank account access
3. Configuring RentCast for property valuations (optional)
4. Setting up LLM integration for transaction categorization (optional)
5. Adding environment variables to your shell configuration

Subcommands:
  simplefin  - Set up SimpleFIN credentials only
  rentcast   - Set up RentCast API key only

Examples:
  money init               # Interactive full setup
  money init simplefin     # SimpleFIN setup only
  money init rentcast      # RentCast setup only
`,
	Call: initCommand,
}

var InitSimpleFIN = &Z.Cmd{
	Name:     "simplefin",
	Summary:  "Set up SimpleFIN credentials for bank account access",
	Commands: []*Z.Cmd{help.Cmd},
	Usage:    "simplefin [setup-token]",
	Description: `
Set up SimpleFIN credentials for accessing your bank accounts.

This command will:
1. Use the provided setup token or prompt you for one
2. Exchange the token for permanent access credentials
3. Store the credentials securely in the local database
4. Test the connection to verify setup

You can get a setup token from your financial institution's
SimpleFIN portal or bridge service.

Examples:
  money init simplefin                                    # Interactive mode
  money init simplefin aHR0cHM6Ly9icmlkZ2Uuc2ltcGxlZmlu...  # Non-interactive mode
`,
	Call: initSimpleFinCommand,
}

var InitRentCast = &Z.Cmd{
	Name:     "rentcast",
	Summary:  "Set up RentCast API key for property valuations",
	Commands: []*Z.Cmd{help.Cmd},
	Usage:    "rentcast [api-key]",
	Description: `
Set up RentCast API key for property valuations.

This command will:
1. Use the provided API key or prompt you for one
2. Validate the API key format
3. Store the API key securely in the local database

You can get a RentCast API key from: https://developers.rentcast.io/

Examples:
  money init rentcast                 # Interactive mode
  money init rentcast your-api-key    # Non-interactive mode
`,
	Call: initRentCastCommand,
}

func initCommand(cmd *Z.Cmd, args ...string) error {
	fmt.Println("💰 Welcome to Money CLI Setup!")
	fmt.Println("==============================")
	fmt.Println()
	fmt.Println("This interactive setup will help you configure the money CLI step by step.")
	fmt.Println()

	cfg := config.New()

	// Step 1: Configure data storage location
	fmt.Println("📁 Step 1: Data Storage Location")
	fmt.Println("--------------------------------")
	fmt.Printf("Current data directory: %s\n", cfg.MoneyDir)

	if RunConfirmation("Would you like to change the data storage location?") {
		newDir := RunInputWithValidator("Enter new data directory path", cfg.MoneyDir, DirectoryValidator)
		if newDir != "" {
			cfg.SetMoneyDir(newDir)
			fmt.Printf("Data directory updated to: %s\n", cfg.MoneyDir)
		}
	}
	fmt.Println()

	// Step 2: SimpleFIN Setup
	fmt.Println("🏦 Step 2: Bank Account Access (SimpleFIN)")
	fmt.Println("------------------------------------------")
	fmt.Println("SimpleFIN allows secure access to your bank accounts for transaction data.")

	// Check for existing SimpleFIN credentials
	hasCredentials, err := checkExistingSimpleFINCredentials(cfg)
	if err != nil {
		fmt.Printf("⚠️  Could not check existing SimpleFIN credentials: %v\n", err)
	}

	if hasCredentials {
		fmt.Println("✅ SimpleFIN credentials already configured!")
		if RunConfirmation("Would you like to reconfigure SimpleFIN credentials?") {
			if err := runSimpleFinSetup(cfg); err != nil {
				fmt.Printf("⚠️  SimpleFIN setup failed: %v\n", err)
				fmt.Println("You can set it up later with: money init simplefin")
			}
		} else {
			fmt.Println("Keeping existing SimpleFIN configuration.")
		}
	} else {
		if RunConfirmation("Would you like to set up SimpleFIN now?") {
			if err := runSimpleFinSetup(cfg); err != nil {
				fmt.Printf("⚠️  SimpleFIN setup failed: %v\n", err)
				fmt.Println("You can set it up later with: money init simplefin")
			}
		} else {
			fmt.Println("Skipping SimpleFIN setup. You can set it up later with: money init simplefin")
		}
	}
	fmt.Println()

	// Step 3: RentCast Setup (optional)
	fmt.Println("🏠 Step 3: Property Valuations (RentCast) - Optional")
	fmt.Println("---------------------------------------------------")
	fmt.Println("RentCast provides property value estimates for real estate tracking.")

	// Check for existing RentCast API key
	hasRentCastKey, err := checkExistingRentCastCredentials(cfg)
	if err != nil {
		fmt.Printf("⚠️  Could not check existing RentCast credentials: %v\n", err)
	}

	if hasRentCastKey {
		fmt.Println("✅ RentCast API key already configured!")
		if RunConfirmation("Would you like to reconfigure RentCast API key?") {
			if err := runRentCastSetup(cfg); err != nil {
				fmt.Printf("⚠️  RentCast setup failed: %v\n", err)
				fmt.Println("You can set it up later with: money init rentcast")
			}
		} else {
			fmt.Println("Keeping existing RentCast configuration.")
		}
	} else {
		if RunConfirmation("Would you like to set up RentCast for property valuations?") {
			if err := runRentCastSetup(cfg); err != nil {
				fmt.Printf("⚠️  RentCast setup failed: %v\n", err)
				fmt.Println("You can set it up later with: money init rentcast")
			}
		} else {
			fmt.Println("Skipping RentCast setup. You can set it up later with: money init rentcast")
		}
	}
	fmt.Println()

	// Step 4: LLM Configuration (optional)
	fmt.Println("🤖 Step 4: AI-Powered Transaction Categorization - Optional")
	fmt.Println("----------------------------------------------------------")
	fmt.Println("Configure an LLM (like Claude, ChatGPT, or Ollama) for automatic transaction categorization.")
	fmt.Printf("Current LLM command: %s\n", cfg.LLMPromptCmd)

	if RunConfirmation("Would you like to configure LLM integration?") {
		if err := configureLLMInteractive(cfg); err != nil {
			fmt.Printf("⚠️  LLM configuration failed: %v\n", err)
		}
	} else {
		fmt.Println("Skipping LLM setup. You can configure it later by setting the LLM_PROMPT_CMD environment variable.")
	}
	fmt.Println()

	// Step 5: Shell Configuration
	fmt.Println("🐚 Step 5: Shell Configuration")
	fmt.Println("------------------------------")
	fmt.Println("Add environment variables to your shell configuration for persistence.")

	if err := configureBashrc(cfg); err != nil {
		fmt.Printf("⚠️  Shell configuration failed: %v\n", err)
	}
	fmt.Println()

	// Final summary
	fmt.Println("🎉 Setup Complete!")
	fmt.Println("==================")
	fmt.Println("Your money CLI is now configured. Try these commands:")
	fmt.Println("  money fetch      - Fetch latest account data")
	fmt.Println("  money balance    - Show account balances")
	fmt.Println("  money budget     - Show budget overview")
	fmt.Println("  money transactions - Manage transactions")
	fmt.Println()

	return nil
}

func initSimpleFinCommand(cmd *Z.Cmd, args ...string) error {
	fmt.Println("SimpleFIN Setup")
	fmt.Println("===============")
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

		if !RunConfirmation("Do you want to overwrite the existing setup?") {
			fmt.Println("Setup cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Get SimpleFIN setup token from args or prompt
	var setupToken string
	if len(args) > 0 {
		setupToken = args[0]
	} else {
		setupToken = RunInputWithValidator(
			"Enter your SimpleFIN setup token (base64 encoded)",
			"aHR0cHM6Ly9icmlkZ2Uuc2ltcGxlZmluLm9yZy9zaW1wbGVmaW4vY2xhaW0vYWJjMTIz",
			SetupTokenValidator,
		)
		if setupToken == "" {
			return fmt.Errorf("setup token is required")
		}
	}

	// No validation needed - let SimpleFIN client handle it

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

	// Extract organizations from accounts since they're now embedded
	orgMap := make(map[string]simplefin.Organization)
	for _, account := range accounts.Accounts {
		orgMap[account.Org.ID] = account.Org
	}

	// Confirm successful setup
	fmt.Println()
	fmt.Println("✅ Setup completed successfully!")
	fmt.Printf("Found %d organizations with %d total accounts\n",
		len(orgMap), len(accounts.Accounts))

	if len(orgMap) > 0 {
		fmt.Println()
		fmt.Println("Connected organizations:")
		for _, org := range orgMap {
			accountCount := 0
			for _, account := range accounts.Accounts {
				if account.Org.ID == org.ID {
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

func initRentCastCommand(cmd *Z.Cmd, args ...string) error {
	var apiKey string
	if len(args) > 0 {
		apiKey = args[0]
	} else {
		apiKey = RunInputWithValidator(
			"Enter your RentCast API key",
			"Get one from: https://developers.rentcast.io/",
			APIKeyValidator,
		)
		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}
	}

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
}


func runSimpleFinSetup(cfg *config.Config) error {
	// Set MONEY_DIR temporarily for the database
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", cfg.MoneyDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	// Get setup token
	setupToken := RunInputWithValidator(
		"Enter your SimpleFIN setup token (base64 encoded)",
		"aHR0cHM6Ly9icmlkZ2Uuc2ltcGxlZmluLm9yZy9zaW1wbGVmaW4vY2xhaW0vYWJjMTIz",
		SetupTokenValidator,
	)
	if setupToken == "" {
		return fmt.Errorf("setup token is required")
	}

	// Initialize database
	db, err := database.New()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close()

	// Check for existing credentials
	hasCredentials, err := db.HasCredentials()
	if err != nil {
		return fmt.Errorf("failed to check existing credentials: %w", err)
	}

	if hasCredentials {
		if !RunConfirmation("Existing SimpleFIN credentials found. Do you want to overwrite them?") {
			return fmt.Errorf("setup cancelled by user")
		}
	}

	// Exchange token and save credentials
	fmt.Println("Exchanging setup token for permanent credentials...")
	client, err := simplefin.NewClientFromToken(setupToken)
	if err != nil {
		return fmt.Errorf("failed to exchange setup token: %w", err)
	}

	accessURL, username, password := client.GetCredentials()
	if err := db.SaveCredentials(accessURL, username, password); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// Test connection
	fmt.Println("Testing connection...")
	accounts, err := client.GetAccounts()
	if err != nil {
		return fmt.Errorf("failed to test connection: %w", err)
	}

	fmt.Printf("✅ SimpleFIN setup successful! Found %d accounts\n", len(accounts.Accounts))
	return nil
}

func runRentCastSetup(cfg *config.Config) error {
	// Set MONEY_DIR temporarily for the database
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", cfg.MoneyDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	apiKey := RunInputWithValidator(
		"Enter your RentCast API key",
		"Get one from: https://developers.rentcast.io/",
		APIKeyValidator,
	)
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	if len(apiKey) < 10 {
		return fmt.Errorf("API key appears to be too short")
	}

	db, err := database.New()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.SaveRentCastAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save RentCast API key: %w", err)
	}

	fmt.Println("✅ RentCast setup successful!")
	return nil
}

func configureLLMInteractive(cfg *config.Config) error {
	// LLM Command Selection
	llmOptions := []SelectOption{
		{Label: "claude", Value: "claude", Description: "Anthropic Claude (default)"},
		{Label: "openai", Value: "openai", Description: "OpenAI ChatGPT"},
		{Label: "ollama", Value: "ollama", Description: "Local Ollama installation"},
		{Label: "llm", Value: "llm", Description: "Simon Willison's llm tool"},
		{Label: "custom", Value: "custom", Description: "Enter a custom command"},
		{Label: "keep current", Value: "current", Description: fmt.Sprintf("Keep current: %s", cfg.LLMPromptCmd)},
	}

	selectedLLM := RunSelection("Choose your LLM command:", llmOptions)
	if selectedLLM == nil {
		return fmt.Errorf("LLM selection cancelled")
	}

	if selectedLLM.Value == "custom" {
		customCmd := RunInput("Enter custom LLM command", "my-llm-tool")
		if customCmd != "" {
			cfg.SetLLMPromptCmd(customCmd)
			fmt.Printf("LLM command updated to: %s\n", cfg.LLMPromptCmd)
		}
	} else if selectedLLM.Value != "current" {
		cfg.SetLLMPromptCmd(selectedLLM.Value)
		fmt.Printf("LLM command updated to: %s\n", cfg.LLMPromptCmd)
	}

	// Batch Size Configuration
	batchSizeInput := RunInputWithValidator(
		fmt.Sprintf("Enter batch size for LLM categorization (current: %d)", cfg.LLMBatchSize),
		strconv.Itoa(cfg.LLMBatchSize),
		BatchSizeValidator,
	)

	if batchSizeInput != "" {
		if batchSize, err := strconv.Atoi(batchSizeInput); err == nil {
			cfg.SetLLMBatchSize(batchSize)
			fmt.Printf("LLM batch size updated to: %d\n", cfg.LLMBatchSize)
		}
	}

	return nil
}

func configureBashrc(cfg *config.Config) error {
	exports := cfg.GetBashrcExports()
	if len(exports) == 0 {
		fmt.Println("No custom environment variables to add (using all defaults).")
		return nil
	}

	fmt.Println("The following environment variables need to be added to your shell:")
	fmt.Println()
	for _, export := range exports {
		fmt.Printf("  %s\n", export)
	}
	fmt.Println()

	// Offer shell configuration options
	shellOptions := []SelectOption{
		{Label: "Auto-add to ~/.bashrc", Value: "bashrc", Description: "Automatically append to your .bashrc file"},
		{Label: "Show commands to run manually", Value: "manual", Description: "Display commands for manual configuration"},
		{Label: "Skip shell configuration", Value: "skip", Description: "Skip for now (you can configure manually later)"},
	}

	selection := RunSelection("How would you like to configure your shell?", shellOptions)
	if selection == nil || selection.Value == "skip" {
		fmt.Println("Skipping shell configuration. You can set these environment variables manually.")
		return nil
	}

	if selection.Value == "manual" {
		fmt.Println("Add these lines to your shell configuration file (~/.bashrc, ~/.zshrc, etc.):")
		fmt.Println()
		for _, export := range exports {
			fmt.Printf("  %s\n", export)
		}
		fmt.Println()
		fmt.Println("Then run 'source ~/.bashrc' or restart your terminal to apply changes.")
		return nil
	}

	// Auto-add to .bashrc
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	bashrcPath := filepath.Join(home, ".bashrc")

	// Show a detailed confirmation with safety information
	fmt.Printf("This will modify your %s file.\n", bashrcPath)
	fmt.Println("The changes will be appended to the end of the file with clear comments.")
	fmt.Println()

	if !RunConfirmation("Proceed with modifying your .bashrc file?") {
		fmt.Println("Shell configuration cancelled. You can add the variables manually.")
		return nil
	}

	// Create backup if file exists
	if _, err := os.Stat(bashrcPath); err == nil {
		backupPath := bashrcPath + ".money-backup"
		if err := copyFile(bashrcPath, backupPath); err != nil {
			fmt.Printf("⚠️  Warning: Could not create backup of .bashrc: %v\n", err)
			if !RunConfirmation("Continue without backup?") {
				return fmt.Errorf("operation cancelled by user")
			}
		} else {
			fmt.Printf("📁 Backup created: %s\n", backupPath)
		}
	}

	// Add to .bashrc
	file, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .bashrc: %w", err)
	}
	defer file.Close()

	// Add a clear comment section
	file.WriteString("\n# Money CLI configuration (added by 'money init')\n")
	file.WriteString("# You can safely remove this section if you no longer use Money CLI\n")
	for _, export := range exports {
		file.WriteString(export + "\n")
	}
	file.WriteString("# End Money CLI configuration\n")

	fmt.Printf("✅ Environment variables added to %s\n", bashrcPath)
	fmt.Println("Run 'source ~/.bashrc' or restart your terminal to apply changes.")
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	return err
}



func checkExistingSimpleFINCredentials(cfg *config.Config) (bool, error) {
	// Set MONEY_DIR temporarily for the database
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", cfg.MoneyDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	db, err := database.New()
	if err != nil {
		return false, err
	}
	defer db.Close()

	return db.HasCredentials()
}

func checkExistingRentCastCredentials(cfg *config.Config) (bool, error) {
	// Set MONEY_DIR temporarily for the database
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", cfg.MoneyDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	db, err := database.New()
	if err != nil {
		return false, err
	}
	defer db.Close()

	return db.HasRentCastAPIKey()
}
