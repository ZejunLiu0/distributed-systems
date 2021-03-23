package store

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
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
	db.Exec("CREATE TABLE IF NOT EXISTS anchors (name CHAR(70) PRIMARY KEY, sig CHAR(70), data BLOB)")
	db.Exec("CREATE TABLE IF NOT EXISTS claims (refsig CHAR(70), prevsig CHAR(70), sig CHAR(70) PRIMARY KEY, isLast bool, id int, data BLOB)")

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
