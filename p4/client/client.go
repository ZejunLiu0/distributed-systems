package main

import (
	"flag"
	"fmt"
	"strconv"

	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p4/store"
)

func main() {

	server := flag.String("s", "localhost:8880", "server address")
	qKey := flag.String("q", "", "-q")
	pKey := flag.String("p", "", "-p")

	flag.Parse()

	ServerAddress = *server + "/json"

	tail := flag.Args()
	cmd, args := tail[0], tail[1:]

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
		filename := CmdPut(args[0])
		fmt.Println(filename)

	case "desc":
		PrintAssert(len(args) >= 1, "USAGE: desc <sig>\n")
		CmdDesc(args[0])

	case "del":
		PrintAssert(len(args) >= 1, "USAGE: del <addr> <sig>\n")
		CmdDel(args[0])

	case "info":
		PrintAssert(len(args) >= 0, "USAGE: info <addr>\n")
		CmdInfo()

	case "sync":
		PrintAssert(len(args) >= 2, "USAGE: sync <addr1> <addr2> <height>\n")
		i, _ := strconv.Atoi(args[1])
		PrintAssert(i > 0, "Height must > 0\n")
		CmdSync(args[0], args[1])

	case "genkeys":
		CmdGenkeys()

	case "sign":
		PrintAssert(*qKey != "", "USAGE: -s <serveraddr:port> -q <privateKeyFname> sign <hash>\n")
		PrintAssert(len(args) >= 1, "USAGE: -s <serveraddr:port> -q <privateKeyFname> sign <hash>\n")
		CmdSign(*qKey, args[0])

	case "verify":
		PrintAssert(*pKey != "", "USAGE: -s <serveraddr:port> -p <publicKeyFname> verify <hash>\n")
		PrintAssert(len(args) >= 1, "USAGE: -s <serveraddr:port> -p <publicKeyFname> verify <hash>\n")
		CmdVerify(*pKey, args[0])

	case "anchor":
		// PrintAssert(*qKey != "", "[-s <server:port>] [-q <privatekey.fname>] anchor <rootname>\n")
		PrintAssert(len(args) >= 1, "[-s <server:port>] [-q <privatekey.fname>] anchor <rootname>\n")
		CmdAnchor(*qKey, args[0])

	case "claim":
		// PrintAssert(*qKey != "", "[-s <server:port>] [-q <publickey.fname>]  claim <anchor root name> [<key> <value>]+\n")
		PrintAssert(len(args) >= 3 && args[1] == "rootsig", "[-s <server:port>] [-q <publickey.fname>]  claim <anchor root name> [<key> <value>]+\n")

		CmdClaim(*qKey, args[0], args[1:])

	case "content":
		PrintAssert(len(args) >= 1, "[-s <server:port>] content <rootname>\n")
		CmdContent(args[0])

	case "rootanchor":
		PrintAssert(len(args) >= 1, "[-s <server:port>] rootanchor <rootname>\n")
		CmdRootanchor(args[0])

	case "lastclaim":
		PrintAssert(len(args) >= 1, "[-s <server:port>] lastclaim <rootname>\n")
		CmdLastclaim(args[0])

	case "chain":
		PrintAssert(len(args) >= 1, "[-s <server:port>] chain <claimhash>\n")
		CmdChain(args[0])

	default:
		usage()
	}
}

func usage() {
	PrintExit("USAGE: client <serveraddr> (put <path> | putfile <path> | getsig <sig> <new path> | desc <sig>)\n")
}
