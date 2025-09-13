package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Set MONEY_DIR environment variable to temp directory
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", tempDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)
	
	// Test database initialization
	db, err := New()
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Verify database file exists
	dbPath := filepath.Join(tempDir, "money.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file does not exist at %s", dbPath)
	}
	
	// Test that tables were created by querying one
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='credentials'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query credentials table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 credentials table, got %d", count)
	}
	
	// Test all expected tables exist
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
	
	// Test closing
	err = db.Close()
	if err != nil {
		t.Errorf("Failed to close database: %v", err)
	}
	
	// Test that connection is closed by trying to ping
	err = db.conn.Ping()
	if err == nil {
		t.Error("Expected error when pinging closed database, got nil")
	}
}

func TestGetMoneyDir(t *testing.T) {
	// Test with environment variable set
	testDir := "/test/money/dir"
	oldMoneyDir := os.Getenv("MONEY_DIR")
	os.Setenv("MONEY_DIR", testDir)
	defer os.Setenv("MONEY_DIR", oldMoneyDir)
	
	dir := getMoneyDir()
	if dir != testDir {
		t.Errorf("Expected %s, got %s", testDir, dir)
	}
	
	// Test with environment variable unset
	os.Unsetenv("MONEY_DIR")
	dir = getMoneyDir()
	
	// Should return $HOME/.money
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".money")
	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}