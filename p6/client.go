//
package main

/*

 */

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p6/raft"
)

var myAddr = "127.0.0.1:8888"

var committed chan bool

//=============================================================================

func main() {
	committed = make(chan bool)
	raft.DebugLevel = 0
	if len(os.Args) < 3 {
		raft.PrintExit("USAGE: go run client.go <repID(,repID)> <msg>\n")
	}

	http.HandleFunc("/json", Handler)
	go http.ListenAndServe(myAddr, nil)

	raft.LoadConfig("0,"+os.Args[1], "config.txt")
	raft.SendAll(true, &raft.Message{Type: raft.MSG_COMMAND, S: os.Args[2], ClientAddr: myAddr})

}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		var msg raft.Message
		buf, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(buf, &msg)
		defer r.Body.Close()
		if msg.ClientRespCommitted {
			fmt.Println("get commited")
		}
	}
}

func usage() {
	fmt.Printf("Usage: client <cmd>\n")
	os.Exit(0)
}
