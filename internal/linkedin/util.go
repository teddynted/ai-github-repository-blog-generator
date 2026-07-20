package linkedin

import "strings"

func wordCount(s string) int { return len(strings.Fields(s)) }

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func topStrings(list []string, n int) []string {
	if len(list) > n {
		return list[:n]
	}
	return list
}

func firstSentences(s string, n int) string {
	s = collapse(s)
	if s == "" {
		return ""
	}
	var out []string
	start, count := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		out = append(out, strings.TrimSpace(s[start:i+1]))
		start = i + 1
		count++
		if count >= n {
			break
		}
	}
	if count == 0 {
		return s
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}

func slug(s string) string { return strings.ToLower(collapse(s)) }

func joinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}

func truncateWords(s string, max int) string {
	f := strings.Fields(s)
	if len(f) <= max {
		return strings.Join(f, " ")
	}
	return strings.Join(f[:max], " ") + "…"
}
