package sqlite3

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/xerrors"

	dbi "github.com/anchore/go-rpmdb/pkg/db"
)

type SQLite3 struct {
	*sql.DB
}

var (
	// https://www.sqlite.org/fileformat.html
	SQLite3_HeaderMagic = []byte("SQLite format 3\x00")
	ErrorInvalidSQLite3 = xerrors.Errorf("invalid or unsupported SQLite3 format")
)

func Open(path string) (*SQLite3, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b := make([]byte, 16)
	if err = binary.Read(file, binary.LittleEndian, b); err != nil {
		return nil, xerrors.Errorf("binary read error: %w", err)
	}

	if !bytes.Equal(b, SQLite3_HeaderMagic) {
		return nil, ErrorInvalidSQLite3
	}

	// open sqlite3 database in read-only mode
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&immutable=1", path))
	if err != nil {
		return nil, xerrors.Errorf("failed to open sqlite3: %w", err)
	}

	return &SQLite3{db}, nil
}

func (db *SQLite3) Read() <-chan dbi.Entry {
	entries := make(chan dbi.Entry)

	go func() {
		defer close(entries)

		rows, err := db.Query("SELECT blob FROM Packages")
		if err != nil {
			entries <- dbi.Entry{
				Err: xerrors.Errorf("failed to SELECT query: %w", err),
			}
		}
		if err := db.Close(); err != nil {
			entries <- dbi.Entry{
				Err: xerrors.Errorf("failed to close DB: %w", err),
			}
		}

		if rows == nil {
			entries <- dbi.Entry{
				Err: xerrors.Errorf("query failed to return rows: %w", err),
			}
			return
		}

		for rows.Next() {
			var blob string
			if err := rows.Scan(&blob); err != nil {
				entries <- dbi.Entry{
					Err: xerrors.Errorf("failed to Scan Row: %w", err),
				}
			}

			entries <- dbi.Entry{
				Value: []byte(blob),
				Err:   nil,
			}
		}
	}()

	return entries
}
