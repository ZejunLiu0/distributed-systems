//
package main

/*

 */

import (
	// "bufio"
	"gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p6/raft"
	// "os"
	// "strings"
)

//=============================================================================

func main() {
	go raft.Init()
	// raft.Init()

	/*	reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			text = strings.Trim(text, "\n")

			switch text {
			case "acquire":
				raft.AcquireLock()
			case "release":
				raft.ReleaseLock()
			default:
				raft.Command(text)
			}
		}
	*/
	ch := make(chan int)

	item := <-ch
	raft.PrintAlways("Done: %v\n", item)
}
