package xthread

import (
	"context"
	"fmt"
	"strings"
)

// minPosts / maxPosts bound a configurable thread length.
const (
	minPosts          = 3
	maxPostsPerThread = 10
)

// block is a candidate post before it is finalized into a ThreadPost.
type block struct {
	content string
	code    string
	visual  string
}

// buildThread composes one grounded thread of the requested length. The opening
// hook may be LLM-sharpened; every other post is assembled deterministically and
// trimmed to the platform's character limit.
func (g *Generator) buildThread(ctx context.Context, pkg ReleasePackage, c threadCandidate, length, id int) Thread {
	length = clamp(length, minPosts, maxPostsPerThread)
	takeaways := planTakeaways(pkg, c.Type)
	snippet := ""
	switch c.Type {
	case "Implementation Deep Dive", "Feature Breakdown", "Developer Tips":
		snippet = extractSnippet(pkg)
	}
	visuals := planVisualRefs(pkg, c)

	posts := g.composePosts(ctx, pkg, c, length, takeaways, snippet, visuals)

	return Thread{
		ID:               id,
		Type:             c.Type,
		Title:            threadTitle(pkg, c),
		Audience:         c.Audience,
		Length:           len(posts),
		Posts:            posts,
		Summary:          planSummary(pkg, c),
		KeyTakeaways:     takeaways,
		EngagementPrompt: engagementPrompt(c),
		CTA:              planCTA(pkg, c),
		Hashtags:         planHashtags(pkg, c),
		VisualReferences: visuals,
	}
}

// composePosts builds the ordered posts: a hook first, a CTA last, and grounded
// body posts (context, per-takeaway points, code, stack) in between, trimmed to
// the requested length.
func (g *Generator) composePosts(ctx context.Context, pkg ReleasePackage, c threadCandidate, length int, takeaways []string, snippet string, visuals []VisualRef) []ThreadPost {
	repo := repoShort(pkg)
	t := tag(pkg)

	var pool []block

	// 1. Hook (optionally LLM-sharpened).
	hook := g.hook(ctx, c, hookDraft(c, repo, t))
	pool = append(pool, block{content: hook, visual: firstVisual(visuals)})

	// 2. Context — only when it doesn't name the repository or release version
	// (threads stay evergreen; the substance is in the takeaways/body).
	if s := firstSentences(c.Seed, 2); s != "" && collapse(s) != collapse(hook) && !namesReleaseIdentity(s, pkg) {
		pool = append(pool, block{content: s})
	}

	// 3. Code snippet (grounded from the blog) — placed before the takeaways so
	//    it survives the length trim on code-focused threads.
	if snippet != "" {
		pool = append(pool, block{content: "Here's the core of it 👇", code: snippet})
	}

	// 4. Grounded points (one takeaway per post).
	for _, h := range takeaways {
		pool = append(pool, block{content: "• " + collapse(h)})
	}

	// 5. Stack line.
	if stack := topStrings(append(append([]string{}, awsServices(pkg)...), technologies(pkg)...), 5); len(stack) > 0 {
		pool = append(pool, block{content: "Built with " + joinAnd(stack) + "."})
	}

	// Assemble: hook first, then middle up to length-1, then CTA last.
	kept := pool
	if len(kept) > length-1 {
		kept = kept[:length-1]
	}

	posts := make([]ThreadPost, 0, len(kept)+1)
	for i, b := range kept {
		posts = append(posts, finalizePost(i+1, b))
	}
	// Final CTA post.
	posts = append(posts, finalizePost(len(posts)+1, ctaBlock(pkg, c)))

	// Renumber contiguously.
	for i := range posts {
		posts[i].Index = i + 1
	}
	return posts
}

func finalizePost(index int, b block) ThreadPost {
	content := fitChars(b.content, MaxPostChars)
	return ThreadPost{
		Index:           index,
		Content:         content,
		CodeSnippet:     b.code,
		VisualReference: b.visual,
		CharacterCount:  runeLen(content),
	}
}

// ctaBlock is the closing post: engagement prompt + CTA + hashtags (trimmed to
// fit the character limit).
func ctaBlock(pkg ReleasePackage, c threadCandidate) block {
	var parts []string
	if p := engagementPrompt(c); p != "" {
		parts = append(parts, p)
	}
	if cta := planCTA(pkg, c); cta != "" {
		parts = append(parts, cta)
	}
	content := strings.Join(parts, "\n\n")
	// Append hashtags if they fit.
	if tags := planHashtags(pkg, c); len(tags) > 0 {
		withTags := content + "\n\n" + strings.Join(tags, " ")
		if runeLen(withTags) <= MaxPostChars {
			content = withTags
		}
	}
	return block{content: content}
}

// hookDraft is a problem-first thread opener. Like the LinkedIn openers, it does
// NOT name the repository or release version — a thread should read as an
// evergreen engineering note, not a release announcement. The repo/tag args are
// retained for signature stability but intentionally unused.
func hookDraft(c threadCandidate, _, _ string) string {
	switch c.Type {
	case "Release Announcement":
		return "Some infrastructure work worth a short thread. 🧵"
	case "Feature Breakdown":
		return "A short thread on a design choice from recent work 🧵"
	case "Architecture Walkthrough":
		return "A note on the architecture behind this one — a thread 🧵"
	case "AWS Best Practices":
		return "An AWS pattern worth stealing 🧵"
	case "AI Engineering Insights":
		return "Notes on the AI-engineering side of this work 🧵"
	case "Engineering Lessons Learned":
		return "A lesson worth writing down from recent infrastructure work 🧵"
	case "Implementation Deep Dive":
		return "How this actually works under the hood — a thread 🧵"
	case "Performance Improvements":
		return "Reliability work that quietly pays off 🧵"
	case "Developer Tips":
		return "A small workflow win worth sharing 🧵"
	case "Open Source Update":
		return "An open-source infrastructure update 🧵"
	default:
		return "A few technical notes from recent work 🧵"
	}
}

func hookPrompt(c threadCandidate, draft string) string {
	return fmt.Sprintf(
		"You are writing the opening post of a technical X thread (type: %s, audience: %s). "+
			"Rewrite the DRAFT into ONE punchy opening post UNDER 280 characters that makes engineers want to read on. "+
			"Confident, NOT clickbait, technically accurate. Use ONLY the facts in the draft — invent nothing. "+
			"Output only the post.\n\nDRAFT: %s",
		c.Type, c.Audience, draft)
}

func threadTitle(pkg ReleasePackage, c threadCandidate) string {
	return c.Type + " — " + repoShort(pkg) + " " + tag(pkg)
}

func firstVisual(visuals []VisualRef) string {
	if len(visuals) > 0 {
		return visuals[0].Reference
	}
	return ""
}
