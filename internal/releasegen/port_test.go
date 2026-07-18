package releasegen

import "github.com/teddynted/ai-github-repository-blog-generator/internal/ollama"

// Compile-time proof that the local Ollama client satisfies the Model port, so
// the generation engine runs on the platform's existing local inference without
// any adapter. A future Amazon Bedrock client will satisfy the same port.
var _ Model = (*ollama.Client)(nil)
