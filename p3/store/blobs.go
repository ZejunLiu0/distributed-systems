package store

import (
	"os"
	"time"
)

// import (
// 	"crypto/sha256"
// 	"encoding/base32"
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"path/filepath"
// 	"time"
// )

//=====================================================================

type ObjectFile struct {
	Version int
	Type    string
	Name    string
	ModTime time.Time
	Mode    os.FileMode
	Data    []string
	data    []byte
}

type ObjectDir struct {
	Version   int
	Type      string
	Name      string
	PrevSig   string
	FileNames []string
	FileSigs  []string
}

var BLOB_DIR = os.Getenv("HOME") + "/.blobstore/"
