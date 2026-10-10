package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Reader streams rows from PostgreSQL using chunked cursor queries.
type Reader struct {
	pool *pgxpool.Pool
}

// NewReader instantiates a PostgreSQL reader.
func NewReader(pool *pgxpool.Pool) *Reader {
	return &Reader{pool: pool}
}

// ReadTableStream opens a cursor or paginated query and streams batches across channels.
func (r *Reader) ReadTableStream(ctx context.Context, table *discovery.Table, batchSize int) (<-chan database.Batch, <-chan error) {
	batchChan := make(chan database.Batch, 4)
	errChan := make(chan error, 1)

	if batchSize <= 0 {
		batchSize = 5000
	}

	go func() {
		defer close(batchChan)
		defer close(errChan)

		// Acquire dedicated connection for transaction-scoped server-side cursor
		conn, err := r.pool.Acquire(ctx)
		if err != nil {
			errChan <- fmt.Errorf("acquiring postgres connection for streaming %s: %w", table.Name, err)
			return
		}
		defer conn.Release()

		tx, err := conn.Begin(ctx)
		if err != nil {
			errChan <- fmt.Errorf("beginning cursor transaction for %s: %w", table.Name, err)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Build quoted column projection list
		quotedCols := make([]string, len(table.ColumnOrder))
		for i, col := range table.ColumnOrder {
			quotedCols[i] = fmt.Sprintf("%q", col)
		}
		colsList := strings.Join(quotedCols, ", ")

		cursorName := fmt.Sprintf("cur_%s", table.Name)
		declareSQL := fmt.Sprintf("DECLARE %s NO SCROLL CURSOR FOR SELECT %s FROM %q.%q;", cursorName, colsList, table.Schema, table.Name)
		if _, err := tx.Exec(ctx, declareSQL); err != nil {
			errChan <- fmt.Errorf("declaring cursor for %s: %w", table.Name, err)
			return
		}

		fetchSQL := fmt.Sprintf("FETCH %d FROM %s;", batchSize, cursorName)

		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			rows, err := tx.Query(ctx, fetchSQL)
			if err != nil {
				errChan <- fmt.Errorf("fetching rows from cursor for %s: %w", table.Name, err)
				return
			}

			var batchRows []database.Row
			for rows.Next() {
				values, err := rows.Values()
				if err != nil {
					rows.Close()
					errChan <- fmt.Errorf("reading column values from %s: %w", table.Name, err)
					return
				}

				rowMap := make(map[string]any, len(table.ColumnOrder))
				for i, colName := range table.ColumnOrder {
					rowMap[colName] = values[i]
				}
				batchRows = append(batchRows, database.Row{Values: rowMap})
			}
			rows.Close()

			if len(batchRows) == 0 {
				break // End of stream
			}

			batchChan <- database.Batch{
				TableName: table.Name,
				Columns:   table.ColumnOrder,
				Rows:      batchRows,
			}
		}

		_ = tx.Commit(ctx)
	}()

	return batchChan, errChan
}
