package main

import (
	"fmt"
	"os"
	"time"
	"errors"

	arg "github.com/alexflint/go-arg"
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

func main() {
	var args struct {
		Verbose     bool         `arg:"-v"                 help:"Be verbose"`   // Handled by a global variable

		SaveArgs    *SaveArgs    `arg:"subcommand:save"    help:"Backup folders to a hback repo"`
		RestoreArgs *RestoreArgs `arg:"subcommand:restore" help:"Restore contents of a hback backup"`
		ListArgs    *ListArgs    `arg:"subcommand:list"    help:"List backups inside a repo"`
	}
	p := arg.MustParse(&args)

	verbose = args.Verbose

	var err error
	if args.SaveArgs != nil {
		err = save(*args.SaveArgs)
	} else if args.RestoreArgs != nil {
		err = restore(*args.RestoreArgs)
	} else if args.ListArgs != nil {
		err = list(*args.ListArgs)
	} else if p.Subcommand() == nil {
		err = errors.New("No subcommand specified. Use -h for help.")
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
