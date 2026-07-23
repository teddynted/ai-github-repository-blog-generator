package mcp

// This file wires the built-in server plug-ins into a registry. Each server is a
// self-contained plug-in registered by descriptor + factory; RegisterDefaults is
// the single place the default set is assembled, and package init() installs it
// into DefaultRegistry (the database/sql driver pattern). Tests build an isolated
// registry via NewRegistry + RegisterDefaults (or register a subset) so they
// never touch global state.

// DefaultPublishPlatforms are the platforms the publishing server knows about out
// of the box, with their character limits.
var DefaultPublishPlatforms = []PublishPlatform{
	{Name: "blog", MaxChars: 0},
	{Name: "linkedin", MaxChars: 3000},
	{Name: "x", MaxChars: 280},
	{Name: "youtube", MaxChars: 5000},
}

// RegisterDefaults registers every built-in MCP server into reg. Servers that
// need project data (repository, database, aws, context) are seeded with empty,
// safe defaults; callers that have real data register their own factories before
// this or into a fresh registry. It is idempotent-safe only on a fresh registry —
// a second call errors with ErrAlreadyRegistered by design (fail loud).
func RegisterDefaults(reg *Registry) error {
	regs := []struct {
		d ServerDescriptor
		f Factory
	}{
		{filesystemDescriptor(), func() (Server, error) { return newFilesystemServer(".", 0), nil }},
		{documentationDescriptor(), func() (Server, error) { return newDocumentationServer("docs"), nil }},
		{repositoryDescriptor(), func() (Server, error) { return newRepositoryServer(RepoSnapshot{}), nil }},
		{databaseDescriptor(), func() (Server, error) { return newDatabaseServer(DBSnapshot{}), nil }},
		{cloudformationDescriptor(), func() (Server, error) { return newCloudFormationServer(), nil }},
		{mermaidDescriptor(), func() (Server, error) { return newMermaidServer(), nil }},
		{awsDescriptor(), func() (Server, error) { return newAWSServer(AWSMock{}), nil }},
		{publishingDescriptor(), func() (Server, error) { return newPublishingServer(DefaultPublishPlatforms), nil }},
		{contextDescriptor(), func() (Server, error) { return newContextServer(nil), nil }},
	}
	for _, r := range regs {
		if err := reg.Register(r.d, r.f); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	if err := RegisterDefaults(DefaultRegistry); err != nil {
		panic(err)
	}
}
