package dbutil

import "github.com/arjungandhi/money/pkg/database"

// WithDatabase executes a function with a database connection, handling initialization and cleanup
func WithDatabase(fn func(*database.DB) error) error {
	db, err := database.New()
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}