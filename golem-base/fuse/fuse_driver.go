package fuse

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Config holds the filesystem configuration
type Config struct {
	MountPoint string
	SourceDir  string
	TargetFile string // File to intercept writes for
	ChunkSize  int    // default 4096
}

// FuseDriver represents a mounted FUSE filesystem with lifecycle management
type FuseDriver struct {
	config      *Config
	server      *fuse.Server
	mounted     chan struct{}
	done        chan struct{}
	err         error
	errMux      sync.Mutex
	events      []FuseEvent
	eventsMux   sync.RWMutex
	eventNotify chan struct{}
}

// Mount starts the FUSE filesystem in a separate goroutine
func Mount(config *Config) (*FuseDriver, error) {
	// Create mount point if it doesn't exist
	if err := os.MkdirAll(config.MountPoint, 0755); err != nil {
		return nil, err
	}

	if config.ChunkSize == 0 {
		config.ChunkSize = 4096
	}

	mfs := &FuseDriver{
		config:      config,
		mounted:     make(chan struct{}),
		done:        make(chan struct{}),
		events:      make([]FuseEvent, 0),
		eventNotify: make(chan struct{}, 1),
	}

	// Start mounting in background
	go mfs.serve()

	// Wait for mount to complete
	<-mfs.mounted

	mfs.errMux.Lock()
	err := mfs.err
	mfs.errMux.Unlock()

	if err != nil {
		return nil, err
	}

	log.Debug("Mounted at: %s", config.MountPoint)
	log.Debug("Source directory: %s", config.SourceDir)
	log.Debug("Intercepting writes for: %s", config.TargetFile)

	return mfs, nil
}

func (fd *FuseDriver) serve() {
	log.Debug("Starting FUSE filesystem serve")
	defer func() {
		log.Debug("Server goroutine finishing...")
		close(fd.done)
	}()

	// Create root node
	root := &Node{
		path:   fd.config.SourceDir,
		config: fd.config,
		driver: fd,
	}

	// Mount options for SQLite compatibility
	opts := &fs.Options{
		MountOptions: fuse.MountOptions{
			AllowOther:           true,
			FsName:               "interceptfs",
			Name:                 "interceptfs",
			MaxReadAhead:         128 * 1024,
			MaxWrite:             128 * 1024,
			EnableLocks:          true,
			IgnoreSecurityLabels: true,
		},
		AttrTimeout:  nil, // No caching for SQLite safety
		EntryTimeout: nil,
	}

	// Mount the filesystem
	server, err := fs.Mount(fd.config.MountPoint, root, opts)
	if err != nil {
		fd.setError(err)
		close(fd.mounted)
		return
	}

	fd.server = server
	close(fd.mounted)

	// Wait for unmount (blocking)
	server.Wait()
}

func (fd *FuseDriver) setError(err error) {
	log.Debug("Setting FUSE error: %v", err)
	fd.errMux.Lock()
	defer fd.errMux.Unlock()
	if fd.err == nil {
		fd.err = err
	}
}

// Unmount unmounts the filesystem gracefully
func (fd *FuseDriver) Unmount() error {
	log.Debug("Unmounting FUSE filesystem")
	if fd.server == nil {
		return nil
	}

	if err := fd.server.Unmount(); err != nil {
		log.Error("Server unmount failed, trying system unmount: %v", err)

		// Fallback to system unmount command
		// Try fusermount first (Linux)
		cmd := exec.Command("fusermount", "-u", fd.config.MountPoint)
		if cmdErr := cmd.Run(); cmdErr != nil {
			// Try umount (macOS/BSD)
			cmd = exec.Command("umount", fd.config.MountPoint)
			if cmdErr := cmd.Run(); cmdErr != nil {
				// Try diskutil (macOS)
				cmd = exec.Command("diskutil", "unmount", "force", fd.config.MountPoint)
				if cmdErr := cmd.Run(); cmdErr != nil {
					log.Error("All unmount methods failed: %v", cmdErr)
					return err
				}
			}
		}
	}

	<-fd.done
	log.Debug("Unmounted", "mountPoint", fd.config.MountPoint)

	fd.errMux.Lock()
	defer fd.errMux.Unlock()
	return fd.err
}

// Wait blocks until the filesystem is unmounted
func (fd *FuseDriver) Wait() error {
	log.Debug("Waiting for FUSE filesystem to finish")
	<-fd.done
	fd.errMux.Lock()
	defer fd.errMux.Unlock()
	return fd.err
}

// Event interface for different event types
type Event interface {
	GetPath() string
}

// AddWriteEvent records a write event
func (fd *FuseDriver) AddEvent(event Event) {
	fd.eventsMux.Lock()
	fd.events = append(fd.events, event.(FuseEvent)) // Type assertion
	fd.eventsMux.Unlock()

	// Notify any listeners
	select {
	case fd.eventNotify <- struct{}{}:
	default:
		// Channel already has notification pending
	}
}

// GetEvents returns all events and optionally clears them
func (fd *FuseDriver) GetEvents(clear bool) []FuseEvent {
	fd.eventsMux.Lock()
	defer fd.eventsMux.Unlock()

	result := make([]FuseEvent, len(fd.events))
	copy(result, fd.events)

	if clear {
		fd.events = fd.events[:0]
	}

	return result
}

// WaitForEvents blocks until new events arrive or timeout
func (fd *FuseDriver) WaitForEvents(timeout time.Duration) bool {
	select {
	case <-fd.eventNotify:
		return true
	case <-time.After(timeout):
		return false
	}
}

// EventCount returns the current number of events
func (fd *FuseDriver) EventCount() int {
	fd.eventsMux.RLock()
	defer fd.eventsMux.RUnlock()
	return len(fd.events)
}

// Node represents a filesystem node (file or directory)
type Node struct {
	fs.Inode
	path   string
	config *Config
	driver *FuseDriver
}

var _ = (fs.NodeLookuper)((*Node)(nil))
var _ = (fs.NodeReaddirer)((*Node)(nil))
var _ = (fs.NodeOpener)((*Node)(nil))
var _ = (fs.NodeGetattrer)((*Node)(nil))
var _ = (fs.NodeSetattrer)((*Node)(nil))
var _ = (fs.NodeCreater)((*Node)(nil))
var _ = (fs.NodeMkdirer)((*Node)(nil))
var _ = (fs.NodeUnlinker)((*Node)(nil))
var _ = (fs.NodeRmdirer)((*Node)(nil))
var _ = (fs.NodeRenamer)((*Node)(nil))
var _ = (fs.NodeStatfser)((*Node)(nil))

// Lookup looks up a child node
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Fuse->Lookup", "path", n.path, "name", name)
	fullPath := filepath.Join(n.path, name)

	info, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, syscall.ENOENT
		}
		return nil, syscall.EIO
	}

	// Create child node
	child := &Node{
		path:   fullPath,
		config: n.config,
		driver: n.driver,
	}

	// Set attributes
	n.fillAttr(info, &out.Attr)

	// Determine node type
	stable := fs.StableAttr{
		Mode: uint32(info.Mode()),
		Ino:  n.getInode(info),
	}

	return n.NewInode(ctx, child, stable), 0
}

// Readdir reads directory entries
func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	log.Debug("Fuse->Readdir", "path", n.path)
	entries, err := os.ReadDir(n.path)
	if err != nil {
		return nil, syscall.EIO
	}

	var result []fuse.DirEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		dirType := uint32(syscall.S_IFREG)
		if entry.IsDir() {
			dirType = uint32(syscall.S_IFDIR)
		}

		result = append(result, fuse.DirEntry{
			Name: entry.Name(),
			Ino:  n.getInode(info),
			Mode: dirType,
		})
	}

	return fs.NewListDirStream(result), 0
}

// Open opens a file
func (n *Node) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Fuse->Open", "path", n.path, "flags", flags)
	file, err := os.OpenFile(n.path, int(flags), 0)
	if err != nil {
		return nil, 0, syscall.EIO
	}

	handle := &FileHandle{
		file:     file,
		path:     n.path,
		config:   n.config,
		driver:   n.driver,
		isTarget: filepath.Base(n.path) == n.config.TargetFile,
	}

	// Direct I/O for SQLite
	return handle, fuse.FOPEN_DIRECT_IO, 0
}

// Getattr gets file attributes
func (n *Node) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	info, err := os.Lstat(n.path)
	if err != nil {
		return syscall.EIO
	}

	n.fillAttr(info, &out.Attr)
	return 0
}

// Setattr sets file attributes
func (n *Node) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	log.Debug("Fuse->Setattr", "path", n.path)
	// Handle size changes (truncation)
	if sz, ok := in.GetSize(); ok {
		if err := os.Truncate(n.path, int64(sz)); err != nil {
			return syscall.EIO
		}
	}

	// Handle mode changes
	if mode, ok := in.GetMode(); ok {
		if err := os.Chmod(n.path, os.FileMode(mode)); err != nil {
			return syscall.EIO
		}
	}

	// Handle ownership changes
	uid, uidOk := in.GetUID()
	gid, gidOk := in.GetGID()
	if uidOk || gidOk {
		uidVal := -1
		gidVal := -1
		if uidOk {
			uidVal = int(uid)
		}
		if gidOk {
			gidVal = int(gid)
		}
		if err := os.Chown(n.path, uidVal, gidVal); err != nil {
			return syscall.EIO
		}
	}

	// Get updated attributes
	info, err := os.Lstat(n.path)
	if err != nil {
		return syscall.EIO
	}

	n.fillAttr(info, &out.Attr)
	return 0
}

// Create creates a new file
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Debug("Fuse->Create", "path", n.path, "name", name, "flags", flags, "mode", mode)
	fullPath := filepath.Join(n.path, name)

	// Create the file
	file, err := os.OpenFile(fullPath, int(flags)|os.O_CREATE, os.FileMode(mode))
	if err != nil {
		return nil, nil, 0, syscall.EIO
	}

	// Get file info
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, 0, syscall.EIO
	}

	// Create node
	child := &Node{
		path:   fullPath,
		config: n.config,
		driver: n.driver,
	}

	// Set attributes
	n.fillAttr(info, &out.Attr)

	// Create inode
	stable := fs.StableAttr{
		Mode: uint32(info.Mode()),
		Ino:  n.getInode(info),
	}

	childInode := n.NewInode(ctx, child, stable)

	// Create file handle
	handle := &FileHandle{
		file:     file,
		path:     fullPath,
		config:   n.config,
		driver:   n.driver,
		isTarget: name == n.config.TargetFile,
	}

	return childInode, handle, fuse.FOPEN_DIRECT_IO, 0
}

// Mkdir creates a new directory
func (n *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Debug("Fuse->Mkdir", "path", n.path, "name", name, "mode", mode)
	fullPath := filepath.Join(n.path, name)

	if err := os.Mkdir(fullPath, os.FileMode(mode)); err != nil {
		return nil, syscall.EIO
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, syscall.EIO
	}

	child := &Node{
		path:   fullPath,
		config: n.config,
		driver: n.driver,
	}

	n.fillAttr(info, &out.Attr)

	stable := fs.StableAttr{
		Mode: uint32(info.Mode()),
		Ino:  n.getInode(info),
	}

	return n.NewInode(ctx, child, stable), 0
}

// Unlink removes a file
func (n *Node) Unlink(ctx context.Context, name string) syscall.Errno {
	log.Debug("Fuse->Unlink", "path", n.path, "name", name)
	fullPath := filepath.Join(n.path, name)

	if err := os.Remove(fullPath); err != nil {
		return syscall.EIO
	}

	return 0
}

// Rmdir removes a directory
func (n *Node) Rmdir(ctx context.Context, name string) syscall.Errno {
	log.Debug("Fuse->Rmdir", "path", n.path, "name", name)
	fullPath := filepath.Join(n.path, name)

	if err := os.Remove(fullPath); err != nil {
		return syscall.EIO
	}

	return 0
}

// Rename renames a file or directory
func (n *Node) Rename(ctx context.Context, oldName string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	log.Debug("Fuse->Rename", "path", n.path, "oldName", oldName, "newName", newName)
	oldPath := filepath.Join(n.path, oldName)

	newParentNode := newParent.(*Node)
	newPath := filepath.Join(newParentNode.path, newName)

	if err := os.Rename(oldPath, newPath); err != nil {
		return syscall.EIO
	}

	return 0
}

// Statfs returns filesystem statistics (required for SQLite)
func (n *Node) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	log.Debug("Fuse->Statfs", "path", n.path)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(n.path, &stat); err != nil {
		return syscall.EIO
	}

	out.Blocks = stat.Blocks
	out.Bfree = stat.Bfree
	out.Bavail = stat.Bavail
	out.Files = stat.Files
	out.Ffree = stat.Ffree
	out.Bsize = stat.Bsize
	out.Frsize = stat.Bsize
	out.NameLen = 255

	return 0
}

func (n *Node) fillAttr(info os.FileInfo, out *fuse.Attr) {
	stat := info.Sys().(*syscall.Stat_t)
	out.Size = uint64(info.Size())
	out.Mode = uint32(info.Mode())
	out.Mtime = uint64(info.ModTime().Unix())
	out.Mtimensec = uint32(info.ModTime().Nanosecond())
	out.Atime = uint64(stat.Atimespec.Sec)
	out.Atimensec = uint32(stat.Atimespec.Nsec)
	out.Ctime = uint64(stat.Ctimespec.Sec)
	out.Ctimensec = uint32(stat.Ctimespec.Nsec)
	out.Uid = stat.Uid
	out.Gid = stat.Gid
	out.Nlink = uint32(stat.Nlink)
	out.Ino = n.getInode(info)
}

func (n *Node) getInode(info os.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino
	}
	return 0
}

// FileHandle represents an open file
type FileHandle struct {
	file     *os.File
	path     string
	config   *Config
	driver   *FuseDriver
	isTarget bool
}

var _ = (fs.FileReader)((*FileHandle)(nil))
var _ = (fs.FileWriter)((*FileHandle)(nil))
var _ = (fs.FileFlusher)((*FileHandle)(nil))
var _ = (fs.FileReleaser)((*FileHandle)(nil))
var _ = (fs.FileFsyncer)((*FileHandle)(nil))

// Read reads from the file
func (fh *FileHandle) Read(ctx context.Context, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	log.Debug("Fuse->Read", "path", fh.path, "offset", off, "size", len(dest))
	n, err := fh.file.ReadAt(dest, off)
	if err != nil && err.Error() != "EOF" {
		return nil, syscall.EIO
	}
	return fuse.ReadResultData(dest[:n]), 0
}

// Write writes to the file
func (fh *FileHandle) Write(ctx context.Context, data []byte, off int64) (written uint32, errno syscall.Errno) {
	log.Debug("Fuse->Write", "path", fh.path, "offset", off, "size", len(data))
	// Intercept writes for target file
	if fh.isTarget {
		log.Debug("Fuse->Write intercepted", "file", filepath.Base(fh.path), "offset", off, "size", len(data))
		// Collect write event
		writeEvent := FuseWriteEvent{
			BaseFuseEvent: BaseFuseEvent{
				Path:      fh.path,
				Type:      "write",
				Timestamp: time.Now(),
			},
			StartIndex:    off / int64(fh.config.ChunkSize),
			ChunksChanged: int64(len(data) / fh.config.ChunkSize),
		}
		fh.driver.AddEvent(&writeEvent)
	}

	n, err := fh.file.WriteAt(data, off)
	if err != nil {
		return 0, syscall.EIO
	}

	return uint32(n), 0
}

// Flush flushes the file (critical for SQLite)
func (fh *FileHandle) Flush(ctx context.Context) syscall.Errno {
	log.Debug("Fuse->Flush", "path", fh.path)
	if err := fh.file.Sync(); err != nil {
		return syscall.EIO
	}
	return 0
}

// Fsync syncs the file (critical for SQLite)
func (fh *FileHandle) Fsync(ctx context.Context, flags uint32) syscall.Errno {
	log.Debug("Fuse->Fsync", "path", fh.path, "flags", flags)
	if err := fh.file.Sync(); err != nil {
		return syscall.EIO
	}
	return 0
}

// Release closes the file
func (fh *FileHandle) Release(ctx context.Context) syscall.Errno {
	log.Debug("Fuse->Release", "path", fh.path)
	if err := fh.file.Close(); err != nil {
		return syscall.EIO
	}
	return 0
}

func main() {
	mountPoint := flag.String("mount", "/tmp/fusemount", "Mount point")
	sourceDir := flag.String("source", "/tmp/source", "Source directory")
	targetFile := flag.String("target", "database.db", "File to intercept writes for")
	flag.Parse()

	config := &Config{
		MountPoint: *mountPoint,
		SourceDir:  *sourceDir,
		TargetFile: *targetFile,
	}

	// Ensure source directory exists
	if err := os.MkdirAll(config.SourceDir, 0755); err != nil {
		log.Error("Failed to create source directory: %v", err)
	}

	log.Debug("Starting FUSE filesystem...")

	// Mount in background (non-blocking)
	mfs, err := Mount(config)
	if err != nil {
		log.Error("Mount failed: %v", err)
	}

	// Filesystem is now mounted and ready to use
	log.Debug("Filesystem is ready! Main thread is free to continue...")

	// Example: You can now use the filesystem in your application
	// db, _ := sql.Open("sqlite3", filepath.Join(*mountPoint, "database.db"))

	// Handle graceful shutdown
	// You can unmount explicitly: mfs.Unmount()
	// Or wait for external unmount: mfs.Wait()

	if err := mfs.Wait(); err != nil {
		log.Error("Filesystem error: %v", err)
	}
}
