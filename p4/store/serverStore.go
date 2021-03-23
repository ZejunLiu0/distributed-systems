package store

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
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
	"regexp"
	"strconv"
	"strings"
	"time"
)

var myTemplate *template.Template

var ServerAddress = "localhost:8080/json"
var publicKeyFile = ""

var BLOBSTORE = "server_store"

var m map[string]TreeNode
var S_TIMEOUT = 50

type Img struct {
	Name string
	Data string
	Sig  string
}

func Serve(arg string, pubKF string, sign bool) {
	publicKeyFile = pubKF
	NeedSig = sign
	// fmt.Println("pub key:", publicKeyFile, "Needsig:", NeedSig)

	if _, err := os.Stat(BLOBSTORE); os.IsNotExist(err) {
		os.Mkdir(BLOBSTORE, 0777)
	}
	// tree sig -> node map
	m = make(map[string]TreeNode)
	port := strings.Split(arg, ":")[1]
	println(port)

	http.HandleFunc("/json", Handler)
	http.HandleFunc("/", nonJsonHandler)

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

func nonJsonHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Non json requests
		sig := r.URL.Path[1:]
		var content []byte

		// for anchor
		isAnchor, _ := regexp.MatchString("root/", sig)
		if isAnchor {
			rootname := sig[5:]
			claimSig := getLastClaimSig(rootname)
			rootsig := getRecipeSigFromClaimSig(claimSig)

			if rootsig == "" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Something went wrong"))
			} else {
				byteBuffer, _ := CombineChunks(rootsig)
				if byteBuffer == nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				contentType := http.DetectContentType(byteBuffer)
				w.Header().Set("Content-Type", contentType)
				w.Write(byteBuffer)
			}

		} else {
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
						w.Header().Set("Content-Type", contentType)
						http.ServeContent(w, r, name, modtime, bytes.NewReader(byteBuffer))
					}

				} else {
					fmt.Fprintf(w, "Not a recipe: %s", sig)
					w.WriteHeader(http.StatusNotFound)
					w.Write([]byte("404 - Signature is not a recipe."))
				}
			}
		}
	}
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
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
			// fmt.Println("put msg:", msg)
			// var data map[string]interface{}
			var chunkType, signature, content string

			dataStr := string(msg.Data)

			hasType, _ := regexp.MatchString("\"Type\"", dataStr)
			if hasType {
				strs := strings.Split(dataStr, "\"Type\": \"")[1]
				chunkType = strings.Split(strs, "\"")[0]
			} else {
				chunkType = "data"
			}

			signed, _ := regexp.MatchString("\"signature\"", dataStr)

			// with -x, didn't sign
			if (chunkType == "anchor" || chunkType == "claim") && NeedSig && !signed {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Verification failed"))
				return
			}

			if signed && publicKeyFile != "" {
				// Get type (anchor/claim), signature, then verify with public key
				// strs := strings.Split(dataStr, "\"Type\": \"")[1]
				// chunkType = strings.Split(strs, "\"")[0]

				// if publicKeyFile != "" {
				signature = strings.Split(dataStr, "\"signature\": \"")[1]
				signature = signature[:len(signature)-2]
				decodedSig, err := base32.StdEncoding.DecodeString(signature)
				errCheck(err)
				// fmt.Println("type:", chunkType)
				// fmt.Println("sig:", decodedSig)
				content = strings.Split(string(dataStr), "\"signature\": \"")[0]
				content = content[:len(content)-2]

				pubKey := readPublicKey(publicKeyFile)
				sha := sha256.Sum256([]byte(content))
				shasl := sha[:]

				err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, shasl, decodedSig)
				if err == nil {
					fmt.Println("Verification succeeded")
				} else {
					// os.Stderr.WriteString("ERROR: verification failed: crypto/rsa: verification error\n")
					// http.Error(w, err.Error(), http.StatusBadRequest)
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Verification failed"))
					return
				}
				// }

			}

			// fmt.Println(chunkType)

			sha := sha256.Sum256(msg.Data)
			shasl := sha[:]
			sig := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)

			// start txn
			ctx := context.Background()
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				PrintExit("BeginTx error: %v\n", err)
			}

			if chunkType == "anchor" {
				strs := strings.Split(dataStr, "\"Name\": \"")[1]
				name := strings.Split(strs, "\"")[0]

				// fmt.Println("anchor name:", name, " sig:", sig)
				_, err = tx.ExecContext(ctx, "INSERT or REPLACE INTO anchors (name, sig, data) VALUES (?, ?, ?)", name, sig, msg.Data)
				if err != nil {
					tx.Rollback()
					PrintExit("ExecContext error: %v\n", err)
				}
			} else if chunkType == "claim" {
				// validate signature with server's public key
				// fmt.Println("in claim:", msg.Data)

				// lastID: For sorting in "chain"
				var lastID int

				strs := strings.Split(dataStr, "\"Prevsig\": \"")[1]
				prevsig := strings.Split(strs, "\"")[0]
				strs = strings.Split(dataStr, "\"Refsig\": \"")[1]
				refsig := strings.Split(strs, "\"")[0]

				// If is not the first claim
				if prevsig != "" {
					var buf []byte

					row := db.QueryRow("SELECT id FROM claims WHERE sig=?", prevsig)
					err = row.Scan(&buf)

					if err != nil && err != sql.ErrNoRows {
						PrintExit("Table claims: SELECT DB error: %v\n", err)
					}
					if err == sql.ErrNoRows {
						fmt.Println("Table claims: Did not find:", prevsig)
						w.WriteHeader(http.StatusNotFound)
						w.Write([]byte("404 - Invalid Signature."))
					} else {
						bufStr := string(buf)
						// fmt.Println("buf str:", bufStr)
						lastID, _ = strconv.Atoi(bufStr)
						lastID++
					}
					// Update "isLast" of the last claim to false
					_, err = tx.ExecContext(ctx, "UPDATE claims SET isLast=false WHERE sig=?", prevsig)
					if err != nil {
						tx.Rollback()
						PrintExit("Update \"isLast\" failed: %v\n", err)
					}
				} else {
					// Not the first claim
					lastID = 0
				}

				// fmt.Println("Refsig:", refsig, "Prevsig:", prevsig)

				_, err = tx.ExecContext(ctx, "INSERT or IGNORE INTO claims (refsig, prevsig, sig, isLast, id, data) VALUES (?, ?, ?, ?, ?, ?)", refsig, prevsig, sig, true, lastID, msg.Data)
				if err != nil {
					tx.Rollback()
					PrintExit("ExecContext error: %v\n", err)
				}
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
				_, err = tx.ExecContext(ctx, "DELETE FROM anchors")
				_, err = tx.ExecContext(ctx, "DELETE FROM claims")

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
			// fmt.Println("target:", target)

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

		case "rootanchor":
			// root (Name) -> (Sig)
			// Send request for content hash associated with root Name.

			row := db.QueryRow("SELECT sig FROM anchors WHERE name=?", msg.Name)
			err = row.Scan(&buffer)
			if err != nil && err != sql.ErrNoRows {
				PrintExit("root: SELECT DB error: %v\n", err)
			}
			if err == sql.ErrNoRows {
				fmt.Println("root: Did not find:", msg.Name)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Root Name."))
			} else {
				res := Message{
					Version: 1,
					Type:    "rootresp",
					Sig:     string(buffer),
				}
				resjs, err := json.MarshalIndent(res, "", " ")
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write(resjs)
			}

		case "lastclaim":
			// lastclaim (Name) -> (Sig)
			// Send request for hash of last claim associated with root Name.
			var res Message
			claimSig := getLastClaimSig(msg.Name)
			if claimSig == "-1" {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Root Name."))
			}

			res = Message{
				Version: 1,
				Type:    "lastclaimresp",
				Sig:     claimSig,
			}

			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)

		case "chain":
			/*
				chain (Name) -> (Chain)
				Send request for ordered (earlier-to-later) list of anchor/claim
				hashes for root Name.
			*/

			var rootSig []byte
			var sigList []string
			row := db.QueryRow("SELECT sig FROM anchors WHERE name=?", msg.Name)
			err = row.Scan(&rootSig)
			if err != nil && err != sql.ErrNoRows {
				PrintExit("chain: SELECT anchors error: %v\n", err)
			}
			if err == sql.ErrNoRows {
				fmt.Println("Did not find:", msg.Name)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Root Name."))
			}
			// fmt.Println("root: ", string(rootSig))
			sigList = append(sigList, string(rootSig))

			rows, err := db.Query("SELECT sig FROM claims WHERE refsig=? ORDER BY id ASC", string(rootSig))
			if err != nil {
				PrintExit("chain: SELECT DB error: %v\n", err)
			}
			defer rows.Close()

			for rows.Next() {
				var sig string
				rows.Scan(&sig)
				sigList = append(sigList, sig)
				// fmt.Println(sig, patternStr)
			}
			// fmt.Println(sigList)

			res := Message{
				Version: 1,
				Type:    "chainresp",
				Chain:   sigList,
			}
			resjs, err := json.MarshalIndent(res, "", " ")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(resjs)

		default:
			fmt.Println("Unsupported message type.")
		}
	}
}

func getLastClaimSig(name string) string {
	// name := msg.Name
	var rootSig, claimSig []byte

	fmt.Println("msg.Name:", name)
	row := db.QueryRow("SELECT sig FROM anchors WHERE name=?", name)
	err = row.Scan(&rootSig)
	fmt.Println("rootSig:", string(rootSig))

	if err != nil && err != sql.ErrNoRows {
		PrintExit("SELECT anchors error: %v\n", err)
	}
	if err == sql.ErrNoRows {
		fmt.Println("lastclaim: Did not find:", name)
		return "-1"
	}

	row = db.QueryRow("SELECT sig FROM claims WHERE refsig=? AND isLast=true", string(rootSig))
	// row = db.QueryRow("SELECT sig FROM claims WHERE refsig=?", string(rootSig))

	err = row.Scan(&claimSig)
	// fmt.Println("claimSig:", string(claimSig))
	if err != nil && err != sql.ErrNoRows {
		PrintExit("SELECT claims error: %v\n", err)
	}
	if err == sql.ErrNoRows {
		return ""
	}

	return string(claimSig)
}

func getRecipeSigFromClaimSig(sig string) string {
	var claimContent []byte

	row := db.QueryRow("SELECT data FROM chunks WHERE sig=?", sig)
	err = row.Scan(&claimContent)

	if err != nil && err != sql.ErrNoRows {
		PrintExit("SELECT DB error: %v\n", err)
	}
	if err == sql.ErrNoRows {
		fmt.Println("Did not find:", sig)
		return ""
		// w.WriteHeader(http.StatusNotFound)
		// w.Write([]byte("404 - Invalid Signature."))
	} else {
		// fmt.Println(string(claimContent))
		claimStr := string(claimContent)
		strs := strings.Split(claimStr, "\"rootsig\": \"")[1]
		rootsig := strings.Split(strs, "\"")[0]

		return rootsig
	}
	return ""
}

func postToServer2(msg *Message, addr string, errMsg string) []byte {
	var res *http.Response
	var err error
	URL := "http://" + addr
	fmt.Println("postToServer2:", URL)
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	// fmt.Printf("Send msg:\n%+v\n\n", msg)

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
