//
package raft

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"os"
	"time"
)

const HEAD = "HEAD"

var (
	rootAnchorSig string
	outChan       chan string
)

//=====================================================================

func init() {
	outChan = make(chan string)
	go func() {
		for {
			s := <-outChan
			os.Stderr.WriteString(s)
		}
	}()
}

func PrintLevel(level int, s string, args ...interface{}) {
	if level <= DebugLevel {
		PrintAlways(s, args...)
	}
}

func PrintOne(s string, args ...interface{}) {
	PrintLevel(1, s, args...)
}

func PrintTwo(s string, args ...interface{}) {
	PrintLevel(2, s, args...)
}

func PrintThree(s string, args ...interface{}) {
	PrintLevel(3, s, args...)
}

func PrintFive(s string, args ...interface{}) {
	PrintLevel(3, s, args...)
}

func PrintDebug(s string, args ...interface{}) {
	PrintLevel(1, s, args...)
}

func PrintExit(s string, args ...interface{}) {
	PrintAlways(s, args...)
	os.Exit(1)
}

func PrintAlways(s string, args ...interface{}) {
	outChan <- fmt.Sprintf(s, args...)
}

func MakeError(s string, args ...interface{}) error {
	return errors.New(fmt.Sprintf(s, args...))
}

func PrintAssert(cond bool, s string, args ...interface{}) {
	if !cond {
		PrintExit(s, args...)
	}
}

func PrintNotAssert(cond bool, s string, args ...interface{}) {
	if cond {
		PrintExit(s, args...)
	}
}

//=====================================================================

func p_time(s string) (time.Time, bool) {
	timeFormats := []string{"2006-1-2 15:04:05", "2006-1-2 15:04", "2006-1-2", "1-2-2006 15:04:05",
		"1-2-2006 15:04", "1-6-2006", "2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2",
		"1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006"}
	loc, _ := time.LoadLocation("Local")

	for _, v := range timeFormats {
		if tm, terr := time.ParseInLocation(v, s, loc); terr == nil {
			return tm, false
		}
	}
	return time.Time{}, true
}

// Parse a local time string in the below format.
func parseLocalTime(tm string) (time.Time, error) {
	loc, _ := time.LoadLocation("America/New_York")
	return time.ParseInLocation("2006-1-2T15:04", tm, loc)
}

//=============================================================================

// return base32 (stringified) version of sha1 hash of array of bytes
// Prefixes '_' if JSON string.
func computeSig(buf []byte) string {
	sha := sha256.Sum256(buf)
	shasl := sha[:]
	return "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)
}
