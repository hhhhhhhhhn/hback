package main

import (
	"path/filepath"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"context"
	"os"
	"os/signal"
	"syscall"
	"sync"
)

var hashToInodeNumber      map[string]uint64
var hashToInodeNumberMutex sync.Mutex
var repoPath               string

type BackupNode struct {
	fs.Inode

	hashOrId string   // Corresponds to ID, in the case of backups
	isBackup bool
}
var _ fs.NodeReaddirer = (*BackupNode)(nil)
var _ fs.NodeLookuper = (*BackupNode)(nil)
var _ fs.NodeOpener = (*BackupNode)(nil)

func getHashOrIdInodeNumber(hashOrId string) uint64 {
	hashToInodeNumberMutex.Lock()
	defer hashToInodeNumberMutex.Unlock()
	if _, ok := hashToInodeNumber[hashOrId]; !ok {
		hashToInodeNumber[hashOrId] = uint64(len(hashToInodeNumber))
	}
	return hashToInodeNumber[hashOrId]
}

func (node *BackupNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// There are three cases, it is the root, which displays all the backups,
	// it is one of the backups, which contains multiple files/folders
	// or it a folder inside a backup
	if node.IsRoot() {
		backups, err := listBackups(repoPath)
		if err != nil {
			return nil, syscall.ENOENT
		}
		children := []fuse.DirEntry{}
		for _, backup := range backups {
			children = append(children, fuse.DirEntry{
				Name: backup.Id,
				Ino:  getHashOrIdInodeNumber(backup.Id),
			})
		}
		return fs.NewListDirStream(children), 0
	} else if node.isBackup {
		var backup Backup
		err := readFileJson(filepath.Join(repoPath, "backups", node.hashOrId), &backup)
		if err != nil {
			return nil, syscall.ENOENT
		}
		children := []fuse.DirEntry{}
		for _, child := range backup.Content.Children {
			children = append(children, fuse.DirEntry{
				Name: child.Name,
				Ino:  getHashOrIdInodeNumber(child.Hash),
			})
		}
		return fs.NewListDirStream(children), 0
	} else {
		var dir BackupDir
		err := readFileJson(filepath.Join(repoPath, "hashes", node.hashOrId), &dir)
		if err != nil {
			return nil, syscall.ENOENT
		}
		children := []fuse.DirEntry{}
		for _, child := range dir.Children {
			children = append(children, fuse.DirEntry{
				Name: child.Name,
				Ino:  getHashOrIdInodeNumber(child.Hash),
			})
		}
		return fs.NewListDirStream(children), 0
	}
}

func (node *BackupNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Read comment in Readdir
	if node.IsRoot() {
		backups, err := listBackups(repoPath)
		if err != nil {
			return nil, syscall.ENOENT
		}
		for _, backup := range backups {
			if backup.Id == name {
				stable := fs.StableAttr{
					Mode: fuse.S_IFDIR,
					Ino:  getHashOrIdInodeNumber(backup.Id),
				}
				operations := &BackupNode{hashOrId: backup.Id, isBackup: true}
				return node.NewInode(ctx, operations, stable), 0
			}
		}
		return nil, syscall.ENOENT
	} else if node.isBackup {
		var backup Backup
		err := readFileJson(filepath.Join(repoPath, "backups", node.hashOrId), &backup)
		if err != nil {
			return nil, syscall.ENOENT
		}
		for _, child := range backup.Content.Children {
			if child.Name == name {
				stable := fs.StableAttr{Ino: getHashOrIdInodeNumber(child.Hash)}
				if child.IsDir {
					stable.Mode = fuse.S_IFDIR
				} else {
					stable.Mode = fuse.S_IFREG
				}
				operations := &BackupNode{hashOrId: child.Hash, isBackup: false}
				return node.NewInode(ctx, operations, stable), 0
			}
		}
		return nil, syscall.ENOENT
	} else {
		var dir BackupDir
		err := readFileJson(filepath.Join(repoPath, "hashes", node.hashOrId), &dir)
		if err != nil {
			return nil, syscall.ENOENT
		}
		for _, child := range dir.Children {
			if child.Name == name {
				stable := fs.StableAttr{Ino: getHashOrIdInodeNumber(child.Hash)}
				if child.IsDir {
					stable.Mode = fuse.S_IFDIR
				} else {
					stable.Mode = fuse.S_IFREG
				}
				operations := &BackupNode{hashOrId: child.Hash, isBackup: false}
				return node.NewInode(ctx, operations, stable), 0
			}
		}
		return nil, syscall.ENOENT
	}
}

func (node *BackupNode) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	if flags & (syscall.O_WRONLY | syscall.O_APPEND | syscall.O_CREAT) != 0  {
		return nil, 0, syscall.ENOTSUP
	}
	path := filepath.Join(repoPath, "hashes", node.hashOrId)
	file, err := syscall.Open(path, int(flags), 0)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}
	loopbackFile := fs.NewLoopbackFile(file)
	return loopbackFile, 0, 0
}

type MountArgs struct {
	Repo       string    `arg:"env:HBACK_REPO,required,-r"  help:"Path to the hback repo"`
	Dest       string    `arg:"positional,required"         help:"Mount point"`
}

func mount(args MountArgs) error {
 	hashToInodeNumber = make(map[string]uint64)
	repoPath = args.Repo

	os.Mkdir(args.Dest, 0755)
	root := &BackupNode{hashOrId: "", isBackup: false}
	server, err :=  fs.Mount(args.Dest, root, &fs.Options{MountOptions: fuse.MountOptions{Debug: verbose}})
	if err != nil {
		return err
	}

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGINT, syscall.SIGTERM)
	<-channel
	server.Unmount()

	return nil
}
