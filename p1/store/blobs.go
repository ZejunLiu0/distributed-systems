package store

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ObjectFile struct {
	Version int
	Type    string
	Name    string
	ModTime time.Time
	Mode    os.FileMode
	PrevSig string
	Data    []string
}

type ObjectDir struct {
	Version   int
	Type      string
	Name      string
	PrevSig   string
	FileNames []string
	FileSigs  []string
}

var CHUNK_DIR = os.Getenv("HOME") + "/.blobstore/"

const CHUNKSIZE = 8192

func errCheck(err error) {
	if err != nil {
		panic(err)
	}
}

//=====================================================================
func CmdPut(arg string) string {
	// func CmdPut(arg string) {
	// PrintAlways("cmd PUT %s\n", arg)

	f, err := os.Open(arg)
	errCheck(err)
	fi, err := os.Stat(arg)
	errCheck(err)

	switch mode := fi.Mode(); {
	case mode.IsDir():
		// directory
		files, err := filepath.Glob(filepath.Join(arg, "*"))
		errCheck(err)
		// fmt.Printf("files: %d\n", len(files))
		var names, sigs []string

		for _, f := range files {
			// fmt.Println(f)
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
		sha := sha256.Sum256(dirJs)
		shasl := sha[:]
		fileName := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)
		err = ioutil.WriteFile(".blobstore/"+fileName, dirJs, 0644)
		errCheck(err)
		// fmt.Println(fileName)
		os.Stderr.WriteString(fileName + "\n")

		return fileName

	case mode.IsRegular():
		// file blobs
		var size int64 = fi.Size()
		var offset int64 = 0
		var chunks []string
		buffer := make([]byte, CHUNKSIZE)

		for offset < size {
			_, err := f.Seek(offset, 0)
			errCheck(err)

			if size-offset < CHUNKSIZE {
				buffer = make([]byte, size-offset)
			}
			_, err = f.Read(buffer)

			// Create a sig for the blob
			sha := sha256.Sum256(buffer)
			shasl := sha[:]
			fileName := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)
			err = ioutil.WriteFile(".blobstore/"+fileName, buffer, 0644)
			errCheck(err)

			chunks = append(chunks, fileName)
			offset += CHUNKSIZE
		}

		// file json
		fileObj := &ObjectFile{
			Version: 1,
			Type:    "file",
			Name:    filepath.Base(arg),
			ModTime: fi.ModTime(),
			Mode:    420,
			PrevSig: "",
			Data:    chunks,
		}
		fileJs, err := json.MarshalIndent(fileObj, "", " ")
		errCheck(err)

		sha := sha256.Sum256(fileJs)
		shasl := sha[:]
		fileName := "sha256_32_" + base32.StdEncoding.EncodeToString(shasl)
		err = ioutil.WriteFile(".blobstore/"+fileName, fileJs, 0644)
		errCheck(err)

		// fmt.Println(fileName)
		os.Stderr.WriteString(fileName + "\n")
		return fileName

	}
	return ""
}

//==================================================================
func CmdGet(sig string, newPath string) error {

	content, err := ioutil.ReadFile(".blobstore/" + sig)
	errCheck(err)

	var data map[string]interface{}

	err = json.Unmarshal(content, &data)
	errCheck(err)

	// fmt.Println(data["Type"])
	switch data["Type"] {
	case "file":
		blobs := data["Data"].([]interface{})

		for i := 0; i < len(blobs); i++ {
			blobName := blobs[i].(string)
			content, err := ioutil.ReadFile(".blobstore/" + blobName)
			errCheck(err)
			newFile, err := os.OpenFile(newPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			errCheck(err)
			_, err = newFile.Write(content)
			errCheck(err)
		}

	case "dir":
		fileSigs := data["FileSigs"].([]interface{})
		fileNames := data["FileNames"].([]interface{})
		// fmt.Println(newPath)
		os.Stderr.WriteString(newPath + "\n")
		err := os.Mkdir(newPath, 0755)
		errCheck(err)
		for i := 0; i < len(fileSigs); i++ {
			CmdGet(fileSigs[i].(string), filepath.Join(newPath, fileNames[i].(string)))
		}
	}

	return nil
}

//=====================================================================

func CmdDesc(sig string) {
	// PrintAlways("cmd DESC sig %q\n", sig, newPath)
	filepath := ".blobstore/" + sig
	content, err := ioutil.ReadFile(filepath)
	errCheck(err)
	if string(content)[0] == '{' {
		// println(string(content))
		os.Stderr.WriteString(string(content) + "\n")
	} else {
		fi, err := os.Stat(filepath)
		errCheck(err)
		// println(strconv.FormatInt(fi.Size(), 10) + "-byte blob")
		os.Stderr.WriteString(strconv.FormatInt(fi.Size(), 10) + "-byte blob\n")
	}
}
