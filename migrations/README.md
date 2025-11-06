# Database Migrations

This project uses a custom lightweight migration system inspired by [golang-migrate](https://github.com/golang-migrate/migrate) for database schema migrations with embedded SQL files.

## Overview

- Migrations are stored in `migrations/sql_files/` directory
- Migration files are embedded into the binary using Go's `embed` package
- Migrations run automatically on application startup
- SQLite database with foreign key support

## Migration File Naming Convention

Migration files follow the format: `{version}_{description}.{up|down}.sql`

Example:
- `000001_initial_schema.up.sql` - Creates the initial schema
- `000001_initial_schema.down.sql` - Rolls back the initial schema

## Creating New Migrations

1. Create two files in `migrations/sql_files/` directory:
   - `{next_version}_{description}.up.sql` - Forward migration
   - `{next_version}_{description}.down.sql` - Rollback migration

2. Example for version 2:
   ```sql
   -- 000002_add_users_table.up.sql
   CREATE TABLE IF NOT EXISTS users (
       rowid INTEGER PRIMARY KEY AUTOINCREMENT,
       username VARCHAR(100) NOT NULL UNIQUE,
       email VARCHAR(255) NOT NULL UNIQUE,
       created_time INTEGER NOT NULL
   );

   CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
   CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
   ```

   ```sql
   -- 000002_add_users_table.down.sql
   DROP INDEX IF EXISTS idx_users_email;
   DROP INDEX IF EXISTS idx_users_username;
   DROP TABLE IF EXISTS users;
   ```

3. The migration will run automatically on next application startup

## Migration Execution

Migrations are executed automatically when the application starts via `helpers/db.go`:

```go
// Run database migrations
sqlDB := engine.DB().DB
if err := migrations.Run(sqlDB); err != nil {
    log.Panicln("Failed to run migrations:", err.Error())
}
```

## Features

- **Embedded Migrations**: All SQL files are embedded in the binary
- **Automatic Execution**: Runs on startup
- **Version Tracking**: Maintains migration version in the database
- **Rollback Support**: Each migration has a corresponding down migration
- **Idempotent**: Safe to run multiple times (uses `IF NOT EXISTS` clauses)

## Current Schema

The initial migration (000001) creates:
- `content_categories` - Categories for content organization
- `content` - Main content table with foreign key to categories
- `content_images` - Images associated with content
- `content_tags` - Tags for content
- `navigation` - Navigation menu items

All tables include appropriate indexes for performance.

## Logs

Migration execution is logged with the following information:
- Migration start/completion messages
- Current migration version
- Any errors during migration

Check application logs for migration status.

## Implementation Details

The migration system is implemented as a lightweight custom solution in [migrations.go](migrations.go) that:
- Uses Go's standard `database/sql` package (no external migration dependencies)
- Avoids driver conflicts with `ncruces/go-sqlite3`
- Provides transaction-based migration execution
- Tracks migration state in a `schema_migrations` table with dirty flag support
- Automatically loads and sorts migrations from embedded files

## Best Practices

1. **Always test migrations**: Test both up and down migrations in development
2. **Use transactions**: Wrap related changes in transactions when possible
3. **Make migrations reversible**: Always provide a proper down migration
4. **Keep migrations small**: One logical change per migration
5. **Don't modify existing migrations**: Once applied, create a new migration instead
6. **Use IF NOT EXISTS**: Makes migrations idempotent and safer

## Troubleshooting

If migrations fail:
1. Check application logs for error details
2. Verify SQL syntax in migration files
3. Ensure migration version numbers are sequential
4. Check database file permissions
5. Verify foreign key constraints are valid

The migration system maintains a `schema_migrations` table in the database to track applied migrations.
