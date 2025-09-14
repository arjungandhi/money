package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	conn *sql.DB
}

func New() (*DB, error) {
	// 1. Get MONEY_DIR from env (default: $HOME/.money)
	moneyDir := getMoneyDir()

	// 2. Create directory if it doesn't exist
	if err := os.MkdirAll(moneyDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create money directory: %w", err)
	}

	// 3. Open SQLite connection
	dbPath := filepath.Join(moneyDir, "money.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// 4. Run schema migrations
	if err := db.runMigrations(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

func (db *DB) runMigrations() error {
	// Check if tables already exist by querying sqlite_master
	var tableCount int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to check existing tables: %w", err)
	}

	// Only run migrations if no tables exist (fresh database)
	if tableCount == 0 {
		_, err = db.conn.Exec(schemaSQL)
		if err != nil {
			return fmt.Errorf("failed to execute schema: %w", err)
		}
	} else {
		// Run incremental migrations for existing databases
		err = db.runIncrementalMigrations()
		if err != nil {
			return fmt.Errorf("failed to run incremental migrations: %w", err)
		}
	}

	return nil
}

func (db *DB) runIncrementalMigrations() error {
	// Check if account_type column exists
	var columnExists int
	err := db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('accounts')
		WHERE name = 'account_type'
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("failed to check account_type column: %w", err)
	}

	// Add account_type column if it doesn't exist
	if columnExists == 0 {
		_, err = db.conn.Exec(`
			ALTER TABLE accounts
			ADD COLUMN account_type TEXT CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'loan', 'other'))
		`)
		if err != nil {
			return fmt.Errorf("failed to add account_type column: %w", err)
		}
	}

	// Check if is_transfer column exists in transactions table
	var transferColumnExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('transactions')
		WHERE name = 'is_transfer'
	`).Scan(&transferColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check is_transfer column: %w", err)
	}

	// Add is_transfer column if it doesn't exist
	if transferColumnExists == 0 {
		_, err = db.conn.Exec(`
			ALTER TABLE transactions
			ADD COLUMN is_transfer BOOLEAN DEFAULT FALSE
		`)
		if err != nil {
			return fmt.Errorf("failed to add is_transfer column: %w", err)
		}
	}

	// Check if type column exists in categories table and remove it if it does
	var categoryTypeExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('categories')
		WHERE name = 'type'
	`).Scan(&categoryTypeExists)
	if err != nil {
		return fmt.Errorf("failed to check categories type column: %w", err)
	}

	// Remove type column from categories if it exists (SQLite doesn't support DROP COLUMN easily)
	// We'll need to recreate the table without the type column
	if categoryTypeExists > 0 {
		// Create a new categories table without the type column
		_, err = db.conn.Exec(`
			CREATE TABLE categories_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create new categories table: %w", err)
		}

		// Copy data from old table to new (without type column)
		_, err = db.conn.Exec(`
			INSERT INTO categories_new (id, name, created_at)
			SELECT id, name, created_at FROM categories
		`)
		if err != nil {
			return fmt.Errorf("failed to copy categories data: %w", err)
		}

		// Drop old table and rename new one
		_, err = db.conn.Exec(`DROP TABLE categories`)
		if err != nil {
			return fmt.Errorf("failed to drop old categories table: %w", err)
		}

		_, err = db.conn.Exec(`ALTER TABLE categories_new RENAME TO categories`)
		if err != nil {
			return fmt.Errorf("failed to rename new categories table: %w", err)
		}
	}

	// Check if nickname column exists in accounts table
	var nicknameColumnExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('accounts')
		WHERE name = 'nickname'
	`).Scan(&nicknameColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check nickname column: %w", err)
	}

	// Add nickname column if it doesn't exist
	if nicknameColumnExists == 0 {
		_, err = db.conn.Exec(`
			ALTER TABLE accounts
			ADD COLUMN nickname TEXT
		`)
		if err != nil {
			return fmt.Errorf("failed to add nickname column: %w", err)
		}
	}

	// Check if balance_history table exists
	var tableExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type='table' AND name='balance_history'
	`).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("failed to check balance_history table: %w", err)
	}

	// Create balance_history table if it doesn't exist
	if tableExists == 0 {
		_, err = db.conn.Exec(`
			CREATE TABLE balance_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				account_id TEXT NOT NULL,
				balance INTEGER NOT NULL,
				available_balance INTEGER,
				recorded_at DATETIME NOT NULL,
				FOREIGN KEY (account_id) REFERENCES accounts(id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create balance_history table: %w", err)
		}

		// Create indexes for balance_history
		_, err = db.conn.Exec(`
			CREATE INDEX idx_balance_history_account_id ON balance_history(account_id);
		`)
		if err != nil {
			return fmt.Errorf("failed to create balance_history account_id index: %w", err)
		}

		_, err = db.conn.Exec(`
			CREATE INDEX idx_balance_history_recorded_at ON balance_history(recorded_at);
		`)
		if err != nil {
			return fmt.Errorf("failed to create balance_history recorded_at index: %w", err)
		}
	}

	return nil
}

func getMoneyDir() string {
	if dir := os.Getenv("MONEY_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".money")
}

// Credential methods
func (db *DB) SaveCredentials(accessURL, username, password string) error {
	// Delete any existing credentials (only one set allowed)
	_, err := db.conn.Exec("DELETE FROM credentials")
	if err != nil {
		return fmt.Errorf("failed to clear existing credentials: %w", err)
	}

	// Insert new credentials
	_, err = db.conn.Exec(`
		INSERT INTO credentials (access_url, username, password, last_used) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		accessURL, username, password)
	if err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	return nil
}

func (db *DB) GetCredentials() (accessURL, username, password string, err error) {
	err = db.conn.QueryRow(`
		SELECT access_url, username, password 
		FROM credentials 
		ORDER BY created_at DESC 
		LIMIT 1`).Scan(&accessURL, &username, &password)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", fmt.Errorf("no credentials found - run 'money init' first")
		}
		return "", "", "", fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	// Update last_used timestamp
	_, updateErr := db.conn.Exec("UPDATE credentials SET last_used = CURRENT_TIMESTAMP WHERE access_url = ?", accessURL)
	if updateErr != nil {
		// Log warning but don't fail the operation
		fmt.Printf("Warning: failed to update last_used timestamp: %v\n", updateErr)
	}

	return accessURL, username, password, nil
}

func (db *DB) HasCredentials() (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM credentials").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check credentials: %w", err)
	}
	return count > 0, nil
}

// Organization methods
func (db *DB) SaveOrganization(id, name, url string) error {
	// Use INSERT OR REPLACE to handle both new and existing organizations
	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO organizations (id, name, url)
		VALUES (?, ?, ?)`,
		id, name, sql.NullString{String: url, Valid: url != ""})
	if err != nil {
		return fmt.Errorf("failed to save organization: %w", err)
	}
	return nil
}

func (db *DB) GetOrganizations() ([]Organization, error) {
	query := `
		SELECT id, name, url
		FROM organizations
		ORDER BY name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query organizations: %w", err)
	}
	defer rows.Close()

	var orgs []Organization
	for rows.Next() {
		var org Organization
		var url sql.NullString

		err := rows.Scan(&org.ID, &org.Name, &url)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}

		if url.Valid {
			org.URL = &url.String
		}

		orgs = append(orgs, org)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating organizations: %w", err)
	}

	return orgs, nil
}

// Account methods
func (db *DB) SaveAccount(id, orgID, name, currency string, balance int, availableBalance *int, balanceDate string) error {
	// Use INSERT OR REPLACE to handle both new and existing accounts
	// Update the updated_at timestamp for existing accounts
	var availableBalanceVal sql.NullInt64
	if availableBalance != nil {
		availableBalanceVal = sql.NullInt64{Int64: int64(*availableBalance), Valid: true}
	}
	
	// Use INSERT OR IGNORE first, then UPDATE to preserve account_type
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO accounts (id, org_id, name, currency, balance, available_balance, balance_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, orgID, name, currency, balance, availableBalanceVal,
		sql.NullString{String: balanceDate, Valid: balanceDate != ""})
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}
	
	// Now update existing records (preserves account_type if already set)
	_, err = db.conn.Exec(`
		UPDATE accounts 
		SET org_id = ?, name = ?, currency = ?, balance = ?, available_balance = ?, balance_date = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		orgID, name, currency, balance, availableBalanceVal,
		sql.NullString{String: balanceDate, Valid: balanceDate != ""}, id)
	if err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}
	return nil
}

func (db *DB) GetAccounts() ([]Account, error) {
	query := `
		SELECT a.id, a.org_id, a.name, a.nickname, a.currency, a.balance, a.available_balance, a.balance_date, a.account_type
		FROM accounts a
		ORDER BY a.org_id, a.name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var account Account
		var nickname sql.NullString
		var availableBalance sql.NullInt64
		var balanceDate sql.NullString
		var accountType sql.NullString

		err := rows.Scan(
			&account.ID,
			&account.OrgID,
			&account.Name,
			&nickname,
			&account.Currency,
			&account.Balance,
			&availableBalance,
			&balanceDate,
			&accountType,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan account: %w", err)
		}

		// Handle nullable fields
		if nickname.Valid {
			account.Nickname = &nickname.String
		}
		if availableBalance.Valid {
			balance := int(availableBalance.Int64)
			account.AvailableBalance = &balance
		}
		if balanceDate.Valid {
			account.BalanceDate = &balanceDate.String
		}
		if accountType.Valid {
			account.AccountType = &accountType.String
		}

		accounts = append(accounts, account)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating accounts: %w", err)
	}

	return accounts, nil
}

// Account type methods
func (db *DB) SetAccountType(accountID, accountType string) error {
	// Validate account type
	validTypes := []string{"checking", "savings", "credit", "investment", "loan", "other"}
	isValid := false
	for _, validType := range validTypes {
		if accountType == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid account type: %s. Valid types are: %v", accountType, validTypes)
	}

	_, err := db.conn.Exec(`
		UPDATE accounts 
		SET account_type = ?, updated_at = CURRENT_TIMESTAMP 
		WHERE id = ?`,
		accountType, accountID)
	if err != nil {
		return fmt.Errorf("failed to set account type: %w", err)
	}
	return nil
}

func (db *DB) ClearAccountType(accountID string) error {
	_, err := db.conn.Exec(`
		UPDATE accounts
		SET account_type = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		accountID)
	if err != nil {
		return fmt.Errorf("failed to clear account type: %w", err)
	}
	return nil
}

// Account nickname methods
func (db *DB) SetAccountNickname(accountID, nickname string) error {
	_, err := db.conn.Exec(`
		UPDATE accounts
		SET nickname = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		nickname, accountID)
	if err != nil {
		return fmt.Errorf("failed to set account nickname: %w", err)
	}
	return nil
}

func (db *DB) ClearAccountNickname(accountID string) error {
	_, err := db.conn.Exec(`
		UPDATE accounts
		SET nickname = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		accountID)
	if err != nil {
		return fmt.Errorf("failed to clear account nickname: %w", err)
	}
	return nil
}

func (db *DB) GetAccountByID(accountID string) (*Account, error) {
	query := `
		SELECT a.id, a.org_id, a.name, a.nickname, a.currency, a.balance, a.available_balance, a.balance_date, a.account_type
		FROM accounts a
		WHERE a.id = ?`

	var account Account
	var nickname sql.NullString
	var availableBalance sql.NullInt64
	var balanceDate sql.NullString
	var accountType sql.NullString

	err := db.conn.QueryRow(query, accountID).Scan(
		&account.ID,
		&account.OrgID,
		&account.Name,
		&nickname,
		&account.Currency,
		&account.Balance,
		&availableBalance,
		&balanceDate,
		&accountType,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found: %s", accountID)
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	// Handle nullable fields
	if nickname.Valid {
		account.Nickname = &nickname.String
	}
	if availableBalance.Valid {
		balance := int(availableBalance.Int64)
		account.AvailableBalance = &balance
	}
	if balanceDate.Valid {
		account.BalanceDate = &balanceDate.String
	}
	if accountType.Valid {
		account.AccountType = &accountType.String
	}

	return &account, nil
}

// Transaction methods
func (db *DB) SaveTransaction(id, accountID, posted string, amount int, description string, pending bool) error {
	// Use INSERT OR IGNORE to avoid duplicate transactions
	// If the transaction already exists, we don't update it to preserve any manual categorization
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO transactions (id, account_id, posted, amount, description, pending, is_transfer)
		VALUES (?, ?, ?, ?, ?, ?, FALSE)`,
		id, accountID, posted, amount, description, pending)
	if err != nil {
		return fmt.Errorf("failed to save transaction: %w", err)
	}
	return nil
}

func (db *DB) GetTransactions(accountID string, startDate, endDate string) ([]Transaction, error) {
	var query string
	var args []interface{}

	if accountID != "" {
		if startDate != "" && endDate != "" {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, COALESCE(t.is_transfer, FALSE), t.category_id
				FROM transactions t
				WHERE t.account_id = ? AND t.posted >= ? AND t.posted <= ?
				ORDER BY t.posted DESC`
			args = []interface{}{accountID, startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, COALESCE(t.is_transfer, FALSE), t.category_id
				FROM transactions t
				WHERE t.account_id = ?
				ORDER BY t.posted DESC`
			args = []interface{}{accountID}
		}
	} else {
		if startDate != "" && endDate != "" {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, COALESCE(t.is_transfer, FALSE), t.category_id
				FROM transactions t
				WHERE t.posted >= ? AND t.posted <= ?
				ORDER BY t.posted DESC`
			args = []interface{}{startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, COALESCE(t.is_transfer, FALSE), t.category_id
				FROM transactions t
				ORDER BY t.posted DESC`
			args = []interface{}{}
		}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var categoryID sql.NullInt64

		err := rows.Scan(
			&t.ID,
			&t.AccountID,
			&t.Posted,
			&t.Amount,
			&t.Description,
			&t.Pending,
			&t.IsTransfer,
			&categoryID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		if categoryID.Valid {
			catID := int(categoryID.Int64)
			t.CategoryID = &catID
		}

		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return transactions, nil
}

func (db *DB) GetUncategorizedTransactions() ([]Transaction, error) {
	query := `
		SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, COALESCE(t.is_transfer, FALSE), t.category_id
		FROM transactions t
		WHERE t.category_id IS NULL
		ORDER BY t.posted DESC`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query uncategorized transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var categoryID sql.NullInt64

		err := rows.Scan(
			&t.ID,
			&t.AccountID,
			&t.Posted,
			&t.Amount,
			&t.Description,
			&t.Pending,
			&t.IsTransfer,
			&categoryID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan uncategorized transaction: %w", err)
		}

		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating uncategorized transactions: %w", err)
	}

	return transactions, nil
}

func (db *DB) UpdateTransactionCategory(transactionID string, categoryID int) error {
	_, err := db.conn.Exec(`
		UPDATE transactions
		SET category_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		categoryID, transactionID)
	if err != nil {
		return fmt.Errorf("failed to update transaction category: %w", err)
	}
	return nil
}

func (db *DB) ClearTransactionCategory(transactionID string) error {
	_, err := db.conn.Exec(`
		UPDATE transactions
		SET category_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		transactionID)
	if err != nil {
		return fmt.Errorf("failed to clear transaction category: %w", err)
	}
	return nil
}

// TransactionExists checks if a transaction with the given ID already exists
func (db *DB) TransactionExists(id string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM transactions WHERE id = ?", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check transaction existence: %w", err)
	}
	return count > 0, nil
}

// Category methods
func (db *DB) SaveCategory(name string) (int, error) {
	// Use INSERT OR IGNORE to avoid duplicate categories, then get the ID
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO categories (name)
		VALUES (?)`,
		name)
	if err != nil {
		return 0, fmt.Errorf("failed to insert category: %w", err)
	}

	// Get the category ID
	var id int
	err = db.conn.QueryRow(`
		SELECT id FROM categories
		WHERE name = ?`,
		name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get category ID: %w", err)
	}

	return id, nil
}

func (db *DB) GetCategories() ([]Category, error) {
	query := `
		SELECT id, name
		FROM categories
		ORDER BY name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		err := rows.Scan(&c.ID, &c.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

func (db *DB) GetCategoryByID(categoryID int) (*Category, error) {
	var c Category
	err := db.conn.QueryRow(`
		SELECT id, name
		FROM categories
		WHERE id = ?`,
		categoryID).Scan(&c.ID, &c.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("category not found: %d", categoryID)
		}
		return nil, fmt.Errorf("failed to get category: %w", err)
	}
	return &c, nil
}

func (db *DB) DeleteCategory(name string) error {
	// Check if category is used by any transactions
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM transactions
		WHERE category_id = (SELECT id FROM categories WHERE name = ?)`,
		name).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("cannot delete category '%s': it is used by %d transactions", name, count)
	}

	// Delete the category
	result, err := db.conn.Exec(`DELETE FROM categories WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found: %s", name)
	}

	return nil
}

func (db *DB) SeedDefaultCategories() error {
	defaultCategories := []string{
		"Housing",
		"Transportation",
		"Groceries",
		"Dining Out",
		"Healthcare",
		"Shopping",
		"Entertainment",
		"Bills & Services",
		"Personal Care",
		"Travel",
		"Fees",
		"Projects",
		"Subscriptions",
		"Income",
		"Other",
	}

	for _, categoryName := range defaultCategories {
		_, err := db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to seed category '%s': %w", categoryName, err)
		}
	}

	return nil
}

func (db *DB) MarkTransactionAsTransfer(transactionID string) error {
	result, err := db.conn.Exec(`
		UPDATE transactions
		SET is_transfer = TRUE, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		transactionID)
	if err != nil {
		return fmt.Errorf("failed to mark transaction as transfer: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("transaction not found: %s", transactionID)
	}

	return nil
}

// Balance History methods
func (db *DB) SaveBalanceHistory(accountID string, balance int, availableBalance *int) error {
	var availableBalanceVal sql.NullInt64
	if availableBalance != nil {
		availableBalanceVal = sql.NullInt64{Int64: int64(*availableBalance), Valid: true}
	}

	_, err := db.conn.Exec(`
		INSERT INTO balance_history (account_id, balance, available_balance, recorded_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		accountID, balance, availableBalanceVal)
	if err != nil {
		return fmt.Errorf("failed to save balance history: %w", err)
	}
	return nil
}

func (db *DB) GetBalanceHistory(accountID string, days int) ([]BalanceHistory, error) {
	query := `
		SELECT id, account_id, balance, available_balance, recorded_at
		FROM balance_history
		WHERE account_id = ? AND recorded_at >= datetime('now', '-' || ? || ' days')
		ORDER BY recorded_at ASC`

	rows, err := db.conn.Query(query, accountID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance history: %w", err)
	}
	defer rows.Close()

	var history []BalanceHistory
	for rows.Next() {
		var bh BalanceHistory
		var availableBalance sql.NullInt64

		err := rows.Scan(&bh.ID, &bh.AccountID, &bh.Balance, &availableBalance, &bh.RecordedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan balance history: %w", err)
		}

		if availableBalance.Valid {
			balance := int(availableBalance.Int64)
			bh.AvailableBalance = &balance
		}

		history = append(history, bh)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating balance history: %w", err)
	}

	return history, nil
}

func (db *DB) GetAllBalanceHistory(days int) ([]BalanceHistory, error) {
	query := `
		SELECT id, account_id, balance, available_balance, recorded_at
		FROM balance_history
		WHERE recorded_at >= datetime('now', '-' || ? || ' days')
		ORDER BY recorded_at ASC`

	rows, err := db.conn.Query(query, days)
	if err != nil {
		return nil, fmt.Errorf("failed to query all balance history: %w", err)
	}
	defer rows.Close()

	var history []BalanceHistory
	for rows.Next() {
		var bh BalanceHistory
		var availableBalance sql.NullInt64

		err := rows.Scan(&bh.ID, &bh.AccountID, &bh.Balance, &availableBalance, &bh.RecordedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan balance history: %w", err)
		}

		if availableBalance.Valid {
			balance := int(availableBalance.Int64)
			bh.AvailableBalance = &balance
		}

		history = append(history, bh)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating all balance history: %w", err)
	}

	return history, nil
}

// Data types
type Account struct {
	ID               string
	OrgID            string
	Name             string
	Nickname         *string
	Currency         string
	Balance          int
	AvailableBalance *int
	BalanceDate      *string
	AccountType      *string
}

// DisplayName returns the nickname if set, otherwise returns the original name
func (a *Account) DisplayName() string {
	if a.Nickname != nil && *a.Nickname != "" {
		return *a.Nickname
	}
	return a.Name
}

type BalanceHistory struct {
	ID               int
	AccountID        string
	Balance          int
	AvailableBalance *int
	RecordedAt       string
}

type Transaction struct {
	ID          string
	AccountID   string
	Posted      string
	Amount      int
	Description string
	Pending     bool
	IsTransfer  bool
	CategoryID  *int
}

type Organization struct {
	ID   string
	Name string
	URL  *string
}

type Category struct {
	ID   int
	Name string
}
