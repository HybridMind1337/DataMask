package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/HybridMind1337/DataMask/internal/database"
	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// Reader streams rows in bounded chunks from MySQL.
type Reader struct {
	db *sql.DB
}

// NewReader instantiates a MySQL reader.
func NewReader(db *sql.DB) *Reader {
	return &Reader{db: db}
}

// ReadTableStream streams table rows in batches using keyset pagination or bounded offsets.
func (r *Reader) ReadTableStream(ctx context.Context, table *discovery.Table, batchSize int) (<-chan database.Batch, <-chan error) {
	batchChan := make(chan database.Batch, 4)
	errChan := make(chan error, 1)

	if batchSize <= 0 {
		batchSize = 5000
	}

	go func() {
		defer close(batchChan)
		defer close(errChan)

		quotedCols := make([]string, len(table.ColumnOrder))
		for i, c := range table.ColumnOrder {
			quotedCols[i] = fmt.Sprintf("`%s`", c)
		}
		colsList := strings.Join(quotedCols, ", ")

		// Check if table has a single integer primary key for seek/keyset pagination
		hasIntPK := false
		var pkCol string
		if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) == 1 {
			pkCol = table.PrimaryKey.Columns[0]
			if colMeta, ok := table.Columns[pkCol]; ok {
				dt := strings.ToLower(colMeta.DataType)
				if strings.Contains(dt, "int") || strings.Contains(dt, "serial") {
					hasIntPK = true
				}
			}
		}

		if hasIntPK {
			// Keyset pagination (O(1) index seek - avoids O(N^2) deep offset degradation)
			var lastID int64 = -1
			firstBatch := true

			for {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
				}

				var query string
				var rows *sql.Rows
				var err error

				if firstBatch {
					query = fmt.Sprintf("SELECT %s FROM `%s`.`%s` ORDER BY `%s` ASC LIMIT %d;", colsList, table.Schema, table.Name, pkCol, batchSize)
					rows, err = r.db.QueryContext(ctx, query)
					firstBatch = false
				} else {
					query = fmt.Sprintf("SELECT %s FROM `%s`.`%s` WHERE `%s` > ? ORDER BY `%s` ASC LIMIT %d;", colsList, table.Schema, table.Name, pkCol, pkCol, batchSize)
					rows, err = r.db.QueryContext(ctx, query, lastID)
				}

				if err != nil {
					errChan <- fmt.Errorf("reading mysql rows from %s: %w", table.Name, err)
					return
				}

				var batchRows []database.Row
				for rows.Next() {
					vals := make([]any, len(table.ColumnOrder))
					valPtrs := make([]any, len(table.ColumnOrder))
					for i := range vals {
						valPtrs[i] = &vals[i]
					}

					if err := rows.Scan(valPtrs...); err != nil {
						rows.Close()
						errChan <- fmt.Errorf("scanning mysql row for %s: %w", table.Name, err)
						return
					}

					rowMap := make(map[string]any, len(table.ColumnOrder))
					for i, colName := range table.ColumnOrder {
						v := vals[i]
						if b, ok := v.([]byte); ok {
							rowMap[colName] = string(b)
						} else {
							rowMap[colName] = v
						}
						if colName == pkCol {
							switch idV := v.(type) {
							case int64:
								lastID = idV
							case int:
								lastID = int64(idV)
							case []byte:
								var parsed int64
								fmt.Sscanf(string(idV), "%d", &parsed)
								lastID = parsed
							}
						}
					}
					batchRows = append(batchRows, database.Row{Values: rowMap})
				}
				rows.Close()

				if len(batchRows) == 0 {
					break
				}

				batchChan <- database.Batch{
					TableName: table.Name,
					Columns:   table.ColumnOrder,
					Rows:      batchRows,
				}
			}
			return
		}

		// Fallback to offset pagination
		orderBy := ""
		if table.PrimaryKey != nil && len(table.PrimaryKey.Columns) > 0 {
			pkCols := make([]string, len(table.PrimaryKey.Columns))
			for i, p := range table.PrimaryKey.Columns {
				pkCols[i] = fmt.Sprintf("`%s`", p)
			}
			orderBy = fmt.Sprintf("ORDER BY %s", strings.Join(pkCols, ", "))
		}

		offset := int64(0)
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			default:
			}

			query := fmt.Sprintf("SELECT %s FROM `%s`.`%s` %s LIMIT %d OFFSET %d;", colsList, table.Schema, table.Name, orderBy, batchSize, offset)
			rows, err := r.db.QueryContext(ctx, query)
			if err != nil {
				errChan <- fmt.Errorf("reading mysql rows from %s: %w", table.Name, err)
				return
			}

			var batchRows []database.Row
			for rows.Next() {
				// Allocate pointers for scanning arbitrary columns
				vals := make([]any, len(table.ColumnOrder))
				valPtrs := make([]any, len(table.ColumnOrder))
				for i := range vals {
					valPtrs[i] = &vals[i]
				}

				if err := rows.Scan(valPtrs...); err != nil {
					rows.Close()
					errChan <- fmt.Errorf("scanning mysql row for %s: %w", table.Name, err)
					return
				}

				rowMap := make(map[string]any, len(table.ColumnOrder))
				for i, colName := range table.ColumnOrder {
					// Handle []byte values returned by MySQL driver for strings/blobs
					v := vals[i]
					if b, ok := v.([]byte); ok {
						rowMap[colName] = string(b)
					} else {
						rowMap[colName] = v
					}
				}
				batchRows = append(batchRows, database.Row{Values: rowMap})
			}
			rows.Close()

			if len(batchRows) == 0 {
				break
			}

			batchChan <- database.Batch{
				TableName: table.Name,
				Columns:   table.ColumnOrder,
				Rows:      batchRows,
			}

			offset += int64(len(batchRows))
		}
	}()

	return batchChan, errChan
}
