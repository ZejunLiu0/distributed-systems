package main

import (
	"os"

	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p3/store"
)

//=====================================================================

func main() {

	_, args := os.Args[0], os.Args[1:]
	dbfile := ".chunkdb"

	if len(args) > 0 && args[0] == "-d" {
		args = args[1:]
		Debug = !Debug
	}

	if len(args) < 1 {
		PrintExit("Must specify server port, e.g.: 'serve 8080'\n")
	}

	if len(args) == 2 {
		dbfile = args[1]
	}

	CreateDB(dbfile)
	Serve("localhost:" + args[0])
}
