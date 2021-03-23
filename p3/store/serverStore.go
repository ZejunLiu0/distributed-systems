package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var myTemplate *template.Template

var ServerAddress = "localhost:8080/json"

var BLOBSTORE = "server_store"

var m map[string]TreeNode
var S_TIMEOUT = 50

type Img struct {
	Name string
	Data string
	Sig  string
}

func Serve(arg string) {
	if _, err := os.Stat(BLOBSTORE); os.IsNotExist(err) {
		os.Mkdir(BLOBSTORE, 0777)
	}
	// tree sig -> node map
	m = make(map[string]TreeNode)
	port := strings.Split(arg, ":")[1]
	println(port)
	http.HandleFunc("/json", Handler)
	http.ListenAndServe(":"+port, nil)
}

// path: sig
func CombineChunks(path string) ([]byte, map[string]interface{}) {
	var content []byte
	row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", path)
	err = row.Scan(&content)
	if err != nil {
		PrintExit("SELECT DB error: %v\n", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(content, &data)
	if err != nil {
		panic(err)
	}

	var byteBuffer, datachunk []byte
	if data["Data"] != nil {
		blobs := data["Data"].([]interface{})

		for i := 0; i < len(blobs); i++ {
			blobName := blobs[i].(string)
			row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", blobName)
			err = row.Scan(&datachunk)
			if err != nil {
				PrintExit("SELECT DB error: %v\n", err)
			}
			byteBuffer = append(byteBuffer, datachunk...)
		}
	}
	return byteBuffer, data
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Non json requests
		sig := r.URL.Path[1:]
		var content []byte

		row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", sig)
		err = row.Scan(&content)
		if err != nil && err != sql.ErrNoRows {
			PrintExit("SELECT DB error: %v\n", err)
		}
		if err == sql.ErrNoRows {
			fmt.Println("Did not find:", sig)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 - Invalid Signature."))
		} else {
			if string(content)[0] == '{' {
				var data map[string]interface{}
				err = json.Unmarshal(content, &data)
				if err != nil {
					panic(err)
				}
				// Is a directory, render html
				if data["Type"].(string) == "dir" {
					myTemplate, err := template.ParseFiles("./index.html")
					if err != nil {
						panic(err)
					}
					length := len(data["FileNames"].([]interface{}))
					var dataSlice []Img
					for i := 0; i < length; i += 4 {
						for j := 0; j < 4; j++ {
							if i+j >= length {
								break
							}
							sig := data["FileSigs"].([]interface{})[i+j].(string)
							// p := filepath.Join(BLOBSTORE, sig)
							name := data["FileNames"].([]interface{})[i+j].(string)
							// println(p)
							byteBuffer, _ := CombineChunks(sig)
							b64Str := base64.StdEncoding.EncodeToString(byteBuffer)
							println("name:", data["FileNames"].([]interface{})[i+j].(string))
							im := Img{Name: name, Data: b64Str, Sig: sig}
							dataSlice = append(dataSlice, im)
						}
						myTemplate.Execute(w, dataSlice)
						dataSlice = nil
					}

				} else if data["Type"].(string) == "file" {
					byteBuffer, data := CombineChunks(sig)
					if byteBuffer == nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					layout := "2006-01-02T15:04:05.999999-07:00"
					modtime, _ := time.Parse(layout, data["ModTime"].(string))
					name := data["Name"].(string)

					contentType := http.DetectContentType(byteBuffer)
					fmt.Println("contentType:", contentType)
					w.Header().Set("Content-Type", contentType)
					http.ServeContent(w, r, name, modtime, bytes.NewReader(byteBuffer))
				}

			} else {
				fmt.Fprintf(w, "Not a recipe: %s", sig)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Signature is not a recipe."))
			}
		}

	case "POST":
		var msg Message
		var buffer []byte

		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch msg.Type {
		case "get":
			// fmt.Println("get:", msg.Sig)

			row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", msg.Sig)
			err = row.Scan(&buffer)
			if err != nil && err != sql.ErrNoRows {
				PrintExit("SELECT DB error: %v\n", err)
			}
			if err == sql.ErrNoRows {
				fmt.Println("Did not find:", msg.Sig)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Signature."))
			} else {
				res := Message{
					Version: 1,
					Type:    "getresp",
					Data:    buffer,
				}
				resjs, err := json.MarshalIndent(res, "", " ")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write(resjs)
			}

		case "getfile":
			// fmt.Println("getfile:", msg.Sig)
			var content []byte
			row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", msg.Sig)
			err = row.Scan(&content)

			if err != nil && err != sql.ErrNoRows {
				PrintExit("SELECT DB error: %v\n", err)
			}
			if err == sql.ErrNoRows {
				fmt.Println("Did not find:", msg.Sig)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Signature."))
			} else {
				var data map[string]interface{}
				err = json.Unmarshal(content, &data)

				// if string(content)[0] != '{' {
				if data["Type"].(string) != "file" {
					fmt.Println("Sig is not a file recipe:", msg.Sig)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("500 - Not a Recipe."))
				} else {
					byteBuffer, data := CombineChunks(msg.Sig)
					if byteBuffer == nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					layout := "2006-01-02T15:04:05.999999-07:00"
					time, _ := time.Parse(layout, data["ModTime"].(string))
					res := Message{
						Version: 1,
						Type:    "getfileresp",
						Data:    byteBuffer,
						ModTime: time,
						Mode:    os.FileMode(int(data["Mode"].(float64))),
						Name:    data["Name"].(string),
					}
					resjs, err := json.MarshalIndent(res, "", " ")
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.Write(resjs)
				}
			}

		case "put":
			sha := sha256.Sum256(msg.Data)
			shasl := sha[:]
			sig := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)

			ctx := context.Background()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				PrintExit("BeginTx error: %v\n", err)
			}
			_, err = tx.ExecContext(ctx, "INSERT or IGNORE INTO chunks (sig, data) VALUES (?, ?)", sig, msg.Data)
			if err != nil {
				tx.Rollback()
				PrintExit("ExecContext error: %v\n", err)
			}
			err = tx.Commit()
			if err != nil {
				PrintExit("Commit error: %v\n", err)
			}

			res := Message{
				Version: 1,
				Type:    "putresp",
				Sig:     sig,
			}
			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)

		case "del":
			// delete
			// fmt.Println("in del, sig: ", msg.Sig)
			ctx := context.Background()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				PrintExit("BeginTx error: %v\n", err)
			}
			if msg.Sig == "all" {
				m = make(map[string]TreeNode)
				_, err = tx.ExecContext(ctx, "DELETE FROM chunks")
			} else {
				_, err = tx.ExecContext(ctx, "DELETE FROM chunks WHERE sig=?", msg.Sig)
			}
			if err != nil {
				tx.Rollback()
				PrintExit("ExecContext error: %v\n", err)
			}
			err = tx.Commit()
			if err != nil {
				PrintExit("Commit error: %v\n", err)
			}

		case "info":
			rows, err := db.Query("SELECT sig, LENGTH(data) FROM chunks")
			if err != nil {
				PrintExit("SELECT DB error: %v\n", err)
			}
			defer rows.Close()

			sigs := 0
			totalSz := 0

			for rows.Next() {
				var sig string
				var sz int
				rows.Scan(&sig, &sz)
				sigs++
				totalSz += sz
			}
			info := fmt.Sprintf("%d chunks, %d total bytes", sigs, totalSz)

			res := Message{
				Info: info,
			}
			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)

		case "sync":
			counter := map[string]int{"n_req": 0, "n_bytes": 0, "n_chunks": 0}
			target := msg.TreeTarget
			fmt.Println("target:", target)

			msg := Message{
				Version:    1,
				Type:       "tree",
				TreeSig:    "",
				Sig:        "",
				TreeHeight: msg.TreeHeight,
			}
			m := make(map[string]TreeNode)
			myRoot := buildTree(msg.TreeHeight, "", m)

			body := postToServer2(&msg, target+"/json", "sync failed")

			counter["n_req"]++
			counter["n_bytes"] += len(body)
			counter["n_bytes"] += len([]byte(fmt.Sprintf("%+v", msg)))

			var data Message
			err = json.Unmarshal([]byte(body), &data)
			errCheck(err)
			othersRoot := data.Node

			findDiffAndGet(myRoot, othersRoot, othersRoot, target+"/json", counter)

			info := fmt.Sprintf("sync took %d requests, %d bytes, pulled %d chunks",
				counter["n_req"], counter["n_bytes"], counter["n_chunks"])

			res := Message{
				Version: 1,
				Type:    "syncresp",
				Info:    info,
			}
			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)

		case "tree":
			// fmt.Println("tree height:", msg.TreeHeight)

			var res Message
			// construct a tree
			if msg.TreeHeight > 0 {
				root := buildTree(msg.TreeHeight, "", m)
				res = Message{
					Version: 1,
					Type:    "treeresp",
					Node:    root,
				}

			} else {
				// request a node
				node := m[msg.Sig]
				res = Message{
					Version: 1,
					Type:    "treeresp",
					Node:    &node,
				}
			}
			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)
		}

	}

}

func postToServer2(msg *Message, addr string, errMsg string) []byte {
	var res *http.Response
	var err error
	URL := "http://" + addr
	fmt.Println("postToServer2:", URL)
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	fmt.Printf("Send msg:\n%+v\n\n", msg)

	req, _ := http.NewRequest("POST", URL, buf)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	// client := &http.Client{Timeout: time.Duration(S_TIMEOUT) * time.Millisecond}
	res, err = client.Do(req)
	// if err != nil {
	// 	switch errtype := err.(type) {
	// 	case net.Error:
	// 		if errtype.Timeout() {
	// 			// retry
	// 			res, err = retry(req, 10)
	// 		}
	// 	}
	// }
	errCheck(err)

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	if res.StatusCode != 200 {
		if errMsg != "" {
			errCheck(errors.New(errMsg))
		}
		return nil
	}

	body, err := ioutil.ReadAll(res.Body)
	// fmt.Println("Response:\n", string(body))
	return body
}

func retry(req *http.Request, times int) (*http.Response, error) {
	var res *http.Response
	for i := 0; i < times; i++ {
		client := &http.Client{Timeout: time.Duration((1+i)*S_TIMEOUT) * time.Millisecond}
		fmt.Println("Retrying with deadline: ", S_TIMEOUT*(i+1))
		res, err = client.Do(req)
		if err == nil {
			break
		}
	}
	return res, err
}

func buildTree(height int, pattern string, m map[string]TreeNode) *TreeNode {
	if height == 1 {
		var chunksigs []string
		var sigsStr string

		patternStr := "sha256_32_" + pattern + "%"
		rows, err := db.Query("SELECT sig FROM chunks WHERE sig LIKE ? ORDER BY sig ASC", patternStr)
		if err != nil {
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
		m["sha256_32_"+nodeSig] = node
		return &node
	}

	var children []*TreeNode
	var ChildSigs []string
	var sigsStr string

	for i := 0; i < NUM_CHILDREN; i++ {
		chld := buildTree(height-1, pattern+string(encodeStd[i]), m)
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
	m["sha256_32_"+nodeSig] = node
	return &node
}

// target should be the full addr. E.g. "localhost:8080/json"
func findDiffAndGet(node1 *TreeNode, node2 *TreeNode, tree2root *TreeNode, target string, counter map[string]int) []string {
	if node1.Sig == node2.Sig {
		return nil
	}
	if len(node1.ChildSigs) == 0 && len(node2.ChildSigs) == 0 {
		re := node2.ChunkSigs
		// if returns from leaf, compare chunks
		for i := 0; i < len(re); i++ {
			var buffer []byte
			row := db.QueryRow("SELECT sig FROM chunks WHERE sig=?", re[i])
			err = row.Scan(&buffer)
			if err != nil && err != sql.ErrNoRows {
				PrintExit("SELECT DB error: %v\n", err)
			}
			if err == sql.ErrNoRows {
				msg := Message{
					Version: 1,
					Type:    "get",
					Sig:     re[i],
				}

				var data map[string]interface{}
				body := postToServer2(&msg, target, "Get Request Failed.")
				counter["n_req"]++
				counter["n_bytes"] += len(body)
				counter["n_bytes"] += len([]byte(fmt.Sprintf("%+v", msg)))
				counter["n_chunks"]++

				err = json.Unmarshal([]byte(body), &data)
				errCheck(err)
				bytebuf, err := base64.StdEncoding.DecodeString(data["Data"].(string))
				errCheck(err)

				// add chunk to db
				ctx := context.Background()
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					PrintExit("BeginTx error: %v\n", err)
				}
				_, err = tx.ExecContext(ctx, "INSERT or IGNORE INTO chunks (sig, data) VALUES (?, ?)", re[i], bytebuf)
				if err != nil {
					tx.Rollback()
					PrintExit("findDiffAndGet ExecContext error: %v\n", err)
				}
				err = tx.Commit()
				if err != nil {
					PrintExit("Commit error: %v\n", err)
				}
			}
		}
		return nil
		// return node2.ChunkSigs
	}
	for i := 0; i < NUM_CHILDREN; i++ {
		if node1.ChildSigs[i] != node2.ChildSigs[i] {
			msg := Message{
				Version: 1,
				Type:    "tree",
				// TreeSig: node2.Sig,
				TreeSig: tree2root.Sig,
				Sig:     node2.ChildSigs[i],
			}
			body := postToServer2(&msg, target, strconv.Itoa(i)+" sync failed: "+node2.ChildSigs[i])
			counter["n_req"]++
			counter["n_bytes"] += len(body)
			counter["n_bytes"] += len([]byte(fmt.Sprintf("%+v", msg)))

			var data Message
			err = json.Unmarshal([]byte(body), &data)
			errCheck(err)
			node := data.Node
			findDiffAndGet(node1.children[i], node, tree2root, target, counter)

		} else {
			// fmt.Println("same node sig")
		}
	}
	return nil
}
