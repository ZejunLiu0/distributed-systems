package main

import (
	"os"

	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p2/store"
)

func main() {

	if len(os.Args) < 3 {
		usage()
	}

	_, args := os.Args[0], os.Args[1:]

	if len(args[0]) > 0 && args[0] == "-d" {
		args = args[1:]
		Debug = !Debug
	}
	if len(args) < 2 {
		usage()
	}
	addr, cmd, args := args[0], args[1], args[2:]
	ServerAddress = addr
	println("ServerAddress:", ServerAddress)

	switch cmd {
	case "get":
		PrintAssert(len(args) >= 2, "USAGE: get <sig> <path>\n")
		CmdGet(args[0], args[1])

	case "getfile":
		PrintAssert(len(args) >= 2, "USAGE: get <sig> <path>\n")
		CmdGetFile(args[0], args[1])

	case "getsig":
		PrintAssert(len(args) >= 2, "USAGE: get <sig> <path>\n")
		CmdGetFileNoJSON(args[0], args[1])

	case "put":
		PrintAssert(len(args) >= 1, "USAGE: put <sig>\n")
		CmdPut(args[0])

	case "desc":
		PrintAssert(len(args) >= 1, "USAGE: desc <sig>\n")
		CmdDesc(args[0])

	default:
		usage()
	}
}

func usage() {
	PrintExit("USAGE: client <serveraddr> (put <path> | putfile <path> | getsig <sig> <new path> | desc <sig>)\n")
}
