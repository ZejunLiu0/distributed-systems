package main

import (
	"flag"

	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p4/store"
)

//=====================================================================

func main() {

	port := flag.String("s", "", "serve port")
	publicKeyFile := flag.String("p", "", "public key file")
	dbfile := flag.String("b", ".chunkdb", "dbFileName")
	Debug = *flag.Bool("d", false, "debug mode")
	NeedSigp := flag.Bool("x", false, "verify anchor/claim files")
	flag.Parse()

	if *port == "" {
		PrintExit("Must specify server port, e.g.: 'serve 8080'\n")
	}
	if *NeedSigp && *publicKeyFile == "" {
		PrintExit("-x should be used with a public key\n")
	}

	CreateDB(*dbfile)
	Serve("localhost:"+*port, *publicKeyFile, *NeedSigp)
}
