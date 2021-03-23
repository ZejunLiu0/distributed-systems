package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var myTemplate *template.Template

var ServerAddress = "localhost:8080"

var BLOBSTORE = "server_store"

type Message struct {
	Version int
	Type    string // "get|getfile|put""
	Sig     string
	Data    []byte
	Name    string
	ModTime time.Time
	Mode    os.FileMode
}

type Img struct {
	Name string
	Data string
	Sig  string
}

func Serve(arg string) {
	if _, err := os.Stat(BLOBSTORE); os.IsNotExist(err) {
		os.Mkdir(BLOBSTORE, 0777)
	}
	port := strings.Split(arg, ":")[1]
	println(port)
	http.HandleFunc("/", Handler)
	http.ListenAndServe(":"+port, nil)
}

func CombineChunks(path string) ([]byte, map[string]interface{}) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(content, &data)
	if err != nil {
		panic(err)
	}

	var byteBuffer []byte
	if data["Data"] != nil {
		blobs := data["Data"].([]interface{})

		for i := 0; i < len(blobs); i++ {
			blobName := blobs[i].(string)
			datachunk, err := ioutil.ReadFile(filepath.Join(BLOBSTORE, blobName))
			if err != nil {
				panic(err)
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
		filename := filepath.Join(BLOBSTORE, sig)
		_, err := os.Stat(filename)

		if os.IsNotExist(err) {
			fmt.Fprintf(w, "Did not find: %s", sig)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 - Invalid Signature."))
		} else {
			content, err := ioutil.ReadFile(filename)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if string(content)[0] == '{' {
				var data map[string]interface{}
				err = json.Unmarshal(content, &data)
				if err != nil {
					panic(err)
				}
				// Is a directory, render html
				if data["Type"].(string) == "dir" {
					myTemplate, err := template.ParseFiles("index.html")
					if err != nil {
						panic(err)
					}
					// p := Person{Name: "Mary", age: "31"}
					length := len(data["FileNames"].([]interface{}))
					var dataSlice []Img
					for i := 0; i < length; i += 4 {
						for j := 0; j < 4; j++ {
							if i+j >= length {
								break
							}
							sig := data["FileSigs"].([]interface{})[i+j].(string)
							p := filepath.Join(BLOBSTORE, sig)
							name := data["FileNames"].([]interface{})[i+j].(string)
							println(p)
							byteBuffer, _ := CombineChunks(p)
							b64Str := base64.StdEncoding.EncodeToString(byteBuffer)
							println("name:", data["FileNames"].([]interface{})[i+j].(string))
							im := Img{Name: name, Data: b64Str, Sig: sig}
							dataSlice = append(dataSlice, im)
						}
						myTemplate.Execute(w, dataSlice)
						dataSlice = nil
					}

				} else if data["Type"].(string) == "file" {
					byteBuffer, data := CombineChunks(filename)
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
		err := json.NewDecoder(r.Body).Decode(&msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch msg.Type {
		case "get":
			fmt.Println("get:", msg.Sig)
			filename := filepath.Join(BLOBSTORE, msg.Sig)
			info, err := os.Stat(filename)
			if os.IsNotExist(err) {
				fmt.Println("Did not find:", msg.Sig)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Signature."))
			} else {
				f, _ := os.Open(filename)
				var size int64 = info.Size()
				buffer := make([]byte, size)
				f.Seek(0, 0)
				f.Read(buffer)
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
			fmt.Println("getfile:", msg.Sig)
			filename := filepath.Join(BLOBSTORE, msg.Sig)
			_, err := os.Stat(filename)

			if os.IsNotExist(err) {
				fmt.Println("Did not find:", msg.Sig)
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte("404 - Invalid Signature."))
			} else {
				content, err := ioutil.ReadFile(filename)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				var data map[string]interface{}
				err = json.Unmarshal(content, &data)

				// if string(content)[0] != '{' {
				if data["Type"].(string) != "file" {
					fmt.Println("Sig is not a file recipe:", msg.Sig)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("500 - Not a Recipe."))
				} else {
					byteBuffer, data := CombineChunks(filename)
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

			err = ioutil.WriteFile(filepath.Join(BLOBSTORE, sig), msg.Data, 0666)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
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
		}
	}

}
