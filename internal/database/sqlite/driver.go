package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Driver implements database.Driver for SQLite using modernc.org/sqlite (pure Go, CGO-free).
type Driver struct {
	db        *sql.DB
	path      string
	readOnly  bool
	inspector *Inspector
	reader    *Reader
	writer    *Writer
}

// NewDriver opens or connects to an SQLite database file.
func NewDriver(ctx context.Context, cfg config.DatabaseConfig, readOnly bool) (*Driver, error) {
	dbPath := cfg.Database
	if dbPath == "" {
		dbPath = cfg.Host
	}
	if dbPath == "" {
		return nil, fmt.Errorf("sqlite database path is required")
	}

	// If read-write, ensure directory exists
	if !readOnly && dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("creating sqlite directory: %w", err)
			}
		}
	}

	dsn := dbPath
	if readOnly {
		if strings.Contains(dsn, "?") {
			dsn += "&mode=ro"
		} else {
			dsn += "?mode=ro"
		}
	} else {
		// WAL mode and busy timeout for high-performance concurrent writes
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		} else {
			dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging sqlite database: %w", err)
	}

	d := &Driver{
		db:       db,
		path:     dbPath,
		readOnly: readOnly,
	}
	d.inspector = NewInspector(db)
	d.reader = NewReader(db)
	d.writer = NewWriter(db, readOnly)

	return d, nil
}

func (d *Driver) Type() string { return "sqlite" }

func (d *Driver) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *Driver) Close() error {
	return d.db.Close()
}

func (d *Driver) Inspector() database.SchemaInspector {
	return d.inspector
}

func (d *Driver) Reader() database.RowReader {
	return d.reader
}

func (d *Driver) Writer() database.RowWriter {
	return d.writer
}

func (d *Driver) Constraints() database.ConstraintManager {
	return d
}

func (d *Driver) ExecDirect(ctx context.Context, query string) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := d.db.ExecContext(ctx, query)
	return err
}

func (d *Driver) DisableConstraints(ctx context.Context, table *discovery.Table) error {
	_, err := d.db.ExecContext(ctx, "PRAGMA foreign_keys = OFF;")
	return err
}

func (d *Driver) EnableConstraints(ctx context.Context, table *discovery.Table) error {
	_, err := d.db.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
	return err
}

func (d *Driver) DisableAllForeignKeys(ctx context.Context, schema string) error {
	_, err := d.db.ExecContext(ctx, "PRAGMA foreign_keys = OFF;")
	return err
}

func (d *Driver) EnableAllForeignKeys(ctx context.Context, schema string) error {
	_, err := d.db.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
	return err
}
