package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	arg "github.com/alexflint/go-arg"
	// "encoding/json"
)

type Backup struct {
	Name        string     `json:"name"`
	Date        time.Time  `json:"date"`
	Content     BackupDir  `json:"content"`
	Id          string     `json:"id"`        // Name + "." + Unix timestamp
}

type BackupDir struct {
	Children []BackupDirEntry  `json:"children"` // Sorted alphabetically
}

type BackupDirEntry struct {
	Name     string            `json:"name"`
	Hash     string            `json:"hash"`
	IsDir    bool              `json:"isdir"`
	Created  time.Time         `json:"created"`
	Modified time.Time         `json:"modified"`
}

var verbose bool
func logVerbose(args ...any) {
	if verbose {
		fmt.Fprintln(os.Stderr, args...)
	}
}

type SaveArgs struct {
	Files      []string  `arg:"positional,required"         help:"Files or folders to be backed up"`
	Name       string    `arg:"-b,required"                 help:"Name of the backup"`
	Repo       string    `arg:"env:HBACK_REPO,required,-r"  help:"Path to the hback repo"`
}

type void struct{}
func getKnownHashes(repoDir string) (map[string]void, error) {
	hashes, err := os.ReadDir(filepath.Join(repoDir, "hashes"))
	if err != nil {
		return nil, errors.New("Could not open hback repo: " + err.Error())
	}
	hashMap := make(map[string]void)
	for _, hash := range hashes {
		hashMap[hash.Name()] = struct{}{}
	}
	return hashMap, nil
}

func backupFileIfNeededAndGetHash(path string, knownHashes map[string]void, hashesDir string) (string, error) {
	file, err := os.Open(path)
	defer file.Close()

	if err != nil {
		return "", err
	}
	bfile := bufio.NewReader(file)
	hash := sha256.New()

	_, err = io.Copy(hash, bfile)
	if err != nil {
		return "", err
	}

	hashSum := fmt.Sprintf("%x", hash.Sum(nil))
	if _, ok := knownHashes[hashSum]; !ok {
		logVerbose("Backing up", path, "with hash", hashSum)
		hashFilePath := filepath.Join(hashesDir, hashSum)
		hashFile, err := os.OpenFile(hashFilePath, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return "", err
		}
		defer hashFile.Close()
		file.Seek(0, 0)

		_, err = io.Copy(hashFile, file)
		if err != nil {
			return "", err
		}
		knownHashes[hashSum] = void{}
	} else {
		logVerbose("Skipping already backed up", path, "with hash", hashSum)
	}

	return hashSum, nil
}

// TODO: Reuse hashes
// Non-recursive, only backups the metadata of the files, not its contents
func backupDirIfNeededAndGetHash(dir BackupDir, knownHashes map[string]void, hashesDir string) (string, error) {
	hash := sha256.New()

	for _, child := range dir.Children {
		hash.Write([]byte(child.Name))
		hash.Write([]byte(child.Hash))
		hash.Write([]byte(fmt.Sprint(child.Created.Unix())))
		hash.Write([]byte(fmt.Sprint(child.Created.Unix())))
		hash.Write([]byte(fmt.Sprint(child.IsDir)))
	}

	hashSum := fmt.Sprintf("%x", hash.Sum(nil))

	if _, ok := knownHashes[hashSum]; !ok {
		logVerbose("Backing up dir", dir, "with hash", hashSum)
		hashFilePath := filepath.Join(hashesDir, hashSum)
		data, err := json.Marshal(dir)
		if err != nil {
			return "", err
		}
		os.WriteFile(hashFilePath, data, 0644)
		knownHashes[hashSum] = void{}
	} else {
		logVerbose("Skipping already backed up dir", dir, "with hash", hashSum)
	}

	return hashSum, nil
}

func backupPath(absPath string, knownHashes map[string]void, hashesDir string) (BackupDirEntry, error) {
	stat, err := os.Stat(absPath)
	if err != nil {
		return BackupDirEntry{}, err
	}

	if stat.IsDir() {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return BackupDirEntry{}, err
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		children := []BackupDirEntry{}
		for _, entry := range entries {
			child, err := backupPath(filepath.Join(absPath, entry.Name()), knownHashes, hashesDir)
			if err != nil {
				return BackupDirEntry{}, err
			}
			children = append(children, child)
		}
		dir := BackupDir{
			Children: children,
		}
		hash, err := backupDirIfNeededAndGetHash(dir, knownHashes, hashesDir)
		if err != nil {
			return BackupDirEntry{}, err
		}
		return BackupDirEntry{
			Name:     filepath.Base(absPath),
			Modified: stat.ModTime(),
			Created:  stat.ModTime(), // TODO: Actually check creation time
			IsDir:    true,
			Hash:     hash,
		}, nil
	}

	// It is a file
	hash, err := backupFileIfNeededAndGetHash(absPath, knownHashes, hashesDir)
	if err != nil {
		return BackupDirEntry{}, err
	}
	return BackupDirEntry {
		Name:     filepath.Base(absPath),
		Modified: stat.ModTime(),
		Created:  stat.ModTime(), // TODO: Actually check creation time
		IsDir:    false,
		Hash:     hash,
	}, nil
}

func save(args SaveArgs) error {
	knownHashes, err := getKnownHashes(args.Repo)
	if err != nil {
		return err
	}

	content := BackupDir{}
	for _, path := range args.Files {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		entry, err := backupPath(absPath, knownHashes, filepath.Join(args.Repo, "hashes"))
		if err != nil {
			return err
		}
		content.Children = append(content.Children, entry)
	}

	backup := Backup{
		Name:    args.Name,
		Date:    time.Now(),
		Id:      fmt.Sprintf("%s.%d", args.Name, time.Now().Unix()),
		Content: content,
	}

	backupJson, err := json.Marshal(backup)
	if err != nil {
		return err
	}

	os.Mkdir(filepath.Join(args.Repo, "backups"), 0755)
	err = os.WriteFile(filepath.Join(args.Repo, "backups", backup.Id), backupJson, 0644)
	if err != nil {
		return err
	}

	logVerbose("Succesfully saved backup", backup.Name)
	return nil
}

func main() {
	var args struct {
		Verbose  bool      `arg:"-v"               help:"Be verbose"`                   // Handled by a global variable

		SaveArgs *SaveArgs `arg:"subcommand:save"  help:"Save folders to a hback repo"`
	}
	arg.MustParse(&args)

	verbose = args.Verbose

	var err error
	if(args.SaveArgs != nil) {
		err = save(*args.SaveArgs)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
