package reposource

import "github.com/teddynted/ai-github-repository-blog-generator/internal/processing"

// Compile-time checks that the real implementations satisfy the processing ports.
var (
	_ processing.Cloner          = (*GitCloner)(nil)
	_ processing.ReadmeRetriever = FSReadme{}
	_ processing.DocsRetriever   = FSDocs{}
	_ processing.CommitRetriever = GitCommits{}
	_ processing.Analyzer        = FSAnalyzer{}
	_ TokenSource                = (*MetaTokenSource)(nil)
)
