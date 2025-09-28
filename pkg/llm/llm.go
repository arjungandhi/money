package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/arjungandhi/money/pkg/database"
)

// Client handles LLM interactions using configurable external commands
type Client struct {
	promptCommand string
}

// NewClient creates a new LLM client with the configured command
func NewClient() *Client {
	promptCmd := os.Getenv("LLM_PROMPT_CMD")
	if promptCmd == "" {
		promptCmd = "claude"
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
	TransactionID string  `json:"transaction_id"`
	Category      string  `json:"category"`
	Confidence    float64 `json:"confidence"`
	Reasoning     string  `json:"reasoning"`
}

// TransferAnalysisResult contains the results of transfer identification
type TransferAnalysisResult struct {
	Suggestions []TransferSuggestion `json:"suggestions"`
}

// CategoryAnalysisResult contains the results of transaction categorization
type CategoryAnalysisResult struct {
	Suggestions []CategorySuggestion `json:"suggestions"`
}

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

func (c *Client) CategorizeTransactions(ctx context.Context, transactions []TransactionData, categories []string) (*CategoryAnalysisResult, error) {
	// Convert string categories to database.Category structs (all as regular categories)
	categoryStructs := make([]database.Category, len(categories))
	for i, cat := range categories {
		categoryStructs[i] = database.Category{
			Name:       cat,
			IsInternal: false,
		}
	}
	return c.CategorizeTransactionsWithExamples(ctx, transactions, categoryStructs, nil, nil)
}

func (c *Client) CategorizeTransactionsWithExamples(ctx context.Context, transactions []TransactionData, categories []database.Category, accounts []AccountData, examples []CategorizedExample) (*CategoryAnalysisResult, error) {
	prompt := buildCategorizationPrompt(transactions, categories, accounts, examples)

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
	// No timeout - let LLM take as long as needed

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
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	Posted      string `json:"posted"`
	Amount      int    `json:"amount"`
	Description string `json:"description"`
	Pending     bool   `json:"pending"`
}

// CategorizedExample represents an example of a previously categorized transaction
type CategorizedExample struct {
	Description string `json:"description"`
	Amount      int    `json:"amount"`
	Category    string `json:"category"`
}

// AccountData represents account data for LLM processing
type AccountData struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Nickname    string `json:"nickname,omitempty"`
	AccountType string `json:"account_type,omitempty"`
}

// buildTransferIdentificationPrompt creates a prompt for identifying inter-account transfers
func buildTransferIdentificationPrompt(transactions []TransactionData, accounts []AccountData) string {
	var prompt strings.Builder

	prompt.WriteString(`You are a financial transaction analyzer. Your task is to identify inter-account transfers from the following transactions.

WHAT TO LOOK FOR:
Inter-account transfers are pairs of transactions that move money between the user's own accounts. Look for:

1. MATCHING PAIRS: Two transactions with opposite amounts (+$X and -$X) on the same date or within 1-2 days
2. TRANSFER DESCRIPTIONS: Keywords like "transfer", "move", "deposit from", "withdrawal to", "transfer to savings", "transfer from checking"
3. INTERNAL MOVEMENT: Money moving between accounts the user owns (not payments to external merchants)
4. ROUND AMOUNTS: Often round numbers like $100.00, $500.00, $1000.00

EXAMPLES OF TRANSFERS:
- Transaction A: -$500.00 "Transfer to Savings" (from checking)
- Transaction B: +$500.00 "Transfer from Checking" (to savings)
→ Both should be marked as transfers

EXAMPLES OF NON-TRANSFERS:
- "Starbucks Coffee" -$5.50 → Regular merchant purchase
- "Salary Deposit" +$2500.00 → Income from employer
- "Electric Bill" -$120.00 → Payment to utility company

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
ANALYSIS TASK:
Examine each transaction above and determine if it's an inter-account transfer. For each transaction you identify as a transfer, include it in the JSON response.

IMPORTANT RULES:
- Only return transactions that are CLEARLY transfers (descriptions with "transfer", matching amounts between accounts, etc.)
- Be conservative - it's better to miss some transfers than mark regular purchases as transfers
- If unsure, don't include the transaction
- Look for PAIRS of transactions that match (one negative, one positive with same amount)

CRITICAL: Return ONLY raw JSON with no additional text, explanations, markdown formatting, or code blocks. Do not wrap in code blocks. Return the JSON object directly.

Required JSON format:
{
  "suggestions": [
    {
      "transaction_id": "tx1",
      "is_transfer": true,
      "reasoning": "Transfer to Savings - clear transfer description with matching amount"
    },
    {
      "transaction_id": "tx2",
      "is_transfer": true,
      "reasoning": "Transfer from Checking - matches the previous transfer amount"
    }
  ]
}

Return ONLY the raw JSON object with no markdown formatting:`)

	return prompt.String()
}

// buildCategorizationPrompt creates a prompt for categorizing transactions
func buildCategorizationPrompt(transactions []TransactionData, categories []database.Category, accounts []AccountData, examples []CategorizedExample) string {
	var prompt strings.Builder

	prompt.WriteString(`You are a financial transaction categorizer. Your task is to categorize transactions using ONLY the provided categories.

CATEGORIZATION RULES:
1. Use EXACT category names from the list below - no variations or modifications
2. Match based on merchant names, transaction descriptions, and amount patterns
3. Positive amounts = Income, Negative amounts = Expenses
4. Be specific: "Starbucks" = Dining Out, "Whole Foods" = Groceries, "Shell Gas" = Transportation
5. For inter-account transfers, use internal categories (like "Transfers")

`)

	// Separate regular and internal categories
	var regularCategories []string
	var internalCategories []string
	for _, category := range categories {
		if category.IsInternal {
			internalCategories = append(internalCategories, category.Name)
		} else {
			regularCategories = append(regularCategories, category.Name)
		}
	}

	// Display categories with clear labeling
	if len(regularCategories) > 0 {
		prompt.WriteString("REGULAR CATEGORIES (for income/expenses):\n")
		for _, category := range regularCategories {
			prompt.WriteString(fmt.Sprintf("- %s\n", category))
		}
		prompt.WriteString("\n")
	}

	if len(internalCategories) > 0 {
		prompt.WriteString("INTERNAL CATEGORIES (for transfers between your own accounts):\n")
		for _, category := range internalCategories {
			prompt.WriteString(fmt.Sprintf("- %s\n", category))
		}
		prompt.WriteString("\n")
	}

	// Add account information for transfer detection
	if len(accounts) > 0 {
		prompt.WriteString("YOUR ACCOUNTS:\n")
		for _, account := range accounts {
			displayName := account.Name
			if account.Nickname != "" {
				displayName = account.Nickname
			}
			prompt.WriteString(fmt.Sprintf("- %s (%s) - Type: %s\n", account.ID, displayName, account.AccountType))
		}
		prompt.WriteString("\n")
	}

	// Add examples section if provided
	if len(examples) > 0 {
		prompt.WriteString("\nCATEGORIZATION EXAMPLES:\n")
		prompt.WriteString("Here are examples of how similar transactions have been categorized:\n\n")
		for _, example := range examples {
			amountDollars := float64(example.Amount) / 100.0
			prompt.WriteString(fmt.Sprintf("- Description: \"%s\", Amount: $%.2f → Category: \"%s\"\n",
				example.Description, amountDollars, example.Category))
		}
		prompt.WriteString("\nUse these examples to guide your categorization decisions.\n")
	}

	prompt.WriteString("\nTRANSACTIONS TO CATEGORIZE:\n")
	for _, tx := range transactions {
		amountDollars := float64(tx.Amount) / 100.0
		prompt.WriteString(fmt.Sprintf("ID: %s, Account: %s, Date: %s, Amount: $%.2f, Description: %s\n",
			tx.ID, tx.AccountID, tx.Posted, amountDollars, tx.Description))
	}

	prompt.WriteString(`
ANALYSIS TASK:
For each transaction above, determine the most appropriate category from the AVAILABLE CATEGORIES list.

MATCHING GUIDELINES:
- Food/Coffee shops (Starbucks, McDonald's, etc.) → "Dining Out"
- Grocery stores (Whole Foods, Safeway, etc.) → "Groceries"
- Gas stations (Shell, Exxon, etc.) → "Transportation"
- Salary/Paycheck deposits → "Income"
- Utility companies (PG&E, Comcast, etc.) → "Bills & Services"
- Retail stores (Target, Amazon, etc.) → "Shopping"

TRANSFER DETECTION:
Look for transactions that move money between the user's own accounts:
- Descriptions containing "transfer", "move", "deposit from", "withdrawal to"
- Matching amounts (+$X and -$X) on same/similar dates
- Movement between accounts listed above → Use internal categories (like "Transfers")

CONFIDENCE SCORING:
- 0.8+ = Very confident (obvious match like "Starbucks Coffee" → Dining Out)
- 0.6-0.8 = Moderately confident (reasonable match based on merchant)
- 0.4-0.6 = Low confidence (best guess from available categories)
- Below 0.4 = Don't suggest (skip the transaction)

CRITICAL: Return ONLY raw JSON with no additional text, explanations, markdown formatting, or code blocks. Do not wrap in code blocks. Return the JSON object directly.

Required JSON format:
{
  "suggestions": [
    {
      "transaction_id": "tx3",
      "category": "Dining Out",
      "confidence": 0.95,
      "reasoning": "Starbucks Coffee - clear coffee shop transaction"
    },
    {
      "transaction_id": "tx4",
      "category": "Groceries",
      "confidence": 0.90,
      "reasoning": "Whole Foods Market - grocery store purchase"
    }
  ]
}

Return ONLY the raw JSON object with no markdown formatting:`)

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
