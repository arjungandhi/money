package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client handles LLM interactions using configurable external commands
type Client struct {
	promptCommand string
	batchSize     int // Fixed batch size for processing
}

// NewClient creates a new LLM client with the configured command
func NewClient() *Client {
	// Get command from environment variable, default to claude
	promptCmd := os.Getenv("LLM_PROMPT_CMD")
	if promptCmd == "" {
		// Default command uses claude
		promptCmd = "claude"
	}

	// Simple fixed batch size (default 100)
	batchSize := 100
	if envBatch := os.Getenv("LLM_BATCH_SIZE"); envBatch != "" {
		if batch, err := fmt.Sscanf(envBatch, "%d", &batchSize); err == nil && batch == 1 {
			// Successfully parsed
		}
	}

	return &Client{
		promptCommand: promptCmd,
		batchSize:     batchSize,
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
	// Handle large transaction sets with batching
	batchSize := c.calculateOptimalBatchSize(transactions, "transfer")

	if len(transactions) <= batchSize {
		// Small enough to process in one batch
		return c.identifyTransfersBatch(ctx, transactions, accounts)
	}

	// Process in overlapping batches for transfer identification
	overlap := c.calculateOverlapSize(batchSize)
	step := batchSize - overlap
	fmt.Printf("Processing %d transactions in overlapping batches of %d (overlap: %d)...\n", len(transactions), batchSize, overlap)

	var allSuggestions []TransferSuggestion
	processedTxIDs := make(map[string]bool) // Track processed transactions to avoid duplicates

	for i := 0; i < len(transactions); i += step {
		end := i + batchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		batch := transactions[i:end]
		batchNum := (i / step) + 1
		totalBatches := ((len(transactions) - 1) / step) + 1
		fmt.Printf("📊 Processing batch %d/%d (%d transactions, starting from #%d)...\n",
			batchNum, totalBatches, len(batch), i+1)

		result, err := c.identifyTransfersBatch(ctx, batch, accounts)
		if err != nil {
			return nil, fmt.Errorf("failed to process transfer batch %d: %w", batchNum, err)
		}

		// Add suggestions, avoiding duplicates from overlapping batches
		for _, suggestion := range result.Suggestions {
			if !processedTxIDs[suggestion.TransactionID] {
				allSuggestions = append(allSuggestions, suggestion)
				processedTxIDs[suggestion.TransactionID] = true
			}
		}

		// Break if we've processed all transactions
		if end >= len(transactions) {
			break
		}
	}

	return &TransferAnalysisResult{Suggestions: allSuggestions}, nil
}

// identifyTransfersBatch processes a single batch of transactions for transfer identification
func (c *Client) identifyTransfersBatch(ctx context.Context, transactions []TransactionData, accounts []AccountData) (*TransferAnalysisResult, error) {
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
	// Handle large transaction sets with batching
	batchSize := c.calculateOptimalBatchSize(transactions, "categorize")

	if len(transactions) <= batchSize {
		// Small enough to process in one batch
		return c.categorizeTransactionsBatch(ctx, transactions, categories)
	}

	// Process in batches
	fmt.Printf("Processing %d transactions in batches of %d...\n", len(transactions), batchSize)

	var allSuggestions []CategorySuggestion

	for i := 0; i < len(transactions); i += batchSize {
		end := i + batchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		batch := transactions[i:end]
		fmt.Printf("📊 Processing batch %d/%d (%d transactions)...\n",
			(i/batchSize)+1, (len(transactions)+batchSize-1)/batchSize, len(batch))

		result, err := c.categorizeTransactionsBatch(ctx, batch, categories)
		if err != nil {
			fmt.Printf("❌ Batch %d failed: %v\n", (i/batchSize)+1, err)
			fmt.Printf("   Skipping %d transactions in this batch\n", len(batch))
			continue // Skip failed batches instead of failing completely
		}

		allSuggestions = append(allSuggestions, result.Suggestions...)
	}

	return &CategoryAnalysisResult{Suggestions: allSuggestions}, nil
}

// categorizeTransactionsBatch processes a single batch of transactions for categorization
func (c *Client) categorizeTransactionsBatch(ctx context.Context, transactions []TransactionData, categories []string) (*CategoryAnalysisResult, error) {
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
func buildCategorizationPrompt(transactions []TransactionData, categories []string) string {
	var prompt strings.Builder

	prompt.WriteString(`You are a financial transaction categorizer. Your task is to categorize transactions using ONLY the provided categories.

CATEGORIZATION RULES:
1. Use EXACT category names from the list below - no variations or modifications
2. Match based on merchant names, transaction descriptions, and amount patterns
3. Positive amounts = Income, Negative amounts = Expenses
4. Be specific: "Starbucks" = Dining Out, "Whole Foods" = Groceries, "Shell Gas" = Transportation

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
ANALYSIS TASK:
For each transaction above, determine the most appropriate category from the AVAILABLE CATEGORIES list.

MATCHING GUIDELINES:
- Food/Coffee shops (Starbucks, McDonald's, etc.) → "Dining Out"
- Grocery stores (Whole Foods, Safeway, etc.) → "Groceries"
- Gas stations (Shell, Exxon, etc.) → "Transportation"
- Salary/Paycheck deposits → "Income"
- Utility companies (PG&E, Comcast, etc.) → "Bills & Services"
- Retail stores (Target, Amazon, etc.) → "Shopping"

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

// calculateOptimalBatchSize returns the fixed batch size
func (c *Client) calculateOptimalBatchSize(transactions []TransactionData, operation string) int {
	return c.batchSize
}

// calculateOverlapSize determines how much batches should overlap for transfer identification
func (c *Client) calculateOverlapSize(batchSize int) int {
	// For transfer identification, we want some overlap to catch transfer pairs
	// that might be split across batch boundaries
	overlap := batchSize / 5 // 20% overlap - smaller for memory efficiency
	if overlap < 2 {
		overlap = 2
	}
	if overlap >= batchSize {
		overlap = batchSize - 1 // Ensure overlap is less than batch size
	}
	if overlap > 5 {
		overlap = 5 // Cap at 5 transactions for memory efficiency
	}
	return overlap
}