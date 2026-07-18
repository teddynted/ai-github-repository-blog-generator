package storyboard

import "strings"

// maxSnippetBytes bounds a stored code snippet.
const maxSnippetBytes = 600

// planCode finds fenced code blocks in a section body and turns each into a
// visualization instruction chosen from the language and content.
func planCode(body string) []CodeRef {
	blocks := fencedBlocks(body)
	refs := make([]CodeRef, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b.Code) == "" {
			continue
		}
		refs = append(refs, CodeRef{
			Language:    normalizeLang(b.Lang),
			Instruction: instructionFor(b.Lang, b.Code),
			Snippet:     truncate(b.Code, maxSnippetBytes),
		})
	}
	return refs
}

func normalizeLang(l string) string {
	switch l {
	case "yml":
		return "yaml"
	case "sh", "shell", "console":
		return "bash"
	case "":
		return "text"
	default:
		return l
	}
}

// instructionFor picks a visualization instruction for a code block.
func instructionFor(lang, code string) string {
	l := normalizeLang(lang)
	switch l {
	case "yaml":
		if strings.Contains(code, "Type: AWS::") || strings.Contains(code, "AWSTemplateFormatVersion") {
			return "Zoom into CloudFormation resource"
		}
		return "Reveal YAML progressively"
	case "go":
		return "Highlight function"
	case "bash":
		return "Terminal demonstration"
	case "json":
		return "Highlight configuration"
	case "yaml-workflow":
		return "Highlight GitHub workflow"
	default:
		if strings.Contains(code, "jobs:") && strings.Contains(code, "steps:") {
			return "Highlight GitHub workflow"
		}
		return "Code highlight"
	}
}
