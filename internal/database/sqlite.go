package database

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func NewSQLite(dbPath string) (*sql.DB, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_timeout=5000&_fk=1")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite
	db.SetMaxOpenConns(1) // SQLite only supports one writer
	db.SetMaxIdleConns(1)

	return db, nil
}

func RunMigrations(db *sql.DB) error {
	// Create migrations table
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version TEXT PRIMARY KEY,
            applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied := make(map[string]bool)
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return err
		}
		applied[version] = true
	}

	// Read and sort migration files
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sql") {
			migrations = append(migrations, entry.Name())
		}
	}
	sort.Strings(migrations)

	// Apply migrations
	for _, filename := range migrations {
		version := strings.TrimSuffix(filename, ".sql")
		if applied[version] {
			continue
		}

		content, err := fs.ReadFile(migrationFiles, "migrations/"+filename)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for migration %s: %w", version, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}

		fmt.Printf("Applied migration: %s\n", version)
	}

	return nil
}
