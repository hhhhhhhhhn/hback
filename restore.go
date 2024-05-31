package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"errors"
	"path/filepath"
)

type RestoreArgs struct {
	Id          string    `arg:"positional,required"         help:"ID of the backup"`
	Destination string    `arg:"positional,required"         help:"Destination folder"`
	Repo        string    `arg:"env:HBACK_REPO,required,-r"  help:"Path to the hback repo"`
}

func copyFile(dest, src string) error {
	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer destFile.Close()

	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func readFileJson(file string, dest any) error {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("Could not read file %v, error: %v", file, err)
	}

	err = json.Unmarshal(bytes, dest)
	if err != nil {
		return fmt.Errorf("Could parse file %v, error: %v", file, err)
	}
	return nil
}

func restoreRecursive(entry BackupDirEntry, destination string, hashesDir string) error {
	if entry.IsDir == false {
		return copyFile(filepath.Join(destination, entry.Name), filepath.Join(hashesDir, entry.Hash))
	}

	var dir BackupDir
	err := readFileJson(filepath.Join(hashesDir, entry.Hash), &dir)
	if err != nil {
		return err
	}

	newPath := filepath.Join(destination, entry.Name)
	err = os.Mkdir(newPath, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	for _, child := range dir.Children {
		err = restoreRecursive(child, newPath, hashesDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func restore(args RestoreArgs) error {
	backupPath := filepath.Join(args.Repo, "backups", args.Id)
	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("Backup %s does not exist", args.Id)
		}
		return err
	}

	var backup Backup
	err = json.Unmarshal(backupBytes, &backup)
	if err != nil {
		return err
	}

	if _, err := os.Stat(args.Destination); err == nil {
		return fmt.Errorf("Destination %s already exists", args.Destination)
	}
	err = os.Mkdir(args.Destination, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	for _, dirEntry := range backup.Content.Children {
		err = restoreRecursive(dirEntry, args.Destination, filepath.Join(args.Repo, "hashes"))
		if err != nil {
			return err
		}
	}

	return nil
}
