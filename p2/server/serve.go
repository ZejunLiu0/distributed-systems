package main

import (
	"os"

	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p2/store"
)

//=====================================================================

func main() {

	_, args := os.Args[0], os.Args[1:]

	if len(args) > 0 && args[0] == "-d" {
		args = args[1:]
		Debug = !Debug
	}

	if len(args) < 1 {
		PrintExit("Must specify server port, e.g.: 'serve 8080'\n")
	}

	Serve("localhost:" + args[0])
}
