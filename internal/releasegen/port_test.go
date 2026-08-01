package releasegen

import "github.com/teddynted/ai-github-repository-blog-generator/internal/anthropic"

// Compile-time proof that the Bedrock/Anthropic clients satisfy the Model port, so
// the generation engine runs on the platform's existing local inference without
// any adapter. A future Amazon Bedrock client will satisfy the same port.
var _ Model = (*anthropic.Client)(nil)
