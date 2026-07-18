package shorts

// repoName returns the repository's full name, grounded in the artifacts.
func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.VoiceOver.Metadata.Repository, pkg.YouTube.Metadata.Repository, "this project")
}

// releaseTag returns the release tag, grounded in the artifacts.
func releaseTag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Release, pkg.VoiceOver.Metadata.Release, pkg.YouTube.Metadata.Release)
}

// repoURL returns the repository URL, grounded in the Release Context.
func repoURL(pkg ReleasePackage) string {
	if pkg.Context != nil {
		if pkg.Context.Repository.URL != "" {
			return pkg.Context.Repository.URL
		}
		if pkg.Context.Repository.FullName != "" {
			return "https://github.com/" + pkg.Context.Repository.FullName
		}
	}
	return ""
}

// groundedRefs is the set of reference strings a Visual may legitimately cite:
// Mermaid diagram sources, the repo URL/name, storyboard scene sources, and
// YouTube chapter titles. Validation uses it to reject ungrounded visuals.
func groundedRefs(pkg ReleasePackage) map[string]bool {
	refs := map[string]bool{}
	add := func(s string) {
		if s = collapse(s); s != "" {
			refs[s] = true
		}
	}
	add(repoURL(pkg))
	add(repoName(pkg))
	add(releaseTag(pkg))
	if c := pkg.Context; c != nil {
		for _, d := range c.Mermaid {
			add(d.Source)
		}
		for _, svc := range c.Architecture.AWSServices {
			add(svc)
		}
		for _, d := range c.RepositoryStructure.Directories {
			add(d.Path)
		}
	}
	for _, sc := range pkg.Storyboard.Scenes {
		for _, d := range sc.Diagrams {
			add(d.Source)
		}
	}
	for _, ch := range pkg.YouTube.Chapters {
		add(ch.Title)
		for _, v := range ch.VisualReferences {
			add(v)
		}
	}
	return refs
}
