package apispec_test

import (
	"fmt"
	"slices"
	"strings"
)

// lineDiff lists the hunks that turn old into new, without context lines. A
// hunk header gives the 1-based line in old and in new where it starts.
func lineDiff(oldText, newText string) string {
	oldAll, newAll := strings.Split(oldText, "\n"), strings.Split(newText, "\n")
	prefix := 0
	for prefix < len(oldAll) && prefix < len(newAll) && oldAll[prefix] == newAll[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldAll)-prefix && suffix < len(newAll)-prefix &&
		oldAll[len(oldAll)-1-suffix] == newAll[len(newAll)-1-suffix] {
		suffix++
	}
	oldLines, newLines := oldAll[prefix:len(oldAll)-suffix], newAll[prefix:len(newAll)-suffix]
	common := longestCommon(oldLines, newLines)
	var out strings.Builder
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			i, j = i+1, j+1
			continue
		}
		startOld, startNew := i+1, j+1
		var removed, added []string
		for i < len(oldLines) || j < len(newLines) {
			if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
				break
			}
			if j >= len(newLines) || (i < len(oldLines) && common[i+1][j] >= common[i][j+1]) {
				removed = append(removed, oldLines[i])
				i++
			} else {
				added = append(added, newLines[j])
				j++
			}
		}
		fmt.Fprintf(&out, "@@ -%d +%d @@\n", startOld+prefix, startNew+prefix)
		for _, line := range removed {
			out.WriteString("-" + line + "\n")
		}
		for _, line := range added {
			out.WriteString("+" + line + "\n")
		}
	}
	return out.String()
}

func longestCommon(a, b []string) [][]int {
	table := make([][]int, len(a)+2)
	for i := range table {
		table[i] = make([]int, len(b)+2)
	}
	for i := range slices.Backward(a) {
		for j := range slices.Backward(b) {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else {
				table[i][j] = max(table[i+1][j], table[i][j+1])
			}
		}
	}
	return table
}
