package contentadmin

import "strings"

// LineDiff returns a compact unified-style line diff of a→b: context lines are
// prefixed "  ", removals "- ", additions "+ ". It uses a longest-common-
// subsequence so unchanged lines align. Identical inputs yield "" (no diff).
func LineDiff(a, b string) string {
	if a == b {
		return ""
	}
	al := splitLines(a)
	bl := splitLines(b)

	// LCS length table.
	n, m := len(al), len(bl)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out strings.Builder
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case al[i] == bl[j]:
			out.WriteString("  " + al[i] + "\n")
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			out.WriteString("- " + al[i] + "\n")
			i++
		default:
			out.WriteString("+ " + bl[j] + "\n")
			j++
		}
	}
	for ; i < n; i++ {
		out.WriteString("- " + al[i] + "\n")
	}
	for ; j < m; j++ {
		out.WriteString("+ " + bl[j] + "\n")
	}
	return out.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
