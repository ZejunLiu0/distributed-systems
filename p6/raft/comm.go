/*
   What this code shows:
   - how to initialize and connect REQ/REP and PUB/SUB sockets
   - a way to differentiate ports
   - sending/receiving on different types of ZMQ sockets
   - parsing of required command-line arguments
   - parsing of config file.
*/

package raft

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

// "bytes"
// "encoding/json"
// "net/http"

// The REQ port is the default, others are offset
func RAFT_REPLICA_PORT(x int) int { return x }
func RAFT_CLIENT_PORT(x int) int  { return x + 1 }

const (
	_ = iota // I want the constants to start at "1"
	MSG_CANDIDATE
	MSG_CANDIDATE_REPLY
	MSG_APPEND_ENTRIES
	MSG_APPEND_ENTRIES_REPLY
	MSG_COMMAND
	MSG_DROP_MSGS
	MSG_NO_DROP_MSGS
)

var msgtypes = map[int]string{
	MSG_CANDIDATE:            "Candidate     ",
	MSG_CANDIDATE_REPLY:      "CandidateReply",
	MSG_APPEND_ENTRIES:       "AE            ",
	MSG_APPEND_ENTRIES_REPLY: "AEreply       ",
	MSG_COMMAND:              "Cmd           ",
	MSG_DROP_MSGS:            "Drop          ",
	MSG_NO_DROP_MSGS:         "NoDrop        ",
}

// All of this is application-specific. You may have to augment this...
type Message struct {
	Type         int
	S            string // from client
	Term         int    // current term
	Entries      []LogEntry
	From         int
	To           int
	CurrentIndex int // current latest commited index

	CommitRequest bool // leader->follower
	ReadyToCommit bool // follower->leader
	CommitIndex   bool // leader->followers
	Index         int

	LogReplication bool

	CheckConflict   bool
	ResolveConflict bool
	CheckedEntry    LogEntry
	Conflict        bool
	ConflictTerm    int
	ConflictIndex   int

	ClientRespCommitted bool
	ClientAddr          string

	TermInLog int
}

var ()

func init() {
	// msgmutex = &sync.Mutex{}
	// msgTypeSends = make(map[int]int)
	// msgTypeRecvs = make(map[int]int)
}

//=====================================================================

// assumes msg small, want msgs to be sent immediately without expecting or waiting for answers
func SendAll(sync bool, m *Message) error {
	if sync {
		failed := 0
		for _, rep := range groupMembers {
			err := SendRep(rep, m)
			if err != nil {
				failed++
				if failed > 1 {
					return errors.New("Commit failed!\n")
				}
			}
		}
	} else {
		for _, rep := range groupMembers {
			go SendRep(rep, m)
		}
	}
	return nil
}

func PidToReplica(pid int) *Replica {
	return groupMembers[pid]
}

func Send(toPid int, m *Message) error {
	m.From = mepid
	m.To = toPid
	rep := PidToReplica(toPid)
	if rep == nil {
		PrintAlways("SEND ERROR failed to find rep %d\n", toPid)
		return nil
	}
	return SendRep(rep, m)
}

func fmtEntry(entries []LogEntry) string {
	var re string
	for _, e := range entries {
		re += strconv.Itoa(e.Term)
		re += ":"
		re += e.Cmd
		re += ","
	}
	if re != "" {
		re = re[:len(re)-1]
	}

	return re
}

func SendRep(rep *Replica, m *Message) error {
	if m.Type != MSG_COMMAND {
		mutex.Lock()
		if len(m.Entries) == 0 {
			// color.Red(fmt.Sprintf("send %s\tto\t%d: () %v\n", msgtypes[m.Type], rep.pid, sharedLog))
			PrintOne(fmt.Sprintf("send %s\tto\t%d: () [%s]\n", msgtypes[m.Type], rep.pid, fmtEntry(sharedLog)))
		} else {
			// color.Red(fmt.Sprintf("send %s\tto\t%d: (%v) %v\n", msgtypes[m.Type], rep.pid, m.Entries, sharedLog))
			PrintOne(fmt.Sprintf("send %s\tto\t%d: (%s) [%s]\n", msgtypes[m.Type], rep.pid, fmtEntry(m.Entries), fmtEntry(sharedLog)))
		}
		mutex.Unlock()

	}
	// fmt.Printf("send msg: %+v\n", m)

	b, err := json.Marshal(m)

	req, _ := http.NewRequest("POST", "http://"+rep.addr+"/json", bytes.NewBuffer(b))
	// req.Close = true

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		b, _ := ioutil.ReadAll(res.Body)

		os.Stderr.WriteString(fmt.Sprint(res.StatusCode) + ": " + string(b) + "\n")
		return errors.New(fmt.Sprintf("Replica %d failed to commit!", rep.pid))
	}

	return nil

}
