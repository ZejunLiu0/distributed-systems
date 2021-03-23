package raft

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

type Replica struct {
	pid   int
	mount string
	host  string
	port  int
	addr  string
	live  bool // true after we've received a flush from them
}

var merep *Replica

// returns extra args (starting log)
func LoadConfig(reps, fname string) []string {
	repGroup := strings.Split(reps, ",")
	PrintLevel(3, "repGroup:", repGroup)

	var logEntries []string

	file, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lineArgs := strings.Split(line, ",")
		repNum := lineArgs[0]
		if repGroup[0] == repNum {
			// fmt.Println(lineArgs[1], "port:", lineArgs[2])

			mepid, _ = strconv.Atoi(repNum)
			port, _ := strconv.Atoi(lineArgs[2])
			merep = &Replica{
				pid:  mepid,
				host: lineArgs[1],
				port: port,
				addr: lineArgs[1] + ":" + lineArgs[2],
			}

			for i := 3; i < len(lineArgs); i++ {
				logEntries = append(logEntries, lineArgs[i])
			}

			PrintLevel(3, "merep: ", merep)
			PrintLevel(3, "addr: ", merep.addr)
			PrintLevel(3, "logEntries: ", logEntries)
			break
		}
	}

	for i := 1; i < len(repGroup); i++ {
		found := false
		file.Seek(0, 0)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {

			line := scanner.Text()
			lineArgs := strings.Split(line, ",")
			repNum := lineArgs[0]

			// fmt.Println(repNum, ":", line)
			if repGroup[i] == repNum {
				pid, _ := strconv.Atoi(repNum)
				port, _ := strconv.Atoi(lineArgs[2])

				groupMembers[pid] = &Replica{
					pid:  pid,
					host: lineArgs[1],
					port: port,
					addr: lineArgs[1] + ":" + lineArgs[2],
				}
				found = true
				PrintLevel(3, "\nrepNum: ", repNum)
				PrintLevel(3, "Replica: ", groupMembers[i])

			}
			if found {
				continue
			}
		}

	}
	// fmt.Println("LoadConfig:, groupmembers:", groupMembers)
	return logEntries
}

//=====================================================================
