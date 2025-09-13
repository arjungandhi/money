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
	// TODO: Save SimpleFIN credentials
	return nil
}

func (db *DB) GetCredentials() (accessURL, username, password string, err error) {
	// TODO: Retrieve stored credentials
	return "", "", "", nil
}

// Organization methods
func (db *DB) SaveOrganization(id, name, url string) error {
	// TODO: Save organization data
	return nil
}

// Account methods
func (db *DB) SaveAccount(id, orgID, name, currency string, balance, availableBalance int, balanceDate string) error {
	// TODO: Save account data
	return nil
}

func (db *DB) GetAccounts() ([]Account, error) {
	// TODO: Retrieve all accounts
	return nil, nil
}

// Transaction methods
func (db *DB) SaveTransaction(id, accountID, posted string, amount int, description string, pending bool) error {
	// TODO: Save transaction data
	return nil
}

func (db *DB) GetTransactions(accountID string, startDate, endDate string) ([]Transaction, error) {
	// TODO: Retrieve transactions for account in date range
	return nil, nil
}

func (db *DB) GetUncategorizedTransactions() ([]Transaction, error) {
	// TODO: Retrieve transactions without category assignments
	return nil, nil
}

func (db *DB) UpdateTransactionCategory(transactionID string, categoryID int) error {
	// TODO: Update transaction with category assignment
	return nil
}

// Category methods
func (db *DB) SaveCategory(name, categoryType string) (int, error) {
	// TODO: Save category and return ID
	return 0, nil
}

func (db *DB) GetCategories() ([]Category, error) {
	// TODO: Retrieve all categories
	return nil, nil
}

// Data types
type Account struct {
	ID               string
	OrgID            string
	Name             string
	Currency         string
	Balance          int
	AvailableBalance *int
	BalanceDate      *string
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
	ID   int
	Name string
	Type string
}
