package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HybridMind1337/DataMask/internal/config"
	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Driver implements database.Driver for PostgreSQL using pgx/v5.
type Driver struct {
	pool      *pgxpool.Pool
	cfg       config.DatabaseConfig
	readOnly  bool
	inspector *Inspector
	reader    *Reader
	writer    *Writer
}

// NewDriver connects to PostgreSQL. If readOnly is true, enforces read-only session defaults.
func NewDriver(ctx context.Context, cfg config.DatabaseConfig, readOnly bool) (*Driver, error) {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(cfg.Username),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		port,
		cfg.Database,
		sslMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing postgres connection string: %w", err)
	}

	if readOnly {
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY;")
			return err
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	d := &Driver{
		pool:     pool,
		cfg:      cfg,
		readOnly: readOnly,
	}
	d.inspector = NewInspector(pool, cfg.Schema)
	d.reader = NewReader(pool)
	d.writer = NewWriter(pool, readOnly)

	return d, nil
}

func (d *Driver) Type() string { return "postgres" }

func (d *Driver) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}

func (d *Driver) Close() error {
	d.pool.Close()
	return nil
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
	_, err := d.pool.Exec(ctx, query)
	return err
}

func (d *Driver) DisableConstraints(ctx context.Context, table *discovery.Table) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	query := fmt.Sprintf("ALTER TABLE %q DISABLE TRIGGER ALL;", table.Name)
	_, err := d.pool.Exec(ctx, query)
	return err
}

func (d *Driver) EnableConstraints(ctx context.Context, table *discovery.Table) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	query := fmt.Sprintf("ALTER TABLE %q ENABLE TRIGGER ALL;", table.Name)
	_, err := d.pool.Exec(ctx, query)
	return err
}

func (d *Driver) DisableAllForeignKeys(ctx context.Context, schema string) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := d.pool.Exec(ctx, "SET session_replication_role = 'replica';")
	return err
}

func (d *Driver) EnableAllForeignKeys(ctx context.Context, schema string) error {
	if d.readOnly {
		return database.ErrReadOnlyViolation
	}
	_, err := d.pool.Exec(ctx, "SET session_replication_role = 'origin';")
	return err
}
