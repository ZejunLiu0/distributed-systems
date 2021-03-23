package store

import (
	"fmt"
	"os"
)

// import (
// 	"bytes"
// 	"fmt"
// 	"log"
// 	"os"
// )

var (
	Debug = true
)

func PrintAssert(cond bool, s string, args ...interface{}) {
	if !cond {
		PrintExit(s, args...)
	}
}

func PrintDebug(s string, args ...interface{}) {
	if !Debug {
		return
	}
	PrintAlways(s, args...)
}

func PrintExit(s string, args ...interface{}) {
	PrintAlways(s, args...)
	os.Exit(1)
}

func PrintAlways(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}
