package governance

import "strings"

// factualLeadIns mark sentences that assert something concrete and therefore
// warrant grounding (as opposed to generic marketing/CTA lines).
var factualLeadIns = []string{
	"uses", "built with", "powered by", "runs on", "deployed", "provisions",
	"implements", "supports", "integrates", "reduces", "improves", "adds",
	"introduces", "leverages", "based on", "architecture", "event-driven",
}

// awsTokens are AWS-looking tokens whose presence makes a sentence a factual
// claim that must be grounded.
var awsTokens = []string{
	"aws", "amazon", "lambda", "s3", "dynamodb", "sqs", "sns", "eventbridge",
	"cloudformation", "ec2", "ecs", "eks", "bedrock", "api gateway", "cloudwatch",
	"rds", "aurora", "fargate", "step functions", "kinesis", "cloudfront",
}

// GroundingEngine verifies that factual claims in the content trace back to the
// grounding terms (Release Context, git, CHANGELOG, docs, architecture, CFN).
type GroundingEngine struct{}

// Verify checks each factual-looking sentence against the grounding term set. A
// claim is grounded when it shares a grounded term; unverifiable claims are
// flagged. Content with no grounding terms is treated as unverifiable (fail
// closed) so hallucinated content is never approved.
func (GroundingEngine) Verify(c Content) GroundingResult {
	terms := loweredSet(c.Grounding.Terms)
	// The repo name and release tag are always grounded.
	if c.Grounding.Repository != "" {
		terms[strings.ToLower(lastPath(c.Grounding.Repository))] = true
	}
	if c.Grounding.Release != "" {
		terms[strings.ToLower(c.Grounding.Release)] = true
	}

	var unverified []string
	checked, grounded := 0, 0
	for _, sent := range sentences(c.Body) {
		if !isFactualClaim(sent) {
			continue
		}
		checked++
		if claimGrounded(sent, terms) {
			grounded++
		} else {
			unverified = append(unverified, sent)
		}
	}

	// No factual claims → vacuously grounded, but only if there ARE grounding
	// terms to have grounded against (otherwise we can't verify anything).
	verified := false
	switch {
	case len(terms) == 0:
		verified = false // fail closed — nothing to verify against
	case checked == 0:
		verified = true
	default:
		verified = grounded == checked
	}

	return GroundingResult{
		Verified:         verified,
		CheckedClaims:    checked,
		GroundedClaims:   grounded,
		UnverifiedClaims: topStrings(unverified, 8),
	}
}

// isFactualClaim reports whether a sentence asserts something concrete enough to
// require grounding.
func isFactualClaim(sent string) bool {
	l := strings.ToLower(sent)
	for _, t := range awsTokens {
		if strings.Contains(l, t) {
			return true
		}
	}
	for _, lead := range factualLeadIns {
		if strings.Contains(l, lead) {
			return true
		}
	}
	return false
}

// claimGrounded reports whether a sentence shares a grounded term.
func claimGrounded(sent string, terms map[string]bool) bool {
	for term := range terms {
		if term != "" && containsWord(sent, term) {
			return true
		}
	}
	return false
}

func loweredSet(items []string) map[string]bool {
	set := map[string]bool{}
	for _, s := range items {
		if s = strings.ToLower(strings.TrimSpace(s)); s != "" {
			set[s] = true
		}
	}
	return set
}

func lastPath(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
