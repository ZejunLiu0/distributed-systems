package store

// import (
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// 	"strings"
// )

const (
	NUM_CHILDREN = 32
	encodeStd    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

type TreeNode struct {
	Sig       string
	ChildSigs []string
	ChunkSigs []string
	children  []*TreeNode
}

var (
	reverseEncode = map[string]int{}
	treeRoots     = map[string]*TreeNode{}
)
