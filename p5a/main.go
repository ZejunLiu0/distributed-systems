// memfs implements a simple in-memory file system.
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	. "github.com/mattn/go-getopt"
	"golang.org/x/net/context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

/*
    Need to implement these types from bazil/fuse/fs!

    type FS interface {
	  // Root is called to obtain the Node for the file system root.
	  Root() (Node, error)
    }

    type Node interface {
	  // Attr fills attr with the standard metadata for the node.
	  Attr(ctx context.Context, attr *fuse.Attr) error
    }
*/

//=============================================================================

type Dfs struct{}

type DNode struct {
	Name     string
	Attrs    fuse.Attr
	dirty    bool
	children map[string]*DNode
	data     []uint8

	// Inode uint64
	// Type fuse.DirentType
	// Mode  os.FileMode // file mode
}

var (
	root       *DNode
	Debug      = false
	printCalls = false
	conn       *fuse.Conn
	// mountPoint = "818fs"      // default mount point
	mountPoint = "dss"        // default mount point
	uid        = os.Geteuid() // use the same uid/gid for all
	gid        = os.Getegid()
	nextInode  = 0

	_ fs.Node               = (*DNode)(nil) // make sure that DNode implements bazil's Node interface
	_ fs.FS                 = (*Dfs)(nil)   // make sure that DNode implements bazil's FS interface
	_ fs.Handle             = (*DNode)(nil)
	_ fs.HandleReadAller    = (*DNode)(nil)
	_ fs.HandleReadDirAller = (*DNode)(nil)
	_ fs.HandleWriter       = (*DNode)(nil)
	_ fs.HandleFlusher      = (*DNode)(nil)
	_ fs.NodeMkdirer        = (*DNode)(nil)
	_ fs.NodeStringLookuper = (*DNode)(nil)
	_ fs.NodeGetattrer      = (*DNode)(nil)
	_ fs.NodeFsyncer        = (*DNode)(nil)
	_ fs.NodeSetattrer      = (*DNode)(nil)
	_ fs.NodeCreater        = (*DNode)(nil)
	_ fs.NodeRemover        = (*DNode)(nil)
	_ fs.NodeRenamer        = (*DNode)(nil)
)

//=============================================================================

// Implement:
//   func (Dfs) Root() (n fs.Node, err error)
//   func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error
//   func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error)
//   func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error)
//   func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error
//   func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error
//   func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error
//   func (p *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error)
//   func (p *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error)
//   func (n *DNode) ReadAll(ctx context.Context) ([]byte, error)
//   func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error
//   func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error
//   func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error
//   func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error

//=====================================================================

func main() {
	var c int

	for {
		if c = Getopt("dm:"); c == EOF {
			break
		}

		switch c {
		case 'd':
			Debug = !Debug
		case 'm':
			mountPoint = OptArg
		default:
			println("usage: main.go [-d | -m <mountpt>]", c)
			os.Exit(1)
		}
	}

	PrintDebug("main\n")

	root = new(DNode)
	root.init("", os.ModeDir|0755)

	// nodeMap[uint64(root.Attrs.Inode)] = root
	PrintDebug("root inode %d\n", int(root.Attrs.Inode))

	if _, err := os.Stat(mountPoint); err != nil {
		os.Mkdir(mountPoint, 0755)
	}
	fuse.Unmount(mountPoint)
	conn, err := fuse.Mount(mountPoint, fuse.FSName("dssFS"), fuse.Subtype("project P5a"),
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

	PrintAlways("mt on %q, debug %v\n", mountPoint, Debug)
	err = fs.Serve(conn, Dfs{})
	PrintDebug("AFTER\n")
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-conn.Ready
	if err := conn.MountError; err != nil {
		log.Fatal(err)
	}
}

func (root *DNode) init(s string, mode os.FileMode) error {
	root.Attrs.Inode = uint64(nextInode)
	nextInode++
	root.Attrs.Mode = mode
	now_time := time.Now()
	root.Attrs.Atime = now_time
	root.Attrs.Mtime = now_time
	root.Attrs.Ctime = now_time
	root.Attrs.Nlink = 1
	root.children = make(map[string]*DNode)
	// root.Type = fuse.DT_Dir
	// root.Mode = mode
	return nil
}

func (Dfs) Root() (n fs.Node, err error) {
	PrintDebug("Root\n")

	return root, nil
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	// fmt.Println("attr")
	attr.Inode = n.Attrs.Inode
	// attr.Size = n.Attrs.Size
	attr.Size = uint64(len(n.data))
	if attr.Size != 0 {
		attr.Blocks = uint64(len(n.data)/512 + 1)
	}
	attr.Atime = n.Attrs.Atime
	attr.Ctime = n.Attrs.Ctime
	attr.Mtime = n.Attrs.Mtime
	attr.Mode = n.Attrs.Mode
	attr.Nlink = n.Attrs.Nlink
	// attr.Uid = n.Attrs.Uid
	// attr.Gid = n.Attrs.Gid

	return nil
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// fmt.Println("Lookup")
	n.Attrs.Atime = time.Now()
	if _, ok := n.children[name]; ok {
		return n.children[name], nil
	}
	return nil, fuse.ENOENT
}

func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// fmt.Println("ReadDirAll\n")
	n.Attrs.Atime = time.Now()
	dirDirs := []fuse.Dirent{}
	var nodeType fuse.DirentType
	for name, node := range n.children {

		if n.Attrs.Mode.IsDir() {
			nodeType = fuse.DT_Dir
		} else {
			nodeType = fuse.DT_File
		}
		nextDir := fuse.Dirent{Inode: node.Attrs.Inode, Name: name, Type: nodeType}
		dirDirs = append(dirDirs, nextDir)
	}
	return dirDirs, nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	// fmt.Println("in Getattr:", len(n.data), "n.Attrs.Size:", n.Attrs.Size)

	resp.Attr.Inode = n.Attrs.Inode
	// resp.Attr.Size = n.Attrs.Size
	// resp.Attr.Blocks = uint64(n.Attrs.Size/512 + 1)
	resp.Attr.Size = uint64(len(n.data))
	if resp.Attr.Size != 0 {
		resp.Attr.Blocks = uint64(len(n.data)/512 + 1)
	}
	resp.Attr.Atime = n.Attrs.Atime
	resp.Attr.Ctime = n.Attrs.Ctime
	resp.Attr.Mtime = n.Attrs.Mtime
	resp.Attr.Mode = n.Attrs.Mode
	resp.Attr.Nlink = n.Attrs.Nlink
	// resp.Attr.Uid = n.Attrs.Uid
	// resp.Attr.Gid = n.Attrs.Gid
	return nil
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// fmt.Println("fsync")
	return nil
}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	// fmt.Println("in Setattr")
	// resp.Attr.Inode = n.Attrs.Inode
	if req.Valid.Size() {
		n.Attrs.Size = req.Size
	}
	// if req.Size != 0 {
	// 	n.Attrs.Blocks = uint64(req.Size/512 + 1)
	// }
	if req.Valid.Atime() {
		n.Attrs.Atime = req.Atime
	}
	if req.Valid.Mtime() {
		n.Attrs.Mtime = req.Mtime
	}
	// if req.Valid.Uid() {
	// 	n.Attrs.Uid = req.Uid
	// }
	// if req.Valid.Gid() {
	// 	n.Attrs.Gid = req.Gid
	// }

	if req.Valid.Mode() {
		n.Attrs.Mode = req.Mode
	}

	resp.Attr.Size = n.Attrs.Size
	// resp.Attr.Blocks = n.Attrs.Blocks
	resp.Attr.Atime = n.Attrs.Atime
	resp.Attr.Mtime = n.Attrs.Mtime
	resp.Attr.Mode = n.Attrs.Mode
	// resp.Attr.Uid = n.Attrs.Uid
	// resp.Attr.Gid = n.Attrs.Gid

	return nil
}

func (p *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	// fmt.Println("Mkdir:", "name:", req.Name)
	PrintDebug("name:", req.Name)
	PrintDebug("mode:", req.Mode)
	PrintDebug("nextInode:", nextInode)

	// newNode := new(DNode)
	var newNode DNode
	newNode.Name = req.Name
	// newNode.dirty = false
	// newNode.Type = fuse.DT_Dir
	newNode.children = make(map[string]*DNode)

	newNode.Attrs.Inode = uint64(nextInode)
	nextInode++
	now_time := time.Now()
	newNode.Attrs.Atime = now_time
	newNode.Attrs.Mtime = now_time
	newNode.Attrs.Ctime = now_time
	newNode.Attrs.Mode = req.Mode
	newNode.Attrs.Nlink = 1
	// newNode.Attrs.Uid = uint32(uid)
	// newNode.Attrs.Gid = uint32(gid)

	p.children[req.Name] = &newNode
	return &newNode, nil
}

func (p *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	// fmt.Println("Create inode:", nextInode, "req.Mode:", req.Mode)

	// newNode := new(DNode)
	var newNode DNode
	newNode.Name = req.Name

	newNode.Attrs.Inode = uint64(nextInode)
	nextInode++
	newNode.Attrs.Size = 0
	now_time := time.Now()
	newNode.Attrs.Atime = now_time
	newNode.Attrs.Mtime = now_time
	newNode.Attrs.Ctime = now_time
	newNode.Attrs.Nlink = 1
	newNode.Attrs.Mode = req.Mode
	// newNode.Attrs.Uid = uint32(uid)
	// newNode.Attrs.Gid = uint32(gid)

	p.children[req.Name] = &newNode
	return &newNode, &newNode, nil
}

func (n *DNode) ReadAll(ctx context.Context) ([]byte, error) {
	// fmt.Println("ReadAll\n")
	n.Attrs.Atime = time.Now()
	return n.data, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// fmt.Println("Writing:", n.Name, "len(req.Data):", len(req.Data), "n.Attrs.Size:", n.Attrs.Size)

	// OpenFlags???
	PrintDebug("req.Offset:", req.Offset, "\n")
	PrintDebug("req.Data:", req.Data, "\n")
	PrintDebug("req.WriteFlags:", req.Flags, "\n")
	PrintDebug("req.OpenFlags:", req.FileFlags, "\n")
	// fmt.Println("len(n.data):", len(n.data), "n.Attrs.Size:", n.Attrs.Size, "req.Offset:", req.Offset, "len(req.Data):", len(req.Data))

	// pdLength := req.Offset + int64(len(req.Data)) - int64(n.Attrs.Size)
	pdLength := req.Offset + int64(len(req.Data)) - int64(len(n.data))
	if pdLength > 0 {
		padding := make([]uint8, pdLength)
		n.data = append(n.data, padding...)
	}
	copy(n.data[req.Offset:], req.Data)

	n.Attrs.Size = uint64(len(n.data))
	if n.Attrs.Size != 0 {
		n.Attrs.Blocks = uint64(n.Attrs.Size/512 + 1)
	}
	now_time := time.Now()
	n.Attrs.Atime = now_time
	n.Attrs.Mtime = now_time
	// n.Attrs.Uid = uint32(uid)
	// n.Attrs.Gid = uint32(gid)

	resp.Size = len(req.Data)
	// fmt.Println("resp.Size:", resp.Size)

	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// fmt.Println("in flush")
	n.Attrs.Atime = time.Now()
	return nil
}

func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// fmt.Println("remove:", req.Name)
	if _, ok := n.children[req.Name]; ok {
		if n.children[req.Name].Attrs.Mode.IsDir() {
			delete(n.children, req.Name)
		} else {
			n.children[req.Name].Attrs.Nlink--
			if n.children[req.Name].Attrs.Nlink == 0 {
				delete(n.children, req.Name)
			}
		}
		now_time := time.Now()
		n.Attrs.Atime = now_time
		n.Attrs.Mtime = now_time
	}
	return nil
}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	// fmt.Println("In Rename:", "req.NewDir:", req.NewDir, "req.OldName:", req.OldName, "req.NewName:", req.NewName)

	var node *DNode
	node = newDir.(*DNode)
	// fmt.Println("node children:", len(node.children), "node name:", node.Name)
	// for key, _ := range node.children {
	// 	fmt.Println("Key:", key)
	// }
	// fmt.Println("n.Name:", n.Name, "n.children:", len(n.children))
	// for key, _ := range n.children {
	// 	fmt.Println("Key:", key)
	// }
	oldNode := n.children[req.OldName]
	oldNode.Name = req.NewName
	now_time := time.Now()
	oldNode.Attrs.Atime = now_time
	oldNode.Attrs.Ctime = now_time
	node.children[req.NewName] = oldNode
	delete(n.children, req.OldName)
	return nil
}

func PrintAssert(cond bool, s string, args ...interface{}) {
	if !cond {
		PrintExit(s, args...)
	}
}

func PrintDebug(s string, args ...interface{}) {
	if !Debug {
		return
	}
	PrintAlways(s, args...)
}

func PrintExit(s string, args ...interface{}) {
	PrintAlways(s, args...)
	os.Exit(0)
}

func PrintAlways(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

func PrintCall(s string, args ...interface{}) {
	if printCalls {
		fmt.Printf(s, args...)
	}
}
