package main

import (
	"os"
	"path/filepath"
)

type NewArgs struct {
	Name       string    `arg:"positional,required"    help:"Name for the new hback repo"`
}

func neww(args NewArgs) error {
	err := os.Mkdir(args.Name, 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(filepath.Join(args.Name, "backups"), 0755)
	if err != nil {
		return err
	}
	err = os.Mkdir(filepath.Join(args.Name, "hashes"), 0755)
	if err != nil {
		return err
	}
	return nil
}
