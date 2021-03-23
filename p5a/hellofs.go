// Hellofs implements a simple "hello world" file system.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	. "github.com/mattn/go-getopt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"golang.org/x/net/context"
)

var debug = true
var mountPoint = "818fs"

//=============================================================================

func PrintDebug(s string, args ...interface{}) {
	if !debug {
		return
	}
	PrintAlways(s, args...)
}

func PrintAlways(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

//=============================================================================

func main() {
	var flag int

	for {
		if flag = Getopt("dm:"); flag == EOF {
			break
		}

		switch flag {
		case 'd':
			debug = !debug
		case 'm':
			mountPoint = OptArg
		default:
			println("usage: main.go [-d | -m <mountpt>]", flag)
			os.Exit(1)
		}
	}

	PrintAlways("mount %q, debug %v\n", mountPoint, debug)

	if _, err := os.Stat(mountPoint); err != nil {
		os.Mkdir(mountPoint, 0755)
	}
	fuse.Unmount(mountPoint)
	conn, err := fuse.Mount(mountPoint, fuse.FSName("dssFS"), fuse.Subtype("project P1"),
		fuse.LocalVolume(), fuse.VolumeName("dssFS"))
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() {
		<-ch
		defer conn.Close()
		fuse.Unmount(mountPoint)
		os.Exit(1)
	}()

	err = fs.Serve(conn, FS{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-conn.Ready
	if err := conn.MountError; err != nil {
		log.Fatal(err)
	}
}

// FS implements the hello world file system.
type FS struct{}

func (FS) Root() (fs.Node, error) {
	fmt.Println("in root")
	return Dir{}, nil
}

// Dir implements both Node and Handle for the root directory.
type Dir struct{}

func (Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	fmt.Println("in attr")

	return nil
}

func (Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Println("in lookup")
	if name == "hello" {
		return File{}, nil
	}
	return nil, fuse.ENOENT
}

var dirDirs = []fuse.Dirent{
	{Inode: 2, Name: "hello", Type: fuse.DT_File},
}

func (Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return dirDirs, nil
}

// File implements both Node and Handle for the hello file.
type File struct{}

const greeting = "hello, world\n"

func (File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 2
	a.Mode = 0444
	a.Size = uint64(len(greeting))
	return nil
}

func (File) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte(greeting), nil
}
