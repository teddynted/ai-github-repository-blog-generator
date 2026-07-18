package voiceover

import "strings"

// planTransition returns the spoken bridge from the current scene into the next,
// so the video flows as one continuous explanation. It is deterministic and
// grounded — it uses the next scene's own title/type, never invented content.
// The final scene gets a closing line instead of a bridge.
func planTransition(curType, nextType, nextTitle string, isLast bool) string {
	if isLast {
		return closingLine(curType)
	}
	lead := leadOut(curType)
	intro := leadIn(nextType, nextTitle)
	return strings.TrimSpace(lead + " " + intro)
}

// leadOut acknowledges what the current scene just covered.
func leadOut(curType string) string {
	switch curType {
	case "introduction":
		return "With the stage set,"
	case "problem":
		return "With the problem clear,"
	case "architecture", "diagram":
		return "Now that we've walked the architecture,"
	case "cloudformation":
		return "With the infrastructure defined,"
	case "repository":
		return "Having toured the code,"
	case "implementation":
		return "With the implementation in place,"
	case "results":
		return "With the results in hand,"
	case "lessons":
		return "With those lessons noted,"
	default:
		return "From here,"
	}
}

// leadIn points at the next scene by its real title.
func leadIn(nextType, nextTitle string) string {
	title := strings.TrimSpace(nextTitle)
	switch nextType {
	case "architecture", "diagram":
		return "let's step through the architecture."
	case "cloudformation":
		return "let's look at how it's provisioned."
	case "repository":
		return "let's open the repository."
	case "implementation":
		return "let's see how it's built."
	case "results":
		return "let's look at what it delivers."
	case "lessons":
		return "let's draw out the practical takeaways."
	case "conclusion":
		return "let's wrap up."
	default:
		if title != "" {
			return "let's move on to " + lowerFirst(title) + "."
		}
		return "let's move on."
	}
}

func closingLine(curType string) string {
	switch curType {
	case "conclusion":
		return "That's the release end to end — thanks for watching."
	default:
		return "That brings us to the end — thanks for watching."
	}
}

// lowerFirst lowercases the first rune of s (for mid-sentence titles).
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
