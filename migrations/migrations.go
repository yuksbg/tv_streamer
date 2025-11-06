package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"tv_streamer/helpers/logs"
)

//go:embed sql_files/*.sql
var migrationFS embed.FS

// Migration represents a single database migration
type Migration struct {
	Version uint
	Name    string
	UpSQL   string
	DownSQL string
}

// Run executes all pending database migrations
func Run(db *sql.DB) error {
	logger := logs.GetLogger()

	// Create schema_migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			dirty INTEGER NOT NULL DEFAULT 0,
			executed_at DATETIME
		)
	`)
	if err != nil {
		logger.WithError(err).Error("Failed to create schema_migrations table")
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Add executed_at column if it doesn't exist (for existing databases)
	var columnExists bool
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('schema_migrations') WHERE name='executed_at'").Scan(&columnExists)
	if err != nil {
		logger.WithError(err).Error("Failed to check for executed_at column")
		return fmt.Errorf("failed to check for executed_at column: %w", err)
	}

	if !columnExists {
		logger.Info("Adding executed_at column to schema_migrations table")
		_, err = db.Exec("ALTER TABLE schema_migrations ADD COLUMN executed_at DATETIME")
		if err != nil {
			logger.WithError(err).Error("Failed to add executed_at column")
			return fmt.Errorf("failed to add executed_at column: %w", err)
		}
	}

	// Get current version
	var currentVersion uint
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations WHERE dirty = 0").Scan(&currentVersion)
	if err != nil {
		logger.WithError(err).Error("Failed to get current migration version")
		return fmt.Errorf("failed to get current version: %w", err)
	}

	logger.WithField("current_version", currentVersion).Info("Current migration version")

	// Load all migrations
	migrations, err := loadMigrations()
	if err != nil {
		logger.WithError(err).Error("Failed to load migrations")
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Filter migrations that need to be applied
	pendingMigrations := []Migration{}
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) == 0 {
		logger.Info("No new migrations to apply")
		return nil
	}

	logger.WithField("count", len(pendingMigrations)).Info("Running database migrations...")

	// Apply migrations
	for _, migration := range pendingMigrations {
		startTime := time.Now()

		logger.WithFields(map[string]interface{}{
			"version":   migration.Version,
			"name":      migration.Name,
			"timestamp": startTime.Format("2006-01-02 15:04:05"),
		}).Info("Applying migration")

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Mark as dirty
		_, err = tx.Exec("INSERT INTO schema_migrations (version, dirty) VALUES (?, 1) ON CONFLICT(version) DO UPDATE SET dirty = 1", migration.Version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to mark migration as dirty: %w", err)
		}

		// Execute migration with error handling for idempotent operations
		_, err = tx.Exec(migration.UpSQL)
		if err != nil {
			// Check if error is due to duplicate column (idempotent migration)
			if strings.Contains(err.Error(), "duplicate column name") {
				logger.WithFields(map[string]interface{}{
					"version": migration.Version,
					"warning": "Column already exists, treating as successful",
				}).Warn("Migration already applied manually")
				// Don't rollback - column already exists, migration goal achieved
			} else {
				tx.Rollback()
				logger.WithError(err).WithField("version", migration.Version).Error("Migration failed")
				return fmt.Errorf("failed to execute migration %d: %w", migration.Version, err)
			}
		}

		// Mark as clean and record execution timestamp
		_, err = tx.Exec("UPDATE schema_migrations SET dirty = 0, executed_at = ? WHERE version = ?", time.Now().Format("2006-01-02 15:04:05"), migration.Version)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to mark migration as clean: %w", err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		duration := time.Since(startTime)
		logger.WithFields(map[string]interface{}{
			"version":      migration.Version,
			"duration":     duration.String(),
			"completed_at": time.Now().Format("2006-01-02 15:04:05"),
		}).Info("Migration applied successfully")
	}

	logger.Info("Database migrations completed successfully")
	return nil
}

// loadMigrations loads all migration files from the embedded filesystem
func loadMigrations() ([]Migration, error) {
	migrations := make(map[uint]*Migration)

	err := fs.WalkDir(migrationFS, "sql_files", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Parse filename: {version}_{name}.{up|down}.sql
		filename := filepath.Base(path)
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			return fmt.Errorf("invalid migration filename: %s", filename)
		}

		version, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			return fmt.Errorf("invalid version in filename %s: %w", filename, err)
		}

		// Read file content
		content, err := fs.ReadFile(migrationFS, path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		// Determine if this is an up or down migration
		isUp := strings.HasSuffix(filename, ".up.sql")
		isDown := strings.HasSuffix(filename, ".down.sql")

		if !isUp && !isDown {
			return fmt.Errorf("migration file must end with .up.sql or .down.sql: %s", filename)
		}

		// Get or create migration entry
		v := uint(version)
		if migrations[v] == nil {
			// Extract name from filename
			nameParts := strings.Split(filename, "_")
			name := strings.TrimSuffix(strings.Join(nameParts[1:], "_"), ".up.sql")
			name = strings.TrimSuffix(name, ".down.sql")

			migrations[v] = &Migration{
				Version: v,
				Name:    name,
			}
		}

		if isUp {
			migrations[v].UpSQL = string(content)
		} else {
			migrations[v].DownSQL = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	result := make([]Migration, 0, len(migrations))
	for _, m := range migrations {
		if m.UpSQL == "" {
			return nil, fmt.Errorf("migration %d is missing .up.sql file", m.Version)
		}
		result = append(result, *m)
	}

	// Sort by version
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}
