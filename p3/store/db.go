package store

import (
	"database/sql"
	// "encoding/json"
	// "fmt"
	_ "github.com/mattn/go-sqlite3"
	// "net/http"
)

var (
	db  *sql.DB
	err error
)

func CreateDB(path string) {
	if db, err = sql.Open("sqlite3", path); err != nil {
		PrintExit("Open DB %q error: %v\n", path, err)
	}
	db.Exec("CREATE TABLE IF NOT EXISTS chunks (sig CHAR(70) PRIMARY KEY, data BLOB)")

	rows, err := db.Query("SELECT sig, LENGTH(data) FROM chunks")
	if err != nil {
		PrintExit("SELECT DB error: %v\n", err)
	}
	defer rows.Close()

	sigs := 0
	totalSz := 0

	for rows.Next() {
		var sig string
		var sz int
		rows.Scan(&sig, &sz)
		sigs++
		totalSz += sz
		// PrintDebug("db chunk %q, sz %d\n", sig, sz)
	}
	PrintAlways("db %q has %d chunks, %d bytes\n", path, sigs, totalSz)
}
