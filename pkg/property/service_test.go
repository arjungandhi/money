package property

import (
	"testing"

	"github.com/arjungandhi/money/pkg/database"
)

func TestNewService(t *testing.T) {
	var db *database.DB
	service := &Service{db: db}

	if service == nil {
		t.Error("Service should be creatable")
	}

	if service.db != db {
		t.Error("Service should store the database reference")
	}
}

// Note: More comprehensive tests would require either:
// 1. A test database setup
// 2. Mocking the database interface
// 3. Integration tests with a real database
// For now, we just test the basic structure
