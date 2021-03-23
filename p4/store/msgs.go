package store

import (
	// "bytes"
	// "encoding/json"
	// "io/ioutil"
	// "net/http"
	"os"
	"time"
)

type Message struct {
	Version int
	Type    string
	Sig     string
	Data    []byte
	Name    string
	ModTime time.Time
	Mode    os.FileMode

	TreeSig    string
	TreeHeight int
	TreeTarget string
	Node       *TreeNode
	Info       string

	RandomID string            // 256 bits, always unique, only anchor
	Refsig   string            // anchor's hash
	Prevsig  string            // hash of previous link in chain
	Chain    []string          // return hashes of entire chain
	Adds     map[string]string // for claims
}
