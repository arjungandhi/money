package database

import (
	"database/sql"
	_ "embed"
	"fmt"

	"github.com/arjungandhi/money/pkg/config"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	conn   *sql.DB
	config *config.Config
}

func New() (*DB, error) {
	cfg := config.New()

	if err := cfg.EnsureMoneyDir(); err != nil {
		return nil, fmt.Errorf("failed to create money directory: %w", err)
	}
	dbPath := cfg.DBPath()
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn:   conn,
		config: cfg,
	}

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
	var tableCount int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to check existing tables: %w", err)
	}

	if tableCount == 0 {
		_, err = db.conn.Exec(schemaSQL)
		if err != nil {
			return fmt.Errorf("failed to execute schema: %w", err)
		}
	} else {
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
			ADD COLUMN account_type TEXT CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'loan', 'property', 'other'))
		`)
		if err != nil {
			return fmt.Errorf("failed to add account_type column: %w", err)
		}
	}

	// Check if is_internal column exists in categories table
	var internalColumnExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('categories')
		WHERE name = 'is_internal'
	`).Scan(&internalColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check is_internal column: %w", err)
	}

	// Add is_internal column if it doesn't exist
	if internalColumnExists == 0 {
		_, err = db.conn.Exec(`
			ALTER TABLE categories
			ADD COLUMN is_internal BOOLEAN DEFAULT FALSE
		`)
		if err != nil {
			return fmt.Errorf("failed to add is_internal column: %w", err)
		}
	}

	// Remove is_transfer column from transactions if it exists
	var transferColumnExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('transactions')
		WHERE name = 'is_transfer'
	`).Scan(&transferColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check is_transfer column: %w", err)
	}

	// Remove is_transfer column from transactions if it exists (recreate table)
	if transferColumnExists > 0 {
		// Start a transaction for this complex migration
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		// Create new transactions table without is_transfer column
		_, err = tx.Exec(`
			CREATE TABLE transactions_new (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL,
				posted DATETIME NOT NULL,
				amount INTEGER NOT NULL,
				description TEXT NOT NULL,
				pending BOOLEAN DEFAULT FALSE,
				category_id INTEGER,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (account_id) REFERENCES accounts(id),
				FOREIGN KEY (category_id) REFERENCES categories(id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create new transactions table: %w", err)
		}

		// Copy data from old table to new (without is_transfer column)
		_, err = tx.Exec(`
			INSERT INTO transactions_new (id, account_id, posted, amount, description, pending, category_id, created_at, updated_at)
			SELECT id, account_id, posted, amount, description, pending, category_id, created_at, updated_at FROM transactions
		`)
		if err != nil {
			return fmt.Errorf("failed to copy transactions data: %w", err)
		}

		// Drop old table and rename new one
		_, err = tx.Exec(`DROP TABLE transactions`)
		if err != nil {
			return fmt.Errorf("failed to drop old transactions table: %w", err)
		}

		_, err = tx.Exec(`ALTER TABLE transactions_new RENAME TO transactions`)
		if err != nil {
			return fmt.Errorf("failed to rename new transactions table: %w", err)
		}

		// Recreate indexes for transactions
		_, err = tx.Exec(`CREATE INDEX idx_transactions_account_id ON transactions(account_id)`)
		if err != nil {
			return fmt.Errorf("failed to create transactions account_id index: %w", err)
		}

		_, err = tx.Exec(`CREATE INDEX idx_transactions_posted ON transactions(posted)`)
		if err != nil {
			return fmt.Errorf("failed to create transactions posted index: %w", err)
		}

		_, err = tx.Exec(`CREATE INDEX idx_transactions_category_id ON transactions(category_id)`)
		if err != nil {
			return fmt.Errorf("failed to create transactions category_id index: %w", err)
		}

		// Commit the transaction
		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	// Create index for categories is_internal if it doesn't exist
	_, err = db.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_categories_is_internal ON categories(is_internal)`)
	if err != nil {
		return fmt.Errorf("failed to create categories is_internal index: %w", err)
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

	// Check if properties table exists
	var propertiesTableExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type='table' AND name='properties'
	`).Scan(&propertiesTableExists)
	if err != nil {
		return fmt.Errorf("failed to check properties table: %w", err)
	}

	// Create properties table if it doesn't exist
	if propertiesTableExists == 0 {
		_, err = db.conn.Exec(`
			CREATE TABLE properties (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				account_id TEXT NOT NULL UNIQUE,
				address TEXT NOT NULL,
				city TEXT NOT NULL,
				state TEXT NOT NULL,
				zip_code TEXT NOT NULL,
				latitude REAL,
				longitude REAL,
				last_value_estimate INTEGER,
				last_rent_estimate INTEGER,
				last_updated DATETIME,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (account_id) REFERENCES accounts(id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create properties table: %w", err)
		}

		// Create index for properties
		_, err = db.conn.Exec(`
			CREATE INDEX idx_properties_account_id ON properties(account_id);
		`)
		if err != nil {
			return fmt.Errorf("failed to create properties account_id index: %w", err)
		}
	}

	// Check if we need to update the account_type constraint to include 'property'
	// We need to recreate the accounts table with the updated constraint
	var hasPropertyType int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type='table' AND name='accounts' AND sql LIKE '%property%'
	`).Scan(&hasPropertyType)
	if err != nil {
		return fmt.Errorf("failed to check account_type constraint: %w", err)
	}

	// If the constraint doesn't include 'property', we need to recreate the table
	if hasPropertyType == 0 {
		// Start a transaction for this complex migration
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		defer tx.Rollback()

		// Create new accounts table with updated constraint
		_, err = tx.Exec(`
			CREATE TABLE accounts_new (
				id TEXT PRIMARY KEY,
				org_id TEXT NOT NULL,
				name TEXT NOT NULL,
				nickname TEXT,
				currency TEXT NOT NULL DEFAULT 'USD',
				balance INTEGER NOT NULL,
				available_balance INTEGER,
				balance_date DATETIME,
				account_type TEXT CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'loan', 'property', 'other', 'unset')) DEFAULT 'unset',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (org_id) REFERENCES organizations(id)
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create new accounts table: %w", err)
		}

		// Copy data from old table to new
		_, err = tx.Exec(`
			INSERT INTO accounts_new (id, org_id, name, nickname, currency, balance, available_balance, balance_date, account_type, created_at, updated_at)
			SELECT id, org_id, name, nickname, currency, balance, available_balance, balance_date, account_type, created_at, updated_at
			FROM accounts
		`)
		if err != nil {
			return fmt.Errorf("failed to copy accounts data: %w", err)
		}

		// Drop old table and rename new one
		_, err = tx.Exec(`DROP TABLE accounts`)
		if err != nil {
			return fmt.Errorf("failed to drop old accounts table: %w", err)
		}

		_, err = tx.Exec(`ALTER TABLE accounts_new RENAME TO accounts`)
		if err != nil {
			return fmt.Errorf("failed to rename new accounts table: %w", err)
		}

		// Recreate the index
		_, err = tx.Exec(`CREATE INDEX idx_accounts_org_id ON accounts(org_id)`)
		if err != nil {
			return fmt.Errorf("failed to recreate accounts org_id index: %w", err)
		}

		// Commit the transaction
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit accounts table migration: %w", err)
		}
	}

	// Check if rentcast_credentials table exists
	var rentcastTableExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type='table' AND name='rentcast_credentials'
	`).Scan(&rentcastTableExists)
	if err != nil {
		return fmt.Errorf("failed to check rentcast_credentials table: %w", err)
	}

	// Create rentcast_credentials table if it doesn't exist
	if rentcastTableExists == 0 {
		_, err = db.conn.Exec(`
			CREATE TABLE rentcast_credentials (
				id INTEGER PRIMARY KEY,
				api_key TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				last_used DATETIME
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to create rentcast_credentials table: %w", err)
		}
	}

	// Check if property_type column exists in properties table
	var propertyTypeColumnExists int
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('properties')
		WHERE name = 'property_type'
	`).Scan(&propertyTypeColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check property_type column: %w", err)
	}

	// Add property_type column if it doesn't exist
	if propertyTypeColumnExists == 0 {
		_, err = db.conn.Exec(`
			ALTER TABLE properties
			ADD COLUMN property_type TEXT CHECK (property_type IN ('Single Family', 'Condo', 'Townhouse', 'Manufactured', 'Multi-Family', 'Apartment', 'Land'))
		`)
		if err != nil {
			return fmt.Errorf("failed to add property_type column: %w", err)
		}
	}

	return nil
}

// GetConfig returns the database configuration
func (db *DB) GetConfig() *config.Config {
	return db.config
}

func (db *DB) SaveCredentials(accessURL, username, password string) error {
	_, err := db.conn.Exec("DELETE FROM credentials")
	if err != nil {
		return fmt.Errorf("failed to clear existing credentials: %w", err)
	}

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
		fmt.Printf("Warning: failed to update last_used timestamp: %v\n", updateErr)
	}

	return accessURL, username, password, nil
}

func (db *DB) SaveRentCastAPIKey(apiKey string) error {
	_, err := db.conn.Exec("DELETE FROM rentcast_credentials")
	if err != nil {
		return fmt.Errorf("failed to clear existing RentCast API key: %w", err)
	}

	_, err = db.conn.Exec(`
		INSERT INTO rentcast_credentials (api_key, last_used)
		VALUES (?, CURRENT_TIMESTAMP)`,
		apiKey)
	if err != nil {
		return fmt.Errorf("failed to save RentCast API key: %w", err)
	}

	return nil
}

func (db *DB) GetRentCastAPIKey() (string, error) {
	var apiKey string
	err := db.conn.QueryRow(`
		SELECT api_key
		FROM rentcast_credentials
		ORDER BY created_at DESC
		LIMIT 1`).Scan(&apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("no RentCast API key found - run 'money property config' to set one")
		}
		return "", fmt.Errorf("failed to retrieve RentCast API key: %w", err)
	}

	// Update last_used timestamp
	_, updateErr := db.conn.Exec("UPDATE rentcast_credentials SET last_used = CURRENT_TIMESTAMP WHERE api_key = ?", apiKey)
	if updateErr != nil {
		fmt.Printf("Warning: failed to update last_used timestamp: %v\n", updateErr)
	}

	return apiKey, nil
}

func (db *DB) HasRentCastAPIKey() (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM rentcast_credentials").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check RentCast API key: %w", err)
	}
	return count > 0, nil
}

func (db *DB) HasCredentials() (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM credentials").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check credentials: %w", err)
	}
	return count > 0, nil
}

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

func (db *DB) UpdateAccountBalance(accountID string, balance int) error {
	_, err := db.conn.Exec(`
		UPDATE accounts
		SET balance = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		balance, accountID)
	if err != nil {
		return fmt.Errorf("failed to update account balance: %w", err)
	}
	return nil
}

func (db *DB) SetAccountType(accountID, accountType string) error {
	// Validate account type
	validTypes := []string{"checking", "savings", "credit", "investment", "loan", "property", "other"}
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

// DeleteAccount deletes an account and all associated data
func (db *DB) DeleteAccount(accountID string) error {
	// Start a transaction to ensure data consistency
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete balance history
	_, err = tx.Exec("DELETE FROM balance_history WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete balance history: %w", err)
	}

	// Delete transactions
	_, err = tx.Exec("DELETE FROM transactions WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete transactions: %w", err)
	}

	// Delete property details if it's a property account
	_, err = tx.Exec("DELETE FROM properties WHERE account_id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete property details: %w", err)
	}

	// Delete the account itself
	result, err := tx.Exec("DELETE FROM accounts WHERE id = ?", accountID)
	if err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("account not found: %s", accountID)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit account deletion: %w", err)
	}

	return nil
}

func (db *DB) SaveTransaction(id, accountID, posted string, amount int, description string, pending bool) error {
	// Use INSERT OR IGNORE to avoid duplicate transactions
	// If the transaction already exists, we don't update it to preserve any manual categorization
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO transactions (id, account_id, posted, amount, description, pending)
		VALUES (?, ?, ?, ?, ?, ?)`,
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
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
				FROM transactions t
				WHERE t.account_id = ? AND t.posted >= ? AND t.posted <= ?
				ORDER BY t.posted DESC`
			args = []interface{}{accountID, startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
				FROM transactions t
				WHERE t.account_id = ?
				ORDER BY t.posted DESC`
			args = []interface{}{accountID}
		}
	} else {
		if startDate != "" && endDate != "" {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
				FROM transactions t
				WHERE t.posted >= ? AND t.posted <= ?
				ORDER BY t.posted DESC`
			args = []interface{}{startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
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
		SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
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

func (db *DB) TransactionExists(id string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM transactions WHERE id = ?", id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check transaction existence: %w", err)
	}
	return count > 0, nil
}

func (db *DB) SaveCategory(name string) (int, error) {
	return db.SaveCategoryWithInternal(name, false)
}

func (db *DB) SaveCategoryWithInternal(name string, isInternal bool) (int, error) {
	// Use INSERT OR IGNORE to avoid duplicate categories, then get the ID
	_, err := db.conn.Exec(`
		INSERT OR IGNORE INTO categories (name, is_internal)
		VALUES (?, ?)`,
		name, isInternal)
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
		SELECT id, name, COALESCE(is_internal, FALSE)
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
		err := rows.Scan(&c.ID, &c.Name, &c.IsInternal)
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
		SELECT id, name, COALESCE(is_internal, FALSE)
		FROM categories
		WHERE id = ?`,
		categoryID).Scan(&c.ID, &c.Name, &c.IsInternal)
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
	// Regular categories
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

	// Internal categories (excluded from budget calculations)
	internalCategories := []string{
		"Transfers",
	}

	// Seed regular categories
	for _, categoryName := range defaultCategories {
		_, err := db.SaveCategory(categoryName)
		if err != nil {
			return fmt.Errorf("failed to seed category '%s': %w", categoryName, err)
		}
	}

	// Seed internal categories
	for _, categoryName := range internalCategories {
		_, err := db.SaveCategoryWithInternal(categoryName, true)
		if err != nil {
			return fmt.Errorf("failed to seed internal category '%s': %w", categoryName, err)
		}
	}

	return nil
}

func (db *DB) SetCategoryInternal(categoryID int, isInternal bool) error {
	result, err := db.conn.Exec(`
		UPDATE categories
		SET is_internal = ?
		WHERE id = ?`,
		isInternal, categoryID)
	if err != nil {
		return fmt.Errorf("failed to set category internal flag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found: %d", categoryID)
	}

	return nil
}

func (db *DB) SetCategoryInternalByName(categoryName string, isInternal bool) error {
	result, err := db.conn.Exec(`
		UPDATE categories
		SET is_internal = ?
		WHERE name = ?`,
		isInternal, categoryName)
	if err != nil {
		return fmt.Errorf("failed to set category internal flag: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("category not found: %s", categoryName)
	}

	return nil
}

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

func (db *DB) GetTransactionsByCategory(startDate, endDate string, excludeInternal bool) (map[string][]Transaction, error) {
	var query string
	var args []interface{}

	if excludeInternal {
		if startDate != "" && endDate != "" {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending,
				       t.category_id, c.name as category_name
				FROM transactions t
				LEFT JOIN categories c ON t.category_id = c.id
				WHERE t.posted >= ? AND t.posted <= ? AND COALESCE(c.is_internal, FALSE) = FALSE
				ORDER BY t.posted DESC`
			args = []interface{}{startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending,
				       t.category_id, c.name as category_name
				FROM transactions t
				LEFT JOIN categories c ON t.category_id = c.id
				WHERE COALESCE(c.is_internal, FALSE) = FALSE
				ORDER BY t.posted DESC`
			args = []interface{}{}
		}
	} else {
		if startDate != "" && endDate != "" {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending,
				       t.category_id, c.name as category_name
				FROM transactions t
				LEFT JOIN categories c ON t.category_id = c.id
				WHERE t.posted >= ? AND t.posted <= ?
				ORDER BY t.posted DESC`
			args = []interface{}{startDate, endDate}
		} else {
			query = `
				SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending,
				       t.category_id, c.name as category_name
				FROM transactions t
				LEFT JOIN categories c ON t.category_id = c.id
				ORDER BY t.posted DESC`
			args = []interface{}{}
		}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions by category: %w", err)
	}
	defer rows.Close()

	categoryTransactions := make(map[string][]Transaction)

	for rows.Next() {
		var t Transaction
		var categoryID sql.NullInt64
		var categoryName sql.NullString

		err := rows.Scan(
			&t.ID,
			&t.AccountID,
			&t.Posted,
			&t.Amount,
			&t.Description,
			&t.Pending,
			&categoryID,
			&categoryName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}

		if categoryID.Valid {
			catID := int(categoryID.Int64)
			t.CategoryID = &catID
		}

		// Determine category name
		var catName string
		if categoryName.Valid {
			catName = categoryName.String
		} else {
			catName = "Uncategorized"
		}

		categoryTransactions[catName] = append(categoryTransactions[catName], t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating transactions: %w", err)
	}

	return categoryTransactions, nil
}

func (db *DB) SaveProperty(accountID, address, city, state, zipCode string, propertyType *string, latitude, longitude *float64) error {
	var latVal, lonVal sql.NullFloat64
	var propTypeVal sql.NullString
	if latitude != nil {
		latVal = sql.NullFloat64{Float64: *latitude, Valid: true}
	}
	if longitude != nil {
		lonVal = sql.NullFloat64{Float64: *longitude, Valid: true}
	}
	if propertyType != nil {
		propTypeVal = sql.NullString{String: *propertyType, Valid: true}
	}

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO properties (account_id, address, city, state, zip_code, property_type, latitude, longitude)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, address, city, state, zipCode, propTypeVal, latVal, lonVal)
	if err != nil {
		return fmt.Errorf("failed to save property: %w", err)
	}
	return nil
}

func (db *DB) GetProperty(accountID string) (*Property, error) {
	var p Property
	var lat, lon sql.NullFloat64
	var propertyType sql.NullString
	var lastValueEstimate, lastRentEstimate sql.NullInt64
	var lastUpdated sql.NullString

	err := db.conn.QueryRow(`
		SELECT account_id, address, city, state, zip_code, property_type, latitude, longitude,
		       last_value_estimate, last_rent_estimate, last_updated
		FROM properties
		WHERE account_id = ?`,
		accountID).Scan(
		&p.AccountID, &p.Address, &p.City, &p.State, &p.ZipCode, &propertyType,
		&lat, &lon, &lastValueEstimate, &lastRentEstimate, &lastUpdated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("property not found for account: %s", accountID)
		}
		return nil, fmt.Errorf("failed to get property: %w", err)
	}

	if propertyType.Valid {
		p.PropertyType = &propertyType.String
	}
	if lat.Valid {
		p.Latitude = &lat.Float64
	}
	if lon.Valid {
		p.Longitude = &lon.Float64
	}
	if lastValueEstimate.Valid {
		estimate := int(lastValueEstimate.Int64)
		p.LastValueEstimate = &estimate
	}
	if lastRentEstimate.Valid {
		estimate := int(lastRentEstimate.Int64)
		p.LastRentEstimate = &estimate
	}
	if lastUpdated.Valid {
		p.LastUpdated = &lastUpdated.String
	}

	return &p, nil
}

func (db *DB) UpdatePropertyValuation(accountID string, valueEstimate, rentEstimate *int) error {
	var valueVal, rentVal sql.NullInt64
	if valueEstimate != nil {
		valueVal = sql.NullInt64{Int64: int64(*valueEstimate), Valid: true}
	}
	if rentEstimate != nil {
		rentVal = sql.NullInt64{Int64: int64(*rentEstimate), Valid: true}
	}

	_, err := db.conn.Exec(`
		UPDATE properties
		SET last_value_estimate = ?, last_rent_estimate = ?, last_updated = CURRENT_TIMESTAMP
		WHERE account_id = ?`,
		valueVal, rentVal, accountID)
	if err != nil {
		return fmt.Errorf("failed to update property valuation: %w", err)
	}
	return nil
}

func (db *DB) GetAllProperties() ([]Property, error) {
	query := `
		SELECT account_id, address, city, state, zip_code, property_type, latitude, longitude,
		       last_value_estimate, last_rent_estimate, last_updated
		FROM properties
		ORDER BY address`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query properties: %w", err)
	}
	defer rows.Close()

	var properties []Property
	for rows.Next() {
		var p Property
		var lat, lon sql.NullFloat64
		var propertyType sql.NullString
		var lastValueEstimate, lastRentEstimate sql.NullInt64
		var lastUpdated sql.NullString

		err := rows.Scan(
			&p.AccountID, &p.Address, &p.City, &p.State, &p.ZipCode, &propertyType,
			&lat, &lon, &lastValueEstimate, &lastRentEstimate, &lastUpdated)
		if err != nil {
			return nil, fmt.Errorf("failed to scan property: %w", err)
		}

		if propertyType.Valid {
			p.PropertyType = &propertyType.String
		}
		if lat.Valid {
			p.Latitude = &lat.Float64
		}
		if lon.Valid {
			p.Longitude = &lon.Float64
		}
		if lastValueEstimate.Valid {
			estimate := int(lastValueEstimate.Int64)
			p.LastValueEstimate = &estimate
		}
		if lastRentEstimate.Valid {
			estimate := int(lastRentEstimate.Int64)
			p.LastRentEstimate = &estimate
		}
		if lastUpdated.Valid {
			p.LastUpdated = &lastUpdated.String
		}

		properties = append(properties, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating properties: %w", err)
	}

	return properties, nil
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
	CategoryID  *int
}

type Organization struct {
	ID   string
	Name string
	URL  *string
}

type Category struct {
	ID         int
	Name       string
	IsInternal bool
}

type Property struct {
	ID                int
	AccountID         string
	Address           string
	City              string
	State             string
	ZipCode           string
	PropertyType      *string
	Latitude          *float64
	Longitude         *float64
	LastValueEstimate *int
	LastRentEstimate  *int
	LastUpdated       *string
}

func (db *DB) GetCategorizedExamples(limit int) ([]Transaction, error) {
	query := `
		SELECT t.id, t.account_id, t.posted, t.amount, t.description, t.pending, t.category_id
		FROM transactions t
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.category_id IS NOT NULL AND COALESCE(c.is_internal, FALSE) = FALSE
		ORDER BY t.posted DESC
		LIMIT ?`

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query categorized examples: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var categoryID *int
		err := rows.Scan(&t.ID, &t.AccountID, &t.Posted, &t.Amount, &t.Description, &t.Pending, &categoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan categorized example: %w", err)
		}
		t.CategoryID = categoryID
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate categorized examples: %w", err)
	}

	return transactions, nil
}
