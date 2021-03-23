package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"fmt"
	"testing"
)

var (
	root *TreeNode
)

func TestTree1(t *testing.T) {
	CreateDB("dbTest")

	// the following is duplicated verbatim from func makeTestDB
	db.Exec("DELETE FROM chunks")

	data1 := []byte("very nice")
	data2 := []byte("mediocre")
	data3 := []byte("")
	sig1 := computeSig(data1)
	sig2 := computeSig(data2)
	sig3 := computeSig(data3)
	db.Exec("INSERT INTO chunks (sig, data) VALUES (?,?)", sig1, data1)
	db.Exec("INSERT INTO chunks (sig, data) VALUES (?,?)", sig2, data2)
	db.Exec("INSERT INTO chunks (sig, data) VALUES (?,?)", sig3, data3)
	fmt.Println(sig1)
	fmt.Println(sig2)
	fmt.Println(sig3)

	// Resulting chunks are:
	// sha256_32_GQ3BJPG3QKPX4CVR4OUHNXCUG6QBU3XKHXZ72EBRO5FV42I73FEQ====
	// sha256_32_GKR55CZCIXM62KB4UKGXFJKZKKNZC6I5JOJGUDQGZ6K7DW466ZDA====
	// sha256_32_4OYMIQUY7QOBJGX36TEJS35ZEQT24QPEMSNZGTFESWMRW6CSXBKQ====
	//
	// For reference: 	encodeStd    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

	root = createTree(1, "")
	if root == nil {
		t.Fatalf("tree1 err: %v\n", err)
	}

	if num := len(root.children); num != 0 {
		t.Fatalf("tree1 err #children: %v\n", num)
	}

	if num := len(root.ChunkSigs); num != 3 {
		t.Fatalf("tree1 A err #sigs: %v\n", num)
	}

}

func TestTree2(t *testing.T) {
	CreateDB("dbTest")
	root = createTree(2, "")
	if root == nil {
		t.Fatalf("tree1 err: %v\n", err)
	}

	if num := len(root.children); num != NUM_CHILDREN {
		t.Fatalf("tree1 err #children: %v\n", num)
	}

	if num := len(root.children[6].ChunkSigs); num != 2 {
		t.Fatalf("tree1 A err #sigs: %v\n", num)
	}

}

func TestTree3(t *testing.T) {
	CreateDB("dbTest")
	root = createTree(3, "")
	if root == nil {
		t.Fatalf("tree1 err: %v\n", err)
	}

	if num := len(root.children); num != NUM_CHILDREN {
		t.Fatalf("tree3 root #children: %v\n", num)
	}

	if num := len(root.children[0].children); num != NUM_CHILDREN {
		t.Fatalf("tree3 level 2 #children: %v\n", num)
	}

	if num := len(root.children[6].children[10].ChunkSigs); num != 1 {
		t.Fatalf("tree3 G - K #sigs: %v\n", num)
	}

}

func computeSig(data []byte) string {
	sha := sha256.Sum256(data)
	shasl := sha[:]
	sig := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)
	return sig
}

func createTree(height int, pattern string) *TreeNode {
	if height == 1 {
		var chunksigs []string
		var sigsStr string

		patternStr := "sha256_32_" + pattern + "%"
		fmt.Println(pattern)

		rows, err := db.Query("SELECT sig FROM chunks WHERE sig LIKE ? ORDER BY sig ASC", patternStr)
		if err != nil && err != sql.ErrNoRows {
			PrintExit("SELECT DB error: %v\n", err)
		}
		defer rows.Close()

		for rows.Next() {
			var sig string
			rows.Scan(&sig)
			chunksigs = append(chunksigs, sig)
			sigsStr += sig
			// fmt.Println(sig, patternStr)
		}

		sha := sha256.Sum256([]byte(sigsStr))
		shasl := sha[:]
		nodeSig := base32.StdEncoding.EncodeToString(shasl)

		node := TreeNode{
			Sig:       "sha256_32_" + nodeSig,
			ChunkSigs: chunksigs,
		}
		// m["sha256_32_"+nodeSig] = node
		return &node
	}

	var children []*TreeNode
	var ChildSigs []string
	var sigsStr string

	for i := 0; i < NUM_CHILDREN; i++ {
		chld := createTree(height-1, pattern+string(encodeStd[i]))
		children = append(children, chld)
		ChildSigs = append(ChildSigs, chld.Sig)
		sigsStr += chld.Sig
	}
	sha := sha256.Sum256([]byte(sigsStr))
	shasl := sha[:]
	nodeSig := base32.StdEncoding.EncodeToString(shasl)
	node := TreeNode{
		Sig:       "sha256_32_" + nodeSig,
		ChildSigs: ChildSigs,
		children:  children,
	}
	// m["sha256_32_"+nodeSig] = node
	return &node
}
