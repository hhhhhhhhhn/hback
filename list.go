package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

type ListArgs struct {
	Repo       string    `arg:"env:HBACK_REPO,required,-r"  help:"Path to the hback repo"`
}

func list(args ListArgs) error {
	repoFolder := filepath.Join(args.Repo, "backups")

	entries, err := os.ReadDir(repoFolder)
	if err != nil {
		return err
	}
	backups := []Backup{}
	for _, entry := range entries {
		bytes, err := os.ReadFile(filepath.Join(repoFolder, entry.Name()))
		if err != nil {
			return err
		}
		var backup Backup
		err = json.Unmarshal(bytes, &backup)
		if err != nil {
			return err
		}
		backups = append(backups, backup)
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Date.After(backups[j].Date)
	})

	headers := []string{"NAME", "DATE", "TIME", "ID"}
	rows := [][]string{}

	for _, backup := range backups {
		rows = append(rows, []string{backup.Name, backup.Date.Format("Jan 2, 2006"), backup.Date.Format("15:04"), backup.Id})
	}

	printTable(headers, rows)
	return nil
}
