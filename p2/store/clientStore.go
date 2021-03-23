package store

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

//=====================================================================

func errCheck(err error) {
	if err != nil {
		panic(err)
	}
}

func CmdPut(arg string) string {
	// URL := "http://" + ServerAddress
	// println("URL:", URL)
	f, err := os.Open(arg)
	errCheck(err)
	fi, err := os.Stat(arg)
	errCheck(err)

	switch mode := fi.Mode(); {
	case mode.IsDir():
		// directory
		files, err := filepath.Glob(filepath.Join(arg, "*"))
		errCheck(err)
		var names, sigs []string

		for _, f := range files {
			sigs = append(sigs, CmdPut(f))
			names = append(names, filepath.Base(f))
		}

		// directories
		dirObj := &ObjectDir{
			Version:   1,
			Type:      "dir",
			Name:      filepath.Base(arg),
			PrevSig:   "",
			FileNames: names,
			FileSigs:  sigs,
		}
		dirJs, err := json.MarshalIndent(dirObj, "", "    ")
		errCheck(err)

		// post to server
		msg := Message{
			Version: 1,
			Type:    "put",
			Data:    dirJs,
		}
		fileName := postToServer(&msg)
		if fileName == "" {
			errCheck(errors.New("Failed to get the signature."))
		}
		fmt.Println("Dir Signature:", fileName)

		return fileName

	case mode.IsRegular():
		// file blobs
		var size int64 = fi.Size()
		var offset int64 = 0
		var chunks []string
		var chunkSize, chunkLen uint64
		var chunkNum int
		buffer := make([]byte, size)

		_, err := f.Seek(offset, 0)
		errCheck(err)
		_, err = f.Read(buffer)
		errCheck(err)

		for offset < size {
			chunkSize = rkchunk(buffer[offset:], uint64(size-offset))
			chunkNum++
			chunkLen += chunkSize

			// println("Chunk at offset", offset, ", len", chunkSize)

			chunkBuf := make([]byte, chunkSize)
			f.Seek(offset, 0)
			f.Read(chunkBuf)
			offset += int64(chunkSize)

			// post to server
			msg := Message{
				Version: 1,
				Type:    "put",
				Data:    chunkBuf,
			}
			fileName := postToServer(&msg)
			if fileName == "" {
				errCheck(errors.New("Failed to get the signature."))
			}
			fmt.Println("Chunk Signature:", fileName)

			chunks = append(chunks, fileName)
		}

		// file json
		fileObj := &ObjectFile{
			Version: 1,
			Type:    "file",
			Name:    filepath.Base(arg),
			ModTime: fi.ModTime(),
			Mode:    420,
			Data:    chunks,
		}
		fileJs, err := json.MarshalIndent(fileObj, "", " ")
		errCheck(err)

		msg := Message{
			Version: 1,
			Type:    "put",
			Data:    fileJs,
		}
		fileName := postToServer(&msg)
		if fileName == "" {
			errCheck(errors.New("Failed to get the signature."))
		}
		fmt.Println("File Recipe Signature:", fileName)
		return fileName
	}
	return ""
}

func postToServer(msg *Message) string {
	URL := "http://" + ServerAddress
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	req, _ := http.NewRequest("POST", URL, buf)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	errCheck(err)

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	if res.StatusCode != 200 {
		return ""
	}

	var data map[string]interface{}
	body, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal([]byte(body), &data)
	errCheck(err)

	// Print the body to the stdout
	io.Copy(os.Stdout, res.Body)
	return data["Sig"].(string)
}

func rkchunk(buf []byte, len uint64) uint64 {
	const HASHLEN = 32
	const THE_PRIME = 31
	const MINCHUNK = 2048
	const TARGETCHUNK = 4096
	const MAXCHUNK = 8192
	var hash, off, b, b_n uint64
	var saved [256]uint64

	if b == 0 {
		b = THE_PRIME
		b_n = 1
		for i := 0; i < (HASHLEN - 1); i++ {
			b_n *= b
		}

		for i := 0; i < 256; i++ {
			saved[i] = uint64(i) * b_n
		}
	}

	for (off < HASHLEN) && (uint64(off) < len) {
		hash = hash*b + uint64(buf[off])
		off++
	}

	for off < len {
		hash = (hash-saved[buf[off-HASHLEN]])*b + uint64(buf[off])
		off++

		if (off >= MINCHUNK && hash%TARGETCHUNK == 1) || off >= MAXCHUNK {
			return off
		}
	}
	return off
}

func CmdGet(sig string, nPath string) error {
	URL := "http://" + ServerAddress
	// send request to server
	msg := Message{
		Version: 1,
		Type:    "get",
		Sig:     sig,
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	req, _ := http.NewRequest("POST", URL, buf)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	errCheck(err)

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	if res.StatusCode != 200 {
		return errors.New("Get Request Failed.")
	}

	// response from server
	var data map[string]interface{}
	body, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal([]byte(body), &data)
	errCheck(err)
	bytebuf, err := base64.StdEncoding.DecodeString(data["Data"].(string))
	errCheck(err)
	// if is not a recipe,
	if string(bytebuf)[0] != '{' {
		err = ioutil.WriteFile(nPath, bytebuf, 0666)
		errCheck(err)
	} else {
		var data map[string]interface{}
		err = json.Unmarshal(bytebuf, &data)
		errCheck(err)

		if data["Type"].(string) == "file" {
			CmdGetFile(sig, nPath)
		} else if data["Type"].(string) == "dir" {
			if _, err := os.Stat(nPath); os.IsNotExist(err) {
				os.Mkdir(nPath, 0777)
			}

			sigs := data["FileSigs"].([]interface{})
			names := data["FileNames"].([]interface{})

			for i := 0; i < len(sigs); i++ {
				CmdGet(sigs[i].(string), filepath.Join(nPath, names[i].(string)))
			}
		} else {
			fmt.Println("Unsupported Type.")
		}
	}
	return nil
}

func CmdGetFile(sig string, nPath string) error {
	URL := "http://" + ServerAddress
	msg := Message{
		Version: 1,
		Type:    "getfile",
		Sig:     sig,
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	req, _ := http.NewRequest("POST", URL, buf)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	errCheck(err)

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	if res.StatusCode != 200 {
		return errors.New("GetFile Request Failed.")
	}

	// response from server
	var data map[string]interface{}
	body, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal([]byte(body), &data)
	errCheck(err)
	if data["Data"] != nil {
		bytebuf, err := base64.StdEncoding.DecodeString(data["Data"].(string))
		errCheck(err)
		err = ioutil.WriteFile(nPath, bytebuf, 0666)
		errCheck(err)
	} else {
		emptyFile, err := os.Create(nPath)
		errCheck(err)
		emptyFile.Close()
	}

	return nil
}

func CmdGetFileNoJSON(sig string, nPath string) error {
	return nil
}

func CmdDesc(sig string) {
	URL := "http://" + ServerAddress
	// println("URL:", URL)

	msg := Message{
		Version: 1,
		Type:    "get",
		Sig:     sig,
	}
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	req, _ := http.NewRequest("POST", URL, buf)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	errCheck(err)

	defer res.Body.Close()
	fmt.Println("response Status:", res.Status)
	if res.StatusCode != 200 {
		return
	}

	// response from server
	var data map[string]interface{}
	body, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal([]byte(body), &data)
	errCheck(err)
	bytebuf, err := base64.StdEncoding.DecodeString(data["Data"].(string))
	errCheck(err)
	// if is not a recipe,
	if string(bytebuf)[0] != '{' {
		println(strconv.Itoa(len(bytebuf)) + "-byte blob")
	} else {
		s := string(bytebuf)
		fmt.Println(s)
	}
}
