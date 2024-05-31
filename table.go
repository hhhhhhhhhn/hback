package main

import (
	"fmt"
)

const padding = "      "
func printTable(headers []string, rows [][]string) {
	maxColumnLengths := make([]int, len(headers))
	for c := range headers {
		maxColumnLength := len(headers[c])
		for r := range rows {
			if len(rows[r][c]) > maxColumnLength {
				maxColumnLength = len(rows[r][c])
			}
		}
		maxColumnLengths[c] = maxColumnLength
	}

	for c, header := range headers {
		if c > 0 {
			fmt.Print(padding)
		}
		fmt.Printf("%-*v", maxColumnLengths[c], header)
	}
	fmt.Print("\n")

	for _, row := range rows {
		for c := range row {
			if c > 0 {
				fmt.Print(padding)
			}
			fmt.Printf("%-*v", maxColumnLengths[c], row[c])
		}
		fmt.Print("\n")
	}
}
