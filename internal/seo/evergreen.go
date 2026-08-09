package seo

// evergreenRule keeps a generator's spoken/on-screen BODY copy evergreen: it must
// speak to the capability — what the system does — never naming the release, its
// version, or the repository, and never framing the piece as a changelog. It
// governs PROSE only; structural metadata (repository/version fields, canonical
// URLs) is unaffected. Appended to a model prompt after its grounding clause.
const evergreenRule = ` Keep the copy EVERGREEN: never say "this release", "the release", "this update", "this version", or "this feature", never state a version number, and never name the repository. Describe what the system DOES, not that a version shipped it (say "CloudWatch logs every exit inline", not "this release adds logging"), and never frame it as "by the numbers" or a commit/file/line count.`
