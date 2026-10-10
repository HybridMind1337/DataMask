package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Driver implements database.Driver for MySQL and MariaDB.
type Driver struct {
	db        *sql.DB
	cfg       config.DatabaseConfig
	readOnly  bool
	inspector *Inspector
	reader    *Reader
	writer    *Writer
}

// NewDriver connects to MySQL / MariaDB.
func NewDriver(ctx context.Context, cfg config.DatabaseConfig, readOnly bool) (*Driver, error) {
	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		port,
		cfg.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening mysql connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("pinging mysql: %w", err)
	}

	d := &Driver{
		db:       db,
		cfg:      cfg,
		readOnly: readOnly,
	}
	d.inspector = NewInspector(db, cfg.Database)
	d.reader = NewReader(db)
	d.writer = NewWriter(db, readOnly)

	return d, nil
}

func (d *Driver) Type() string { return "mysql" }

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
		upper := strings.ToUpper(strings.TrimSpace(query))
		if strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE") || strings.HasPrefix(upper, "DELETE") || strings.HasPrefix(upper, "DROP") || strings.HasPrefix(upper, "TRUNCATE") {
			return database.ErrReadOnlyViolation
		}
	}
	_, err := d.db.ExecContext(ctx, query)
	return err
}

func (d *Driver) DisableConstraints(ctx context.Context, table *discovery.Table) error {
	return d.DisableAllForeignKeys(ctx, table.Schema)
}

func (d *Driver) EnableConstraints(ctx context.Context, table *discovery.Table) error {
	return d.EnableAllForeignKeys(ctx, table.Schema)
}

func (d *Driver) DisableAllForeignKeys(ctx context.Context, schema string) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0;")
	return err
}

func (d *Driver) EnableAllForeignKeys(ctx context.Context, schema string) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := d.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1;")
	return err
}
