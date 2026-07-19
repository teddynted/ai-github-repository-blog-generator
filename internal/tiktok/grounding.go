package tiktok

// repoName returns the repository's full name, grounded in the artifacts.
func repoName(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Repository.FullName != "" {
		return pkg.Context.Repository.FullName
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Repository, pkg.VoiceOver.Metadata.Repository, pkg.YouTube.Metadata.Repository, pkg.Shorts.Metadata.Repository, "this project")
}

// releaseTag returns the release tag, grounded in the artifacts.
func releaseTag(pkg ReleasePackage) string {
	if pkg.Context != nil && pkg.Context.Release.Tag != "" {
		return pkg.Context.Release.Tag
	}
	return firstNonEmpty(pkg.Storyboard.Metadata.Release, pkg.VoiceOver.Metadata.Release, pkg.YouTube.Metadata.Release, pkg.Shorts.Metadata.Release)
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

// diagramSource returns the first Mermaid diagram source, grounded.
func diagramSource(pkg ReleasePackage) string {
	if c := pkg.Context; c != nil {
		for _, d := range c.Mermaid {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	for _, sc := range pkg.Storyboard.Scenes {
		for _, d := range sc.Diagrams {
			if d.Source != "" {
				return d.Source
			}
		}
	}
	return ""
}

func awsServices(pkg ReleasePackage, n int) []string {
	if pkg.Context == nil {
		return nil
	}
	return topStrings(pkg.Context.Architecture.AWSServices, n)
}

// groundedRefs is the set of reference strings a Visual may legitimately cite:
// Mermaid sources, the repo, AWS services, storyboard scene sources, YouTube
// chapter titles/visuals, and the source Shorts' visual references. Validation
// uses it to reject ungrounded visuals.
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
	for _, s := range pkg.Shorts.Shorts {
		for _, v := range s.Visuals {
			add(v.Reference)
		}
	}
	return refs
}
