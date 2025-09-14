package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Client handles LLM interactions using configurable external commands
type Client struct {
	promptCommand string
}

// NewClient creates a new LLM client with the configured command
func NewClient() *Client {
	// Get command from environment variable, default to ollama with llama3.2
	promptCmd := os.Getenv("LLM_PROMPT_CMD")
	if promptCmd == "" {
		// Default command uses ollama with llama3.2 model
		promptCmd = "ollama run llama3.2"
	}

	return &Client{
		promptCommand: promptCmd,
	}
}

// TransferSuggestion represents a suggested inter-account transfer
type TransferSuggestion struct {
	TransactionID string `json:"transaction_id"`
	IsTransfer    bool   `json:"is_transfer"`
	Reasoning     string `json:"reasoning"`
}

// CategorySuggestion represents a suggested category for a transaction
type CategorySuggestion struct {
	TransactionID string `json:"transaction_id"`
	Category      string `json:"category"`
	Confidence    float64 `json:"confidence"`
	Reasoning     string `json:"reasoning"`
}

// TransferAnalysisResult contains the results of transfer identification
type TransferAnalysisResult struct {
	Suggestions []TransferSuggestion `json:"suggestions"`
}

// CategoryAnalysisResult contains the results of transaction categorization
type CategoryAnalysisResult struct {
	Suggestions []CategorySuggestion `json:"suggestions"`
}

// IdentifyTransfers uses LLM to identify inter-account transfers
func (c *Client) IdentifyTransfers(ctx context.Context, transactions []TransactionData, accounts []AccountData) (*TransferAnalysisResult, error) {
	prompt := buildTransferIdentificationPrompt(transactions, accounts)

	response, err := c.runLLMCommand(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to run LLM command for transfer identification: %w", err)
	}

	var result TransferAnalysisResult
	err = json.Unmarshal([]byte(response), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response for transfers: %w", err)
	}

	return &result, nil
}

// CategorizeTransactions uses LLM to categorize transactions based on available categories
func (c *Client) CategorizeTransactions(ctx context.Context, transactions []TransactionData, categories []string) (*CategoryAnalysisResult, error) {
	prompt := buildCategorizationPrompt(transactions, categories)

	response, err := c.runLLMCommand(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to run LLM command for categorization: %w", err)
	}

	var result CategoryAnalysisResult
	err = json.Unmarshal([]byte(response), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response for categories: %w", err)
	}

	return &result, nil
}

// runLLMCommand executes the configured LLM command with the given prompt
func (c *Client) runLLMCommand(ctx context.Context, prompt string) (string, error) {
	// Create context with timeout - increased for LLM processing
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Split command and arguments
	parts := strings.Fields(c.promptCommand)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty prompt command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	// Set up stdin pipe to send the prompt
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Write prompt to stdin in a goroutine
	go func() {
		defer stdin.Close()
		stdin.Write([]byte(prompt))
	}()

	// Execute the command and get output
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute LLM command: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// TransactionData represents transaction data for LLM processing
type TransactionData struct {
	ID          string  `json:"id"`
	AccountID   string  `json:"account_id"`
	Posted      string  `json:"posted"`
	Amount      int     `json:"amount"`
	Description string  `json:"description"`
	Pending     bool    `json:"pending"`
}

// AccountData represents account data for LLM processing
type AccountData struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Nickname     string `json:"nickname,omitempty"`
	AccountType  string `json:"account_type,omitempty"`
}

// buildTransferIdentificationPrompt creates a prompt for identifying inter-account transfers
func buildTransferIdentificationPrompt(transactions []TransactionData, accounts []AccountData) string {
	var prompt strings.Builder

	prompt.WriteString(`You are a financial transaction analyzer. Your task is to identify inter-account transfers from the following transactions.

Inter-account transfers typically have these characteristics:
1. Similar amounts and timing between different accounts
2. Descriptions mentioning "transfer", "move", "deposit from", "withdrawal to"
3. Round amounts (like $100.00, $500.00)
4. Transactions that appear to move money between the user's own accounts

ACCOUNTS:
`)

	for _, account := range accounts {
		displayName := account.Name
		if account.Nickname != "" {
			displayName = account.Nickname
		}
		prompt.WriteString(fmt.Sprintf("- %s (%s) - Type: %s\n", account.ID, displayName, account.AccountType))
	}

	prompt.WriteString("\nTRANSACTIONS:\n")
	for _, tx := range transactions {
		amountDollars := float64(tx.Amount) / 100.0
		prompt.WriteString(fmt.Sprintf("ID: %s, Account: %s, Date: %s, Amount: $%.2f, Description: %s\n",
			tx.ID, tx.AccountID, tx.Posted, amountDollars, tx.Description))
	}

	prompt.WriteString(`
Please analyze these transactions and identify which ones are likely inter-account transfers.

CRITICAL: Return ONLY valid JSON with no additional text, explanations, or formatting. Do not include markdown code blocks or any other text.

Expected JSON format:
{
  "suggestions": [
    {
      "transaction_id": "tx_id_here",
      "is_transfer": true,
      "reasoning": "Explanation of why this is/isn't a transfer"
    }
  ]
}

Only include transactions where you have reasonable confidence they are transfers. Be conservative - it's better to miss some transfers than incorrectly mark regular transactions as transfers.

Return ONLY the JSON object above with no additional text:`)

	return prompt.String()
}

// buildCategorizationPrompt creates a prompt for categorizing transactions
func buildCategorizationPrompt(transactions []TransactionData, categories []string) string {
	var prompt strings.Builder

	prompt.WriteString(`You are a financial transaction categorizer. Your task is to categorize the following transactions using only the provided categories.

AVAILABLE CATEGORIES:
`)
	for _, category := range categories {
		prompt.WriteString(fmt.Sprintf("- %s\n", category))
	}

	prompt.WriteString("\nTRANSACTIONS TO CATEGORIZE:\n")
	for _, tx := range transactions {
		amountDollars := float64(tx.Amount) / 100.0
		prompt.WriteString(fmt.Sprintf("ID: %s, Account: %s, Date: %s, Amount: $%.2f, Description: %s\n",
			tx.ID, tx.AccountID, tx.Posted, amountDollars, tx.Description))
	}

	prompt.WriteString(`
Please categorize each transaction using ONLY the categories listed above. Consider:
1. Transaction description and merchant names
2. Transaction amounts (positive = income, negative = expense)
3. Common spending patterns and merchant types

CRITICAL: Return ONLY valid JSON with no additional text, explanations, or formatting. Do not include markdown code blocks or any other text.

Expected JSON format:
{
  "suggestions": [
    {
      "transaction_id": "tx_id_here",
      "category": "Category Name",
      "confidence": 0.85,
      "reasoning": "Brief explanation of categorization choice"
    }
  ]
}

Use confidence scores from 0.0 to 1.0 where:
- 0.8+ = Very confident (obvious categorization)
- 0.6-0.8 = Moderately confident (good match based on description)
- 0.4-0.6 = Low confidence (best guess from available categories)
- Below 0.4 = Don't suggest a category

Only suggest categories you're reasonably confident about (0.4+).

Return ONLY the JSON object above with no additional text:`)

	return prompt.String()
}

// PromptForApproval interactively asks user to approve suggestions
func PromptForApproval(message string) (bool, error) {
	fmt.Print(message + " (y/N): ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return response == "y" || response == "yes", nil
	}
	return false, scanner.Err()
}