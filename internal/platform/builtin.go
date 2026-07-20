package platform

// init auto-registers every built-in reference provider into the process-wide
// default registry — the plugin-discovery pattern. Importing internal/platform is
// enough to make the reference providers available; a production provider does
// the same from its own package init(), so provider selection is never
// hard-coded and adding a provider never edits existing code.
func init() {
	RegisterDefaults(defaultRegistry)
}

// RegisterDefaults registers all built-in reference providers into a registry.
// It is exported so tests and embedders can populate a fresh registry without
// relying on global state.
func RegisterDefaults(r *Registry) {
	registerLLM(r)
	registerImage(r)
	registerVideo(r)
	registerVoice(r)
	registerTranslation(r)
	registerVector(r)
	registerPublishing(r)
	registerMCP(r)
	registerWorkflow(r)
	registerContent(r)
}
