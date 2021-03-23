// you may make any changes here. Code below are merely suggestions.
package raft

import (
	// "encoding/json"

	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/mattn/go-getopt"
)

const (
	FOLLOWER = iota
	CANDIDATE
	LEADER
)

const (
	stateAbbvs = "fcl"
)

type LogEntry struct {
	Term int
	Cmd  string
}

var (
	heartbeatTimeout          = 2 * time.Second
	electionTimeout           = 4 * heartbeatTimeout
	electionTimeoutStochastic = 4 * heartbeatTimeout

	// persistent
	state        int = FOLLOWER
	currentTerm  int
	votedFor     int
	votedForTerm int // used either for voting for another candidate, or when I'm a candidate
	votesForMe   int
	sharedLog    []LogEntry
	commitIndex  = -1
	logSizes     = make(map[int]int)
	Debug        = false
	DebugLevel   = 1
	cmdIndex     = 0

	dropping       = false
	msgChannel     chan string
	electionTimer  = time.NewTimer(electionTimeout + time.Duration(float32(electionTimeoutStochastic)*rand.Float32()))
	heartbeatTimer = time.NewTimer(heartbeatTimeout)

	mepid             int
	groupMembers      = make(map[int]*Replica)
	mutex             = &sync.Mutex{}
	waitingForEntries = false
	checkingConflict  = false
	needConflictCheck = true // turns false once passed
	committing        = false
	Committed         chan bool

	termInLog int
)

//=====================================================================

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func candidateMsgHandler(m *Message) {
	checkingConflict = false
	// if (m.CurrentIndex < commitIndex && m.Term-1 == currentTerm) || m.Term < currentTerm {
	// 	return
	// }
	fmt.Println("my termInLog:", termInLog, "msg termInLog:", m.TermInLog)
	if m.Term < currentTerm {
		return
	}
	if termInLog == m.TermInLog && m.CurrentIndex < commitIndex {
		return
	}

	// if votedFor == 0 || m.Term > currentTerm || m.CurrentIndex >= commitIndex {
	if votedFor == 0 || votedFor == mepid {
		// Vote
		state = FOLLOWER
		res := &Message{
			Type:         MSG_CANDIDATE_REPLY,
			From:         mepid,
			To:           m.From,
			CurrentIndex: commitIndex,
		}
		// lock
		resetElectionTimeout()
		err := SendRep(groupMembers[m.From], res)
		if err == nil {
			votedFor = m.From
			votedForTerm = m.Term
			currentTerm = m.Term
			termInLog = m.Term
		} else {
			panic(err)
		}
		// unlock
	}
}

func candidateReplyMsgHandler(m *Message) {
	votesForMe++
	if votesForMe > (len(groupMembers)+1)/2 {
		state = LEADER
		// go heartbeatChecker()
		// fmt.Println("I'm leader now")
		votesForMe = 0

		entry := LogEntry{
			Cmd:  "leader" + strconv.Itoa(mepid),
			Term: currentTerm,
		}

		// mutex.Lock()
		sharedLog = append(sharedLog, entry)
		// mutex.Unlock()

		var sendEntries []LogEntry

		sendEntries = append(sendEntries, entry)

		committing = true

		if commitIndex != -1 {
			res := &Message{
				Type:          MSG_APPEND_ENTRIES,
				Term:          currentTerm,
				Entries:       sendEntries,
				From:          mepid,
				CurrentIndex:  commitIndex,
				CommitRequest: true,
				Index:         commitIndex + 1,
				CheckConflict: true,
				CheckedEntry:  sharedLog[commitIndex],
			}
			SendAll(false, res)
		} else {
			res := &Message{
				Type:          MSG_APPEND_ENTRIES,
				Term:          currentTerm,
				Entries:       sendEntries,
				From:          mepid,
				CurrentIndex:  commitIndex,
				CommitRequest: true,
				Index:         commitIndex + 1,
			}
			SendAll(false, res)
		}
		go heartbeatChecker()
	}
}

//=====================================================================

func heartbeat(cmd string) {
	resetHeartbeatTimeout()
	resetElectionTimeout()
}

func resetHeartbeatTimeout() {
	heartbeatTimer.Reset(heartbeatTimeout)
}

func resetElectionTimeout() {
	electionTimer.Reset(electionTimeout + time.Duration(float32(electionTimeoutStochastic)*rand.Float32()))
}

func heartbeatChecker() {
	// for leader, send out heartbeat periodically,
	for state == LEADER {
		heartbeat("")
		<-heartbeatTimer.C

		msg := &Message{
			Type:         MSG_APPEND_ENTRIES,
			From:         mepid,
			Term:         currentTerm,
			CurrentIndex: commitIndex,
		}
		SendAll(false, msg)
	}
}

func electionChecker() {
	// for followers, election timeout -> become a candidate -> vote for self, request vote to other nodes
	for state != LEADER {
		PrintThree("electionChecker: follower\n")
		<-electionTimer.C
		PrintThree("electionChecker: election timeout\n")

		state = CANDIDATE
		currentTerm++
		votesForMe = 1
		votedForTerm = currentTerm
		votedFor = mepid

		msg := &Message{
			Type:         MSG_CANDIDATE,
			From:         mepid,
			Term:         currentTerm,
			CurrentIndex: commitIndex,
			TermInLog:    termInLog,
		}

		SendAll(false, msg)

		resetElectionTimeout()
	}

}

func RemoveIndex(s []LogEntry, index int) []LogEntry {
	return append(s[:index], s[index+1:]...)
}

func appendEntriesMsgHandler(m *Message) {
	// PrintFive("AE handler INCOMING %d entries, commit %d, term %d, LastLogIndex %d, LastLogTerm %d: LOCAL %d entries, commit %d, term %d\n",
	// 	len(m.Entries), m.LeaderCommit, m.Term, m.LastLogIndex, m.LastLogTerm, len(sharedLog), commitIndex, currentTerm)

	state = FOLLOWER
	resetElectionTimeout()

	votedFor = 0
	currentTerm = m.Term
	termInLog = m.Term
	// fmt.Println("needConflictCheck?", needConflictCheck)

	if committing && !m.CommitIndex {
		return
	}

	if (len(m.Entries) != 0 || needConflictCheck) && m.CurrentIndex < commitIndex {
		// fmt.Println("m.CurrentIndex:", m.CurrentIndex, "commitIndex:", commitIndex)

		// if len(m.Entries) != 0 && strings.HasPrefix(m.Entries[0].Cmd, "leader") && m.CurrentIndex < commitIndex {
		commitIndex = m.CurrentIndex

		mutex.Lock()
		for len(sharedLog) != commitIndex+1 {
			sharedLog = RemoveIndex(sharedLog, len(sharedLog)-1)
		}
		mutex.Unlock()

	}

	if m.CheckConflict || m.ResolveConflict || len(sharedLog) == 0 {
		needConflictCheck = false
	}
	// conflict check on first discovered node
	if needConflictCheck && !m.CheckConflict {
		PrintTwo("Need to check conflict")
		res := &Message{
			Type:          MSG_APPEND_ENTRIES_REPLY,
			Term:          currentTerm,
			From:          mepid,
			CurrentIndex:  commitIndex,
			Conflict:      true,
			ConflictTerm:  sharedLog[len(sharedLog)-1].Term,
			ConflictIndex: len(sharedLog) - 1,
		}
		SendRep(groupMembers[m.From], res)
		return
	}

	// fmt.Println("checkingConflict:", checkingConflict, "m.ResolveConflict:", m.ResolveConflict, "m.CheckConflict:", m.CheckConflict)
	if checkingConflict && !m.ResolveConflict && !m.CheckConflict {
		return
	}

	if m.CheckConflict && len(sharedLog) != 0 {
		checkingConflict = true

		var checkedEntryIdx int
		if commitIndex < m.CurrentIndex {
			checkedEntryIdx = commitIndex
		} else {
			checkedEntryIdx = len(sharedLog) - 1
		}

		if checkedEntryIdx > 0 && m.CheckedEntry.Term != sharedLog[checkedEntryIdx].Term || m.CheckedEntry.Cmd != sharedLog[checkedEntryIdx].Cmd {
			conflictIdx := -1
			conflictTerm := sharedLog[checkedEntryIdx].Term
			for i := len(sharedLog) - 1; i >= 0; i-- {
				if sharedLog[i].Term == conflictTerm {
					conflictIdx = i
					sharedLog = RemoveIndex(sharedLog, i)
					commitIndex--
				} else {
					break
				}
			}
			conflictIdx--

			// PrintTwo("next conflict index:", conflictIdx)

			res := &Message{
				Type:          MSG_APPEND_ENTRIES_REPLY,
				Term:          currentTerm,
				From:          mepid,
				CurrentIndex:  commitIndex,
				Conflict:      true,
				ConflictTerm:  conflictTerm,
				ConflictIndex: conflictIdx,
			}
			SendRep(groupMembers[m.From], res)
			return
		}
	}

	checkingConflict = false
	if waitingForEntries && !m.LogReplication {
		return
	}

	if m.LogReplication && m.CurrentIndex > commitIndex && !committing {
		// PrintTwo("AE: LogReplication, my index:", commitIndex, "m.CurrentIndex:", m.CurrentIndex)
		for _, e := range m.Entries {
			sharedLog = append(sharedLog, e)
		}
		waitingForEntries = false
		commitIndex = m.CurrentIndex
		res := &Message{
			Type:         MSG_APPEND_ENTRIES_REPLY,
			To:           m.From,
			From:         mepid,
			CurrentIndex: commitIndex,
		}
		SendRep(groupMembers[m.From], res)

		return
	}

	// Asking for entries
	if commitIndex < m.CurrentIndex && !m.CommitIndex && !committing {
		// if commitIndex < m.CurrentIndex && len(sharedLog) != m.CurrentIndex {
		waitingForEntries = true
		res := &Message{
			Type:           MSG_APPEND_ENTRIES_REPLY,
			From:           mepid,
			LogReplication: true,
			CurrentIndex:   commitIndex,
		}
		SendRep(groupMembers[m.From], res)
		return
	}

	// Prepare to commit
	if m.CommitRequest {

		// fmt.Println("AE: ReadyToCommit, my index:", commitIndex)

		for _, e := range m.Entries {
			sharedLog = append(sharedLog, e)
		}
		res := &Message{
			Type:          MSG_APPEND_ENTRIES_REPLY,
			From:          mepid,
			ReadyToCommit: true,
			Index:         m.Index,
			CurrentIndex:  commitIndex,
			S:             m.S,
		}

		committing = true
		SendRep(groupMembers[m.From], res)

		return
	}

	if m.CommitIndex && m.CurrentIndex == commitIndex+1 {
		// fmt.Println("AE: CommitIndex")
		if len(sharedLog)-1 < m.Index {
			waitingForEntries = true
			res := &Message{
				Type:           MSG_APPEND_ENTRIES_REPLY,
				From:           mepid,
				LogReplication: true,
				CurrentIndex:   commitIndex,
			}
			SendRep(groupMembers[m.From], res)
			return
		}
		mutex.Lock()
		if m.Index > commitIndex {
			commitIndex = m.Index
			sharedLog[commitIndex].Cmd = strings.ToUpper(sharedLog[commitIndex].Cmd)
		}
		mutex.Unlock()

		committing = false
		res := &Message{
			Type:         MSG_APPEND_ENTRIES_REPLY,
			To:           m.From,
			From:         mepid,
			CurrentIndex: commitIndex,
		}

		SendRep(groupMembers[m.From], res)

		return
	}

	res := &Message{
		Type:         MSG_APPEND_ENTRIES_REPLY,
		To:           m.From,
		From:         mepid,
		CurrentIndex: commitIndex,
	}

	SendRep(groupMembers[m.From], res)

}

func appendEntriesReplyMsgHandler(m *Message) {
	heartbeat("")

	if m.Conflict {
		var sendEntries []LogEntry
		if m.ConflictIndex == -1 {
			for i := 0; i < len(sharedLog); i++ {
				sendEntries = append(sendEntries, sharedLog[i])
			}
			res := &Message{
				Type:            MSG_APPEND_ENTRIES,
				Term:            currentTerm,
				From:            mepid,
				CurrentIndex:    commitIndex,
				LogReplication:  true,
				ResolveConflict: true,
				Entries:         sendEntries,
			}
			SendRep(groupMembers[m.From], res)
			resetHeartbeatTimeout()

			return
		}
		res := &Message{
			Type:          MSG_APPEND_ENTRIES,
			Term:          currentTerm,
			From:          mepid,
			CurrentIndex:  commitIndex,
			CheckConflict: true,
			CheckedEntry:  sharedLog[m.ConflictIndex],
		}

		SendRep(groupMembers[m.From], res)
		resetHeartbeatTimeout()

		return
	}

	// have uncommited data
	if m.ReadyToCommit {
		mutex.Lock()
		if m.Index > commitIndex {
			commitIndex = m.Index
			// commit log entry
			sharedLog[commitIndex].Cmd = strings.ToUpper(sharedLog[commitIndex].Cmd)
		}

		// tell followers to commit
		res := &Message{
			Type:         MSG_APPEND_ENTRIES,
			Term:         currentTerm,
			From:         mepid,
			CommitIndex:  true,
			Index:        commitIndex,
			CurrentIndex: commitIndex,
		}
		mutex.Unlock()
		committing = false
		if m.S != "" {
			select {
			case Committed <- true:
				// fmt.Println("Send committed")
			default:
				// fmt.Println("no message sent")
			}

		}

		resetHeartbeatTimeout()
		SendAll(false, res)

	} else if m.LogReplication {

		var sendEntries []LogEntry
		for i := m.CurrentIndex + 1; i <= commitIndex; i++ {
			sendEntries = append(sendEntries, sharedLog[i])
		}
		res := &Message{
			Type:           MSG_APPEND_ENTRIES,
			Term:           currentTerm,
			Entries:        sendEntries,
			From:           mepid,
			CurrentIndex:   commitIndex,
			LogReplication: true,
		}
		resetHeartbeatTimeout()
		SendRep(groupMembers[m.From], res)

	} else if len(sharedLog)-1 > commitIndex {
		res := &Message{
			Type:          MSG_APPEND_ENTRIES,
			Term:          currentTerm,
			From:          mepid,
			CommitRequest: true,
			Index:         commitIndex + 1,
			CurrentIndex:  commitIndex,
		}
		SendAll(false, res)
		resetHeartbeatTimeout()

	}

}

func serve() {
	serverAddress := merep.addr
	PrintLevel(3, "serverAddress:", serverAddress)

	http.HandleFunc("/json", msgHandler)

	PrintExit("\nResult of listen: %v\n", http.ListenAndServe(serverAddress, nil))
}

func msgHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// case http.MethodPut:
		var msg2 Message
		buf, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(buf, &msg2)
		defer r.Body.Close()

		mutex.Lock()
		if msg2.Type != MSG_COMMAND {
			if len(msg2.Entries) == 0 {
				PrintOne(fmt.Sprintf("recv %s\tfrom\t%d: () [%s]\n", msgtypes[msg2.Type], msg2.From, fmtEntry(sharedLog)))
				// color.Green(fmt.Sprintf("recv %s\tfrom\t%d: () %v\n", msgtypes[msg2.Type], msg2.From, sharedLog))
			} else {
				// color.Green(fmt.Sprintf("recv %s\tfrom\t%d: (%s) [%s]\n", msgtypes[msg2.Type], msg2.From, fmtEntry(msg2.Entries), fmtEntry(sharedLog)))
				PrintOne(fmt.Sprintf("recv %s\tfrom\t%d: (%s) [%s]\n", msgtypes[msg2.Type], msg2.From, fmtEntry(msg2.Entries), fmtEntry(sharedLog)))
			}
		} else if state == LEADER {
			if len(msg2.Entries) == 0 {
				PrintOne(fmt.Sprintf("recv %s\tfrom\t%d: () [%s]\n", msgtypes[msg2.Type], msg2.From, fmtEntry(sharedLog)))
				// color.Green(fmt.Sprintf("recv %s\tfrom\t%d: () %v\n", msgtypes[msg2.Type], msg2.From, sharedLog))
			} else {
				// color.Green(fmt.Sprintf("recv %s\tfrom\t%d: (%v) %v\n", msgtypes[msg2.Type], msg2.From, msg2.Entries, sharedLog))
				PrintOne(fmt.Sprintf("recv %s\tfrom\t%d: (%s) [%s]\n", msgtypes[msg2.Type], msg2.From, fmtEntry(msg2.Entries), fmtEntry(sharedLog)))
			}
		}
		mutex.Unlock()

		// fmt.Printf("get msg:  %+v\n", msg2)

		switch msg2.Type {

		case MSG_CANDIDATE:
			candidateMsgHandler(&msg2)

		case MSG_CANDIDATE_REPLY:
			candidateReplyMsgHandler(&msg2)

		case MSG_APPEND_ENTRIES:
			appendEntriesMsgHandler(&msg2)
		case MSG_APPEND_ENTRIES_REPLY:
			appendEntriesReplyMsgHandler(&msg2)

		case MSG_COMMAND:
			if state == LEADER {
				// fmt.Println("MSG_COMMAND:", msg2)
				entry := LogEntry{
					Term: currentTerm,
					Cmd:  msg2.S,
				}
				mutex.Lock()
				sharedLog = append(sharedLog, entry)
				mutex.Unlock()

				// broadcast
				m := &Message{
					Type:          MSG_APPEND_ENTRIES,
					Term:          currentTerm,
					Entries:       []LogEntry{entry},
					From:          mepid,
					CommitRequest: true,
					Index:         commitIndex + 1,
					CurrentIndex:  commitIndex,
					S:             msg2.S,
				}

				SendAll(false, m)

				<-Committed
				// fmt.Println("MSG_COMMAND committed")

				resp := &Message{
					From:                mepid,
					ClientRespCommitted: true,
				}
				b, _ := json.Marshal(resp)

				req, _ := http.NewRequest("POST", "http://"+msg2.ClientAddr+"/json", bytes.NewBuffer(b))
				// req.Close = true

				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{}
				client.Do(req)

			}
		}
	}
}

func printLog() {
}

//=====================================================================

func usage() {
	println("usage: replica.go [-m <mode> | -n | -r <replica string>]")
	os.Exit(1)
}

func Init() {
	var (
		c             int
		replicaString = "auto"
	)

	for {
		if c = Getopt("dD:m:nr:h:t:"); c == EOF {
			break
		}

		switch c {
		case 'd':
			Debug = !Debug

		case 'D':
			DebugLevel, _ = strconv.Atoi(OptArg)

		case 'h':
			t, _ := strconv.Atoi(OptArg)
			heartbeatTimeout = time.Duration(t) * time.Second
			electionTimeout = 2 * heartbeatTimeout
			electionTimeoutStochastic = 2 * heartbeatTimeout

		case 'r':
			replicaString = OptArg

		default:
			usage()
		}
	}

	if Debug && DebugLevel == 0 {
		DebugLevel = 2
	}
	log := LoadConfig(replicaString, "config.txt")
	for _, s := range log {
		flds := strings.Split(s, ":")
		if len(flds) != 2 {
			PrintExit("Bad starting log element %q\n", s)
		}
		term, _ := strconv.Atoi(flds[0])
		currentTerm = term
		sharedLog = append(sharedLog, LogEntry{Cmd: flds[1], Term: term})
	}
	termInLog = currentTerm

	PrintNotAssert(merep == nil, "Cannot find my name (%q) in config!\n", replicaString)
	commitIndex = len(sharedLog) - 1

	logSizes = make(map[int]int)

	resetElectionTimeout()

	msgChannel = make(chan string)
	Committed = make(chan bool, 1)
	go serve()
	go heartbeatChecker()
	go electionChecker()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ch
		printStats()
		os.Exit(1)
	}()
	fmt.Printf("Startup up with debug %d, replica: %d (%s): %s\n\n", DebugLevel, mepid, replicaString, merep.addr)
}

func printStats() {
	// ....
}
