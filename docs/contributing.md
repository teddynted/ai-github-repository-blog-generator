# Contributing

Thanks for your interest in improving the **GitHub AI Blog Generator**! This guide covers the development workflow, conventions, and standards for contributions.

Related: [Local Development](./local-development.md) · [CI/CD](./ci-cd.md) · [Roadmap](./roadmap.md).

---

## 1. Development Workflow

1. **Find or open an issue** describing the change ([Issue Reporting](#8-issue-reporting)).
2. **Fork** the repo (external contributors) or create a branch (maintainers).
3. **Branch** from `main` using the naming convention below.
4. **Develop** with tests; keep changes focused and small.
5. **Run checks locally** before pushing (see below).
6. **Open a Pull Request** against `main` and fill in the template.
7. **Address review** feedback; keep the branch up to date.
8. **Squash-merge** once approved and green.

Local checks before pushing:

```bash
# Go (registration, webhook-handler, instance-starter, idle-shutdown)
for fn in registration webhook-handler instance-starter idle-shutdown; do
  ( cd lambdas/$fn && gofmt -l . && go vet ./... && go test ./... -race )
done

# CloudFormation
cfn-lint infrastructure/*.yaml

# Shell
shellcheck scripts/*.sh instance/user-data.sh
```

---

## 2. Branch Naming Strategy

`<type>/<short-description>` in kebab-case, optionally with an issue number.

| Prefix | Use for |
| --- | --- |
| `feat/` | New feature |
| `fix/` | Bug fix |
| `docs/` | Documentation only |
| `chore/` | Tooling, deps, housekeeping |
| `refactor/` | Non-behavioral code change |
| `test/` | Tests only |
| `ci/` | CI/CD changes |

Examples: `feat/multi-language-output`, `fix/123-clone-timeout`, `docs/architecture-diagram`.

---

## 3. Commit Message Conventions

Follow **[Conventional Commits](https://www.conventionalcommits.org/)**:

```text
<type>(<optional scope>): <description>

[optional body]

[optional footer(s)]
```

Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `perf`, `build`.

Examples:

```text
feat(analyzer): detect Dockerfiles for tech tagging
fix(handler): reject deliveries with a missing signature header
docs(workflows): add Mermaid diagram for publishing flow
```

- Use the imperative mood ("add", not "added").
- Reference issues in the footer: `Closes #123`.
- Keep the subject ≤ 72 characters.

---

## 4. Pull Request Process

A good PR:

- Is **focused** — one logical change.
- **Links the issue** it addresses (`Closes #NN`).
- Includes **tests** for new behavior and updates docs when behavior/interfaces change.
- Passes all CI checks: lint, `go test`, `cfn-lint` + change set, security scan ([CI/CD](./ci-cd.md)).
- Explains **what** and **why**, and notes any infrastructure/cost impact.

PR checklist (include in the description):

- [ ] Tests added/updated and passing
- [ ] `gofmt` clean / `cfn-lint` passes
- [ ] Docs updated (if applicable)
- [ ] No secrets committed
- [ ] CloudFormation change set reviewed (if infra changed)

---

## 5. Code Review Guidelines

**For authors:** keep PRs small, respond to every comment, and prefer follow-up issues over scope creep.

**For reviewers:**

- Be specific and kind; suggest, don't demand.
- Check correctness, security (least privilege, secrets), tests, and cost impact.
- Approve when it's *better than before*, not only when perfect — file follow-ups for the rest.
- At least one approval is required; infrastructure changes should get extra scrutiny on the CloudFormation change set.

---

## 6. Coding Standards

**Go**

- Format with `gofmt`; pass `go vet` and (where configured) `golangci-lint`.
- Handle every error explicitly; wrap with context.
- Keep functions small and testable; table-driven tests.
- No secrets or full source payloads in logs.

**CloudFormation**

- Pass `cfn-lint`; one nested stack per concern; typed parameters with descriptions.
- No hardcoded account IDs, secrets, or Regions — use parameters and pseudo-parameters (`AWS::Region`, `AWS::AccountId`).
- Least-privilege IAM; explicit resource ARNs over wildcards.
- Tag resources via stack-level `Tags`.

**n8n workflows**

- Export to `workflows/*.json`; reference credentials by ID, never by value.
- Keep each workflow single-purpose and composable ([Workflows](./workflows.md)).

---

## 7. Documentation Standards

- Write in **GitHub-flavored Markdown**.
- Use **tables** for structured data and **Mermaid** for diagrams.
- Cross-reference other docs with **relative links**.
- Mark unimplemented features explicitly as **future work** — avoid silent placeholders.
- Keep terminology consistent with the rest of `docs/`.
- Update docs in the **same PR** as the behavior they describe.

---

## 8. Issue Reporting

Open an issue for bugs, features, or questions. A good bug report includes:

- **Summary** and expected vs. actual behavior.
- **Reproduction steps** (repo URL used, config, environment).
- **Logs** (redacted — no secrets), run ID if available.
- **Environment**: Region, model ID, cfn-lint/Go versions.

For features, describe the **use case** and **why** it matters; link to the [Roadmap](./roadmap.md) if relevant.

> **Security issues:** do **not** file a public issue. Report privately per [Security → Reporting a Vulnerability](./security.md#11-reporting-a-vulnerability).

---

## 9. Code of Conduct

Be respectful and constructive. Assume good intent, keep discussions technical, and help make this a welcoming project for contributors of all backgrounds.

---

Thank you for contributing! 🎉
