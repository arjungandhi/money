package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	tempDir := t.TempDir()

	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", tempDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	db, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	dbPath := filepath.Join(tempDir, "money.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file does not exist at %s", dbPath)
	}

	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='credentials'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query credentials table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 credentials table, got %d", count)
	}

	expectedTables := []string{"credentials", "organizations", "accounts", "categories", "transactions"}
	for _, tableName := range expectedTables {
		err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query %s table: %v", tableName, err)
		}
		if count != 1 {
			t.Errorf("Expected 1 %s table, got %d", tableName, count)
		}
	}
}

func TestClose(t *testing.T) {
	tempDir := t.TempDir()

	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", tempDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	db, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Errorf("Failed to close database: %v", err)
	}

	err = db.conn.Ping()
	if err == nil {
		t.Error("Expected error when pinging closed database, got nil")
	}
}

func TestGetMoneyDir(t *testing.T) {
	testDir := "/test/money/dir"
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", testDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	dir := getMoneyDir()
	if dir != testDir {
		t.Errorf("Expected %s, got %s", testDir, dir)
	}

	os.Unsetenv("MONEY_DIR")
	dir = getMoneyDir()

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".money")
	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}

func TestCredentials(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Set MONEY_DIR environment variable to temp directory
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", tempDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	// Initialize database
	db, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test HasCredentials on empty database
	hasCredentials, err := db.HasCredentials()
	if err != nil {
		t.Errorf("Failed to check credentials: %v", err)
	}
	if hasCredentials {
		t.Error("Expected no credentials in empty database")
	}

	// Test SaveCredentials
	testAccessURL := "https://example.com/api"
	testUsername := "testuser"
	testPassword := "testpass"

	err = db.SaveCredentials(testAccessURL, testUsername, testPassword)
	if err != nil {
		t.Errorf("Failed to save credentials: %v", err)
	}

	// Test HasCredentials after saving
	hasCredentials, err = db.HasCredentials()
	if err != nil {
		t.Errorf("Failed to check credentials: %v", err)
	}
	if !hasCredentials {
		t.Error("Expected credentials to be present after saving")
	}

	// Test GetCredentials
	accessURL, username, password, err := db.GetCredentials()
	if err != nil {
		t.Errorf("Failed to get credentials: %v", err)
	}
	if accessURL != testAccessURL {
		t.Errorf("Expected access URL %s, got %s", testAccessURL, accessURL)
	}
	if username != testUsername {
		t.Errorf("Expected username %s, got %s", testUsername, username)
	}
	if password != testPassword {
		t.Errorf("Expected password %s, got %s", testPassword, password)
	}

	// Test overwriting credentials
	newAccessURL := "https://new.example.com/api"
	newUsername := "newuser"
	newPassword := "newpass"

	err = db.SaveCredentials(newAccessURL, newUsername, newPassword)
	if err != nil {
		t.Errorf("Failed to save new credentials: %v", err)
	}

	// Verify old credentials are replaced
	accessURL, username, password, err = db.GetCredentials()
	if err != nil {
		t.Errorf("Failed to get updated credentials: %v", err)
	}
	if accessURL != newAccessURL {
		t.Errorf("Expected new access URL %s, got %s", newAccessURL, accessURL)
	}
	if username != newUsername {
		t.Errorf("Expected new username %s, got %s", newUsername, username)
	}
	if password != newPassword {
		t.Errorf("Expected new password %s, got %s", newPassword, password)
	}

	// Verify only one set of credentials exists
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM credentials").Scan(&count)
	if err != nil {
		t.Errorf("Failed to count credentials: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 set of credentials after overwrite, got %d", count)
	}
}

func TestAccountsAndOrganizations(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Set MONEY_DIR environment variable to temp directory
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", tempDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)

	// Initialize database
	db, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test empty accounts
	accounts, err := db.GetAccounts()
	if err != nil {
		t.Errorf("Failed to get accounts from empty database: %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("Expected 0 accounts in empty database, got %d", len(accounts))
	}

	// Test empty organizations
	orgs, err := db.GetOrganizations()
	if err != nil {
		t.Errorf("Failed to get organizations from empty database: %v", err)
	}
	if len(orgs) != 0 {
		t.Errorf("Expected 0 organizations in empty database, got %d", len(orgs))
	}

	// Add test organization
	err = db.SaveOrganization("test-org-1", "Test Bank", "https://test.bank.com")
	if err != nil {
		t.Fatalf("Failed to save test organization: %v", err)
	}

	// Add test account
	err = db.SaveAccount("test-acc-1", "test-org-1", "Test Checking Account", "USD", 123456, nil, "")
	if err != nil {
		t.Fatalf("Failed to save test account: %v", err)
	}

	// Test getting organizations
	orgs, err = db.GetOrganizations()
	if err != nil {
		t.Errorf("Failed to get organizations: %v", err)
	}
	if len(orgs) != 1 {
		t.Errorf("Expected 1 organization, got %d", len(orgs))
	}
	if len(orgs) > 0 {
		org := orgs[0]
		if org.ID != "test-org-1" {
			t.Errorf("Expected organization ID 'test-org-1', got '%s'", org.ID)
		}
		if org.Name != "Test Bank" {
			t.Errorf("Expected organization name 'Test Bank', got '%s'", org.Name)
		}
		if org.URL == nil || *org.URL != "https://test.bank.com" {
			t.Errorf("Expected organization URL 'https://test.bank.com', got %v", org.URL)
		}
	}

	// Test getting accounts
	accounts, err = db.GetAccounts()
	if err != nil {
		t.Errorf("Failed to get accounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Errorf("Expected 1 account, got %d", len(accounts))
	}
	if len(accounts) > 0 {
		account := accounts[0]
		if account.ID != "test-acc-1" {
			t.Errorf("Expected account ID 'test-acc-1', got '%s'", account.ID)
		}
		if account.OrgID != "test-org-1" {
			t.Errorf("Expected account org ID 'test-org-1', got '%s'", account.OrgID)
		}
		if account.Name != "Test Checking Account" {
			t.Errorf("Expected account name 'Test Checking Account', got '%s'", account.Name)
		}
		if account.Currency != "USD" {
			t.Errorf("Expected account currency 'USD', got '%s'", account.Currency)
		}
		if account.Balance != 123456 {
			t.Errorf("Expected account balance 123456, got %d", account.Balance)
		}
		if account.AvailableBalance != nil {
			t.Errorf("Expected nil available balance, got %v", *account.AvailableBalance)
		}
		if account.BalanceDate != nil {
			t.Errorf("Expected nil balance date, got %v", *account.BalanceDate)
		}
	}
}
