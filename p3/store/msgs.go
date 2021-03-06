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
	Version    int
	Type       string
	Sig        string
	Data       []byte
	Name       string
	ModTime    time.Time
	Mode       os.FileMode
	TreeSig    string
	TreeHeight int
	TreeTarget string
	Node       *TreeNode
	Info       string
}
