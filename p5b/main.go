// memfs implements a simple in-memory file system.
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	// . "github.com/mattn/go-getopt"
	"golang.org/x/net/context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	. "gitlab.cs.umd.edu/cmsc818eFall20/cmsc818e-zliu1238/p5b/store"
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
	Name       string
	Attrs      fuse.Attr
	ParentSig  string
	Version    int
	PrevSig    *string
	ChildSigs  map[string]string
	DataBlocks []string

	sig       string
	dirty     bool
	metaDirty bool
	expanded  bool
	parent    *DNode
	children  map[string]*DNode
	data      []byte
}

type Head struct {
	Root    string
	NextInd uint64
	Replica uint64
}

var (
	root       *DNode
	Debug      = false
	resetfs    = false
	printCalls = false
	conn       *fuse.Conn
	// mountPoint = "818fs"      // default mount point
	mountPoint     = "/dss"       // default mount point
	uid            = os.Geteuid() // use the same uid/gid for all
	gid            = os.Getegid()
	nextInode      = 0
	ServerAddress  string
	anchorRootName             = "HEAD"
	mutex                      = &sync.Mutex{}
	RDONLY_F       os.FileMode = 0400
	timePassedIn   time.Time
	nodesRead      = 0
	expansions     = 0
	bytesWritten   = 0
	serverRequests = 0
	bytesSent      = 0

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
func errCheck(err error) {
	if err != nil {
		log.Println(err)
	}
}

func main() {
	flusherInt := flag.Int("f", 5, "The flusher period")
	mntPoint := flag.String("m", "/tmp/dss", "mount point")
	reset := flag.Bool("n", false, "\"new file system\": reset the file system to initial state. Do NOT delete anything.")
	ServerAddr := flag.String("s", "", "Host / colon / port") // localhost:8888
	timestamp := flag.String("t", "", "Time travel: specifies that the file system should reflect a specific time")
	dbg := flag.Bool("d", false, "debug mode")
	flag.Parse()

	ServerAddress = *ServerAddr
	GetServerAddr(ServerAddress)

	if timestamp != nil {
		timePassedIn, _ = parseLocalTime(*timestamp)
	}

	mountPoint := *mntPoint
	Debug = *dbg
	flusherInterval := *flusherInt
	resetfs = *reset

	fmt.Println("\nCommand-line arguments:")
	fmt.Println("------------------------")
	fmt.Println("debug mode:\t", Debug)
	fmt.Println("flusher period:\t", flusherInterval)
	fmt.Println("mount point:\t", mountPoint)
	fmt.Println("new file system:\t", resetfs)
	fmt.Println("server addr:\t", ServerAddress)
	fmt.Println("time travel:\t", timePassedIn)
	fmt.Println("------------------------")

	root = new(DNode)
	root.init("", os.ModeDir|0755)

	// nodeMap[uint64(root.Attrs.Inode)] = root
	// PrintDebug("root inode %d\n", int(root.Attrs.Inode))

	if _, err := os.Stat(mountPoint); err != nil {
		os.MkdirAll(mountPoint, 0755)
	}
	fuse.Unmount(mountPoint)
	conn, err := fuse.Mount(mountPoint, fuse.FSName("dssFS"), fuse.Subtype("project P5a"),
		fuse.LocalVolume(), fuse.VolumeName("dssFS"))
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)
	flushed := make(chan bool)
	ch := make(chan os.Signal, 1)
	ticker := time.NewTicker(time.Duration(flusherInterval) * time.Second)

	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() {
		DebugPrint("here")
		<-ch

		done <- true
		<-flushed
		defer conn.Close()
		fuse.Unmount(mountPoint)

		fmt.Println("\n")
		fmt.Println(nodesRead, "nodes read")
		fmt.Println(expansions, "expansions")
		fmt.Println(bytesWritten, "bytes written")
		fmt.Println(serverRequests, "server requests")
		fmt.Println(bytesSent, "bytes sent")

		os.Exit(1)
	}()

	PrintAlways("mt on %q, debug %v\n", mountPoint, Debug)

	go func() {
		for {
			select {
			case <-done:
				flusher_thread(root)
				flushed <- true
				DebugPrint("done")
				ticker.Stop()
				return
			case <-ticker.C:
				fmt.Print(".")
				go flusher_thread(root)
			}
		}
	}()

	err = fs.Serve(conn, Dfs{})
	// PrintDebug("AFTER\n")
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-conn.Ready
	if err := conn.MountError; err != nil {
		log.Fatal(err)
	}
}

func flusher_thread(root *DNode) {
	mutex.Lock()
	DebugPrint("in flusher")
	if root.metaDirty {
		DebugPrint("dirty")
		now_time := time.Now()
		flusher(root, now_time)
		newRootSig := putDNodeToStore(root)
		head := Head{
			Root:    newRootSig,
			NextInd: uint64(nextInode),
		}
		headSig := putHeadToStore(&head)

		var KVpairs []string
		KVpairs = append(KVpairs, "rootsig")
		KVpairs = append(KVpairs, headSig)
		// Get the anchor name prom previous version
		CmdClaim("", anchorRootName, KVpairs)
		root.metaDirty = false
		DebugPrint("put root to store:", root.Name, newRootSig)
	}
	mutex.Unlock()
}

func flusher(root *DNode, t time.Time) {
	DebugPrint("in flusher!!")
	if root != nil && root.metaDirty {
		for name, node := range root.children {
			if node.metaDirty {
				node.Attrs.Mtime = t
				node.Attrs.Atime = t
				flusher(node, t)
				newDNodeSig := putDNodeToStore(node)
				DebugPrint("put to store:", name, newDNodeSig)
				parentNode := node.parent
				parentNode.ChildSigs[name] = newDNodeSig

				node.metaDirty = false
			}
		}
	}
}

func (root *DNode) init(s string, mode os.FileMode) error {
	// nodesRead++
	lastClaimSig := CmdLastclaim(anchorRootName)
	if lastClaimSig == "" || resetfs {
		// DebugPrint("Creating anchor for", anchorRootName)
		if !timePassedIn.IsZero() {
			log.Fatal("Cannot time travel before the first entry!")
			return syscall.EINVAL
		}

		root.Name = "/"
		root.Attrs.Inode = uint64(nextInode)
		nextInode++
		root.Attrs.Mode = mode
		now_time := time.Now()
		root.Attrs.Atime = now_time
		root.Attrs.Mtime = now_time
		root.Attrs.Ctime = now_time
		root.Attrs.Nlink = 1
		// root.Version = 1
		root.children = make(map[string]*DNode)
		root.ChildSigs = make(map[string]string)
		// Create HEAD anchor
		CmdAnchor("", anchorRootName)
		// Send root DNode to server
		rootDNodeSig := putDNodeToStore(root)

		// Make Head variable, claim on the "HEAD" anchor
		head := Head{
			Root:    rootDNodeSig,
			NextInd: uint64(nextInode),
		}
		headSig := putHeadToStore(&head)
		var KVpairs []string
		KVpairs = append(KVpairs, "rootsig")
		KVpairs = append(KVpairs, headSig)
		CmdClaim("", anchorRootName, KVpairs)
		DebugPrint("(headSig) KVpairs: " + KVpairs[1])

	} else {
		// retrieve from remote server
		var data Message
		var head Head
		json.Unmarshal(getBySig(lastClaimSig), &data)

		if !timePassedIn.IsZero() { // time travel, read only
			root.Attrs.Mode = os.ModeDir | 0555
			ts := data.ModTime
			for ts.After(timePassedIn) && data.Prevsig != "" {
				json.Unmarshal(getBySig(data.Prevsig), &data)
				ts = data.ModTime
			}
			if ts.After(timePassedIn) {
				log.Fatal("Time travel failed: timestamp too early")
				return syscall.EINVAL
			}
		}
		headsig := data.Adds["rootsig"]

		json.Unmarshal(getBySig(headsig), &head)
		rootsig := head.Root
		json.Unmarshal(getBySig(rootsig), root)
		root.sig = rootsig
		// for k, v := range root.ChildSigs {
		// 	fmt.Println(k, v)
		// }

		// if !timePassedIn.IsZero() { // time travel, read only
		// 	root.Attrs.Mode = os.ModeDir | 0555

		// 	for root.Attrs.Mtime.After(timePassedIn) && *root.PrevSig != "" {
		// 		DebugPrint("After: time:", root.Attrs.Mtime)
		// 		root.ChildSigs = make(map[string]string)
		// 		json.Unmarshal(getBySig(*root.PrevSig), root)
		// 		root.sig = *root.PrevSig
		// 	}
		// 	if root.Attrs.Mtime.After(timePassedIn) {
		// 		log.Fatal("Time travel failed: timestamp too early")
		// 		return syscall.EINVAL
		// 	}
		// 	if root.Attrs.Mtime.Before(timePassedIn) {
		// 		DebugPrint("Before: ", root.Attrs.Mtime, root.sig)
		// 		json.Unmarshal(getBySig(root.sig), root)
		// 		// for k, v := range root.ChildSigs {
		// 		// 	fmt.Println(k, v)
		// 		// }
		// 	}
		// 	// for k, v := range root.ChildSigs {
		// 	// 	fmt.Println(k, v)
		// }
		root.expand()
		// Todo: iterate files/dirs under root
		// workpath, _ := os.Getwd()
		// mntpath := filepath.Join(workpath, mountPoint)
		// createTree(root, mntpath)
	}

	return nil
}

func (n *DNode) expand() error {
	// nodesRead++
	if !n.expanded {
		expansions++
		if n.Attrs.Mode.IsDir() {
			if n.children == nil {
				n.children = make(map[string]*DNode)
			}
			for nodeName, nodeSig := range n.ChildSigs {
				// nodesRead++
				DebugPrint("nodeName:", nodeName, "nodeSig", nodeSig)
				var node DNode
				json.Unmarshal(getBySig(nodeSig), &node)
				if &node == nil {
					log.Fatal("Couldn't find signature for:", nodeName)
					return syscall.EINVAL
				}
				if !timePassedIn.IsZero() { // time travel, read only
					n.Attrs.Mode = os.ModeDir | 0555
				}
				node.parent = n
				node.sig = nodeSig
				n.children[nodeName] = &node
			}
		} else {
			if !timePassedIn.IsZero() { // read only
				n.Attrs.Mode = RDONLY_F
			}
			var databuf []byte
			for _, dataSig := range n.DataBlocks {
				databytes := getBySig(dataSig)
				databuf = append(databuf, databytes...)
			}
			n.data = databuf
		}
		n.expanded = true
	}
	return nil
}

func (Dfs) Root() (n fs.Node, err error) {
	return root, nil
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	// nodesRead++
	mutex.Lock()

	n.expand()
	// DebugPrint("attr")
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
	mutex.Unlock()

	return nil
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	nodesRead++
	// DebugPrint("Lookup")
	n.expand()

	n.Attrs.Atime = time.Now()
	// n.Attrs.Mtime = time.Now()

	if _, ok := n.children[name]; ok {
		return n.children[name], nil
	}

	return nil, fuse.ENOENT
}

func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	mutex.Lock()
	nodesRead++
	n.expand()

	// DebugPrint("ReadDirAll\n")
	n.Attrs.Atime = time.Now()
	// n.Attrs.Mtime = time.Now()

	dirDirs := []fuse.Dirent{}
	var nodeType fuse.DirentType
	for name, node := range n.children {
		nodesRead++
		if n.Attrs.Mode.IsDir() {
			nodeType = fuse.DT_Dir
		} else {
			nodeType = fuse.DT_File
		}
		nextDir := fuse.Dirent{Inode: node.Attrs.Inode, Name: name, Type: nodeType}
		dirDirs = append(dirDirs, nextDir)
	}
	mutex.Unlock()

	return dirDirs, nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	// DebugPrint("in Getattr:", len(n.data), "n.Attrs.Size:", n.Attrs.Size)
	mutex.Lock()
	// nodesRead++
	n.expand()

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
	mutex.Unlock()

	return nil
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	fmt.Println("fsync")
	n.Attrs.Mtime = time.Now()
	return nil
}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	mutex.Lock()
	// nodesRead++
	n.expand()
	if req.Valid.Size() {
		n.Attrs.Size = req.Size
	}
	if req.Valid.Atime() {
		n.Attrs.Atime = req.Atime
	}
	if req.Valid.Mtime() {
		n.Attrs.Mtime = req.Mtime
	}

	if req.Valid.Mode() {
		n.Attrs.Mode = req.Mode
	}

	resp.Attr.Size = n.Attrs.Size
	resp.Attr.Atime = n.Attrs.Atime
	resp.Attr.Mtime = n.Attrs.Mtime
	resp.Attr.Mode = n.Attrs.Mode
	mutex.Unlock()

	return nil
}

func (p *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	mutex.Lock()
	nodesRead += 3
	p.expand()
	// DebugPrint("in mkdir")
	// Read-only
	var newNode DNode
	if p.Attrs.Mode == os.ModeDir|0555 {
		// log.Println(syscall.EPERM)
		mutex.Unlock()
		return nil, syscall.EPERM
	}
	DebugPrint("req.Name:", req.Name)
	DebugPrint("req.Mode:", req.Mode)
	newNode.Name = req.Name
	newNode.children = make(map[string]*DNode)
	newNode.ChildSigs = make(map[string]string)
	newNode.Attrs.Inode = uint64(nextInode)
	nextInode++
	now_time := time.Now()
	newNode.Attrs.Atime = now_time
	newNode.Attrs.Mtime = now_time
	newNode.Attrs.Ctime = now_time
	newNode.Attrs.Mode = req.Mode
	newNode.Attrs.Nlink = 1
	newNode.Version = 1
	newNode.PrevSig = nil

	newNode.ParentSig = p.sig
	newNode.parent = p

	// version dir
	if strings.HasSuffix(req.Name, "@@versions") {
		newNode.Attrs.Mode = os.ModeDir | 0555 // read-only

		filename := strings.Split(req.Name, "@@")[0]
		// DebugPrint("filename:", filename)

		if _, ok := p.children[filename]; ok {
			p.children[req.Name] = &newNode

			currsig := p.children[filename].sig
			for currsig != "" {
				var versionNode DNode
				versionNode.Attrs.Mode = RDONLY_F // read-only
				json.Unmarshal(getBySig(currsig), &versionNode)
				DebugPrint("version node data:", versionNode.DataBlocks)
				DebugPrint("prevsig:", *versionNode.PrevSig)
				versionNode.Name = filename + "." + versionNode.Attrs.Mtime.Format("2006-01-02 15:04:05")

				// fetch data
				for _, dataSig := range versionNode.DataBlocks {
					DebugPrint("dataSig:", dataSig)
					versionNode.data = append(versionNode.data, getBySig(dataSig)...)
				}
				newNode.children[versionNode.Name] = &versionNode
				currsig = *versionNode.PrevSig
			}

		} else {
			log.Println("Couldn't find file:", filename)
			mutex.Unlock()
			return &newNode, nil
		}
	} else { // normal dir
		// createDNodeUpdateHead(&newNode)
		markMetaDirty(&newNode)
		p.children[req.Name] = &newNode

	}
	mutex.Unlock()

	// p.children[req.Name] = &newNode
	return &newNode, nil
}

func (p *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	mutex.Lock()
	// nodesRead++
	p.expand()

	// DebugPrint("in create")
	var newNode DNode
	if p.Attrs.Mode == os.ModeDir|0555 {
		// log.Println(syscall.EPERM)
		mutex.Unlock()
		return nil, nil, syscall.EPERM
	}
	newNode.Name = req.Name
	newNode.Attrs.Inode = uint64(nextInode)
	nextInode++
	newNode.Attrs.Size = 0
	now_time := time.Now()
	newNode.Attrs.Atime = now_time
	newNode.Attrs.Mtime = now_time
	newNode.Attrs.Ctime = now_time
	newNode.Attrs.Mode = req.Mode
	newNode.Attrs.Nlink = 1
	newNode.Version = 1
	newNode.ParentSig = p.sig
	newNode.PrevSig = nil
	newNode.parent = p

	// createDNodeUpdateHead(&newNode)

	markMetaDirty(&newNode)

	p.children[req.Name] = &newNode
	mutex.Unlock()

	return &newNode, &newNode, nil
}

func (n *DNode) ReadAll(ctx context.Context) ([]byte, error) {
	mutex.Lock()
	nodesRead++
	n.expand()

	// DebugPrint("ReadAll\n")
	n.Attrs.Atime = time.Now()
	// n.Attrs.Mtime = time.Now()
	// n.Attrs.Ctime = time.Now()

	mutex.Unlock()

	return n.data, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	mutex.Lock()
	// log.Println("n.Name:", n.Name, n.Attrs.Mode, "parent.Name:", n.parent.Name)
	if n.Attrs.Mode == RDONLY_F || n.Attrs.Mode == os.ModeDir|0555 {
		log.Println(n.Name, syscall.EPERM)
		mutex.Unlock()
		return syscall.EPERM
	}

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
	// n.Attrs.Ctime = now_time
	n.dirty = true
	resp.Size = len(req.Data)
	bytesWritten += len(req.Data)

	// DebugPrint("resp.Size:", resp.Size)
	markMetaDirty(n)
	mutex.Unlock()

	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	mutex.Lock()
	n.expand()
	// DebugPrint("in flush")
	DebugPrint("in flush")

	if n.Attrs.Mode == RDONLY_F {
		// log.Println(syscall.EPERM)
		mutex.Unlock()
		return syscall.EPERM
	}
	var offset uint64 = 0
	var chunklist []string

	if n.dirty {
		for offset < n.Attrs.Size {
			chunkSize := rkchunk(n.data[offset:], uint64(n.Attrs.Size-offset))
			chunkBuf := make([]byte, chunkSize)
			copy(chunkBuf, n.data[offset:])
			offset += uint64(chunkSize)

			sig := putToStore(chunkBuf)
			if sig == "" {
				errCheck(errors.New("Failed to get the signature."))
			}
			chunklist = append(chunklist, sig)
		}

		n.DataBlocks = chunklist
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		// n.Attrs.Ctime = time.Now()

		// createDNodeUpdateHead(n)
		markMetaDirty(n)
		n.dirty = false
	}
	mutex.Unlock()

	return nil
}

// Todo: delete remote data
func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// DebugPrint("remove:", req.Name, "children length:", len(n.children[req.Name].children))
	mutex.Lock()
	// DebugPrint("in remove")
	n.expand()

	if strings.HasSuffix(req.Name, "@@versions") {
		delete(n.children, req.Name)
	} else {
		if n.Attrs.Mode == RDONLY_F || n.Attrs.Mode == os.ModeDir|0555 {
			// log.Println(syscall.EPERM)
			mutex.Unlock()
			return syscall.EPERM
		}
		if _, ok := n.children[req.Name]; ok {
			if n.children[req.Name].Attrs.Mode.IsDir() {
				if len(n.children[req.Name].children) == 0 {
					delete(n.children, req.Name)
					delete(n.ChildSigs, req.Name)
				} else {
					fmt.Println("Directory not empty")
					mutex.Unlock()
					return syscall.EINVAL
				}

			} else {
				delete(n.children, req.Name)
				delete(n.ChildSigs, req.Name)
			}
			now_time := time.Now()
			n.Attrs.Atime = now_time
			n.Attrs.Mtime = now_time

			// createDNodeUpdateHead(n)

			markMetaDirty(n)
		}
	}
	mutex.Unlock()

	return nil
}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	mutex.Lock()

	DebugPrint("Rename:", "n.name:", n.Name, "req.OldName:", req.OldName, "req.newName:", req.NewName)
	// fmt.Println("in rename")
	var node *DNode
	node = newDir.(*DNode)

	n.expand()
	node.expand()
	n.children[req.OldName].expand()
	n.children[req.OldName].expand()
	if n.children[req.OldName].Attrs.Mode == RDONLY_F || n.children[req.OldName].Attrs.Mode == os.ModeDir|0555 {
		// log.Println(syscall.EPERM)
		mutex.Unlock()
		return syscall.EPERM
	}
	if node.Attrs.Mode == os.ModeDir|0555 || n.Attrs.Mode == os.ModeDir|0555 {
		// log.Println(syscall.EPERM)
		mutex.Unlock()
		return syscall.EPERM
	}
	DebugPrint("newDir.Name:", node.Name)
	oldNode := n.children[req.OldName]
	oldNode.Name = req.NewName
	now_time := time.Now()
	oldNode.Attrs.Atime = now_time
	oldNode.Attrs.Mtime = now_time
	n.Attrs.Atime = now_time
	n.Attrs.Mtime = now_time
	node.Attrs.Atime = now_time
	node.Attrs.Mtime = now_time

	node.children[req.NewName] = oldNode
	delete(n.children, req.OldName)

	oldNode.ParentSig = node.sig
	oldNode.parent = node

	// node.ChildSigs[req.NewName] = n.ChildSigs[req.OldName]
	delete(n.ChildSigs, req.OldName)

	// createDNodeUpdateHead(n)
	// createDNodeUpdateHead(node)
	markMetaDirty(n)
	markMetaDirty(node.children[req.NewName])
	mutex.Unlock()
	return nil
}

func PrintAssert(cond bool, s string, args ...interface{}) {
	if !cond {
		PrintExit(s, args...)
	}
}

func DebugPrint(s string, args ...interface{}) {
	if !Debug {
		return
	}
	fmt.Println(s, args)
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

func parseLocalTime(tm string) (time.Time, error) {
	loc, _ := time.LoadLocation("America/New_York")
	return time.ParseInLocation("2006-1-2T15:04", tm, loc)
}

func postToServer(msg *Message, addr string, errMsg string) map[string]interface{} {
	// fmt.Println("msg:", msg.TreeTarget)
	URL := "http://" + addr
	// DebugPrint(URL)
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(msg)
	bytesSent += buf.Len()

	// fmt.Println("URL: ", URL)

	req, _ := http.NewRequest("POST", URL, buf)
	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	errCheck(err)

	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := ioutil.ReadAll(res.Body)
		os.Stderr.WriteString(fmt.Sprint(res.StatusCode) + ": " + string(b) + "\n")
		if errMsg != "" {
			os.Stderr.WriteString(errMsg + "\n")
		}
		return nil
	}

	var data map[string]interface{}
	body, err := ioutil.ReadAll(res.Body)

	if len(body) == 0 {
		return data
	}
	err = json.Unmarshal([]byte(body), &data)
	errCheck(err)

	// io.Copy(os.Stdout, res.Body)
	return data
}

// Get data by signature
func getBySig(sig string) []byte {
	msg := Message{
		Version: 1,
		Type:    "get",
		Sig:     sig,
	}
	data := postToServer(&msg, ServerAddress+"/json", "Get Request Failed.")
	serverRequests++

	bytebuf, err := base64.StdEncoding.DecodeString(data["Data"].(string))
	errCheck(err)
	return bytebuf
}

// return signature
func putToStore(byteBuf []byte) string {
	msg := Message{
		Version: 1,
		Type:    "put",
		Data:    byteBuf,
	}
	data := postToServer(&msg, ServerAddress+"/json", "")
	serverRequests++
	sig := data["Sig"].(string)
	return sig
}

// Return: Sig of the DNode
func putDNodeToStore(node *DNode) string {
	(*node).PrevSig = &(*node).sig
	(*node).Version++
	nodeJs, err := json.MarshalIndent(*node, "", " ")
	errCheck(err)
	newDNodeSig := putToStore(nodeJs)
	(*node).sig = newDNodeSig
	DebugPrint("newDNodeSig: " + newDNodeSig)
	return newDNodeSig
}

func putHeadToStore(h *Head) string {
	hJs, err := json.MarshalIndent(*h, "", " ")
	errCheck(err)
	hSig := putToStore(hJs)
	DebugPrint("hSig: " + hSig)
	return hSig
}

func markMetaDirty(n *DNode) {
	n.metaDirty = true
	DebugPrint("markMetaDirty: " + n.Name)

	currNode := n
	for currNode.Name != "/" {
		parentNode := currNode.parent
		DebugPrint("markMetaDirty: " + parentNode.Name)
		parentNode.metaDirty = true
		currNode = parentNode
	}
}

// Put a DNode to store, update parent's node, new claim on head
func createDNodeUpdateHead(newNode *DNode) string {
	mutex.Lock()
	// Write to object store
	newDNodeSig := putDNodeToStore(newNode)

	// Modify parent DNodes until it hits the root
	currNode := newNode
	for currNode.Name != "/" {
		DebugPrint("*****************8")
		parentNode := currNode.parent
		DebugPrint("current node: " + currNode.Name + " parent name: " + parentNode.Name)
		parentNode.ChildSigs[currNode.Name] = newDNodeSig
		// for next iteration
		newDNodeSig = putDNodeToStore(parentNode) // parent's new DNode sig
		DebugPrint(parentNode.Name + " Sig: " + newDNodeSig)
		currNode = parentNode
	}
	DebugPrint("newDNodeSig Sig: " + newDNodeSig)

	head := Head{
		Root:    newDNodeSig,
		NextInd: uint64(nextInode),
	}
	headSig := putHeadToStore(&head)

	var KVpairs []string
	KVpairs = append(KVpairs, "rootsig")
	KVpairs = append(KVpairs, headSig)
	// Get the anchor name prom previous version
	CmdClaim("", anchorRootName, KVpairs)
	DebugPrint("(headSig) KVpairs: " + KVpairs[1])
	mutex.Unlock()
	return newDNodeSig
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
