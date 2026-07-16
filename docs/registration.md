# Repository Registration (POST /repositories)

The onboarding endpoint. It validates repository access with a GitHub PAT,
**creates the push/release webhook automatically**, stores the PAT + webhook
secret in Secrets Manager, and writes repository metadata to DynamoDB. After
registration the platform reacts to that repository's events with no further
setup.

Related: [Deployment → Register a Repository](./deployment.md#6-register-a-repository) ·
[Manual Trigger](./manual-trigger.md) · [Security](./security.md) ·
[Architecture](./architecture.md).

---

## 1. Endpoint

```text
POST /repositories
Host: https://<api-id>.execute-api.<region>.amazonaws.com/<stage>
Content-Type: application/json
x-api-key: <api key>
```

Resolve the URL, the API-key id, and the key value from stack outputs:

```bash
REGISTRATION_URL=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)

API_KEY_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationApiKeyId'].OutputValue" --output text)
API_KEY=$(aws apigateway get-api-key --api-key "$API_KEY_ID" --include-value --query value --output text)
```

## 2. Authentication

Requires an **API key** (`x-api-key`) — covered by the shared API Gateway usage
plan on the stage (the same key authorises `POST /process`). A request without a
valid key is **403 Forbidden** from API Gateway before the Lambda runs. The
endpoint is never publicly accessible.

## 3. GitHub PAT (prerequisite)

Create a **fine-grained** Personal Access Token in GitHub (**Settings → Developer
settings → Personal access tokens → Fine-grained tokens**) scoped to the target
repository with:

| Permission | Level | Why |
| --- | --- | --- |
| Contents | Read | clone + read the repo during a run |
| Webhooks | Read and write | create the push/release webhook |
| Metadata | Read | resolve repository info |

The PAT is sent **once**, at registration; it is stored in Secrets Manager and
never returned, logged, or written to DynamoDB.

## 4. Request payload

```json
{
  "repository_url": "https://github.com/acme/widget",
  "pat": "github_pat_xxx",
  "trigger_pattern": "regex:^(blog|post):"
}
```

| Field | Required | Default | Notes |
| --- | --- | --- | --- |
| `repository_url` | **yes** | — | full HTTPS repo URL (`https://github.com/<owner>/<name>`) |
| `pat` | **yes** | — | fine-grained GitHub PAT (see §3); stored, never returned |
| `trigger_pattern` | no | platform default (`blog:`) | per-repo publishing trigger — a literal prefix, `regex:<expr>`, or `[literal]`; validated |

## 5. Responses

**Success — 200 OK** (no secret material):

```json
{
  "repo_full_name": "acme/widget",
  "owner": "acme",
  "name": "widget",
  "default_branch": "main",
  "webhook_id": 512345678,
  "trigger_pattern": "blog:",
  "status": "registered"
}
```

**Errors** — `{"error":"<code>","message":"<safe message>"}`:

| Status | Code | When |
| --- | --- | --- |
| 400 | `invalid_input` | body is not JSON; `repository_url`/`pat` missing; `repository_url` unparseable; `trigger_pattern` invalid |
| 401 | `unauthorized` | GitHub rejected the token (insufficient scope or no access) |
| 404 | `not_found` | repository not found or not accessible with this token |
| 403 | — | missing/invalid `x-api-key` (API Gateway default) |
| 502 | `upstream_error` | GitHub request failed or returned an unexpected status |
| 500 | `internal_error` | storing credentials or metadata failed |

Ordering guarantees no half-registered repo: the webhook and secret are created
**before** metadata is written, so a failure never leaves a visible-but-broken
entry.

## 6. Example

```bash
curl -sS -X POST "$REGISTRATION_URL" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d '{"repository_url":"https://github.com/acme/widget","pat":"github_pat_xxx"}'
# → 200 {"repo_full_name":"acme/widget", … ,"status":"registered"}

# With a custom per-repo trigger:
#   -d '{"repository_url":"…","pat":"…","trigger_pattern":"regex:^(blog|post):"}'
```

Confirm a green **✓** under the repo's **Settings → Webhooks → Recent
Deliveries**. To create the webhook by hand instead, see
[Deployment → Manual webhook setup](./deployment.md#6-register-a-repository).

## 7. What registration does

1. Validate `repository_url` + `trigger_pattern`.
2. `GetRepository` with the PAT — proves access + read permission.
3. Generate a webhook signing secret and `CreateWebhook` for **push** and
   **release** events, pointing at `WebhookUrl` (this also proves webhook
   permission).
4. Store the PAT + webhook secret in **Secrets Manager**; keep only an opaque
   reference.
5. Write metadata to **DynamoDB**: full name, repo id, owner, name, URL, default
   branch, webhook id, trigger pattern, `enabled=true`, the secret reference, and
   the registration timestamp — **never the PAT**.

## 8. Logging

The `registration` Lambda logs the outcome only — `repo` + `webhook_id` on
success, or the error **code** on failure. The PAT and webhook secret are never
logged.

## 9. CloudFormation (`serverless.yaml`)

| Resource | Purpose |
| --- | --- |
| `RegistrationRole` | `secretsmanager:CreateSecret`/`PutSecretValue`/`TagResource` (scoped to `${ProjectName}/repos/*`), `dynamodb:PutItem`/`UpdateItem`/`GetItem` on the metadata table, CloudWatch Logs |
| `RegistrationLogGroup` | `/aws/lambda/${ProjectName}-registration`, retention-bounded |
| `RegistrationFunction` | Go Lambda (`provided.al2023`, arm64); env `REPOSITORIES_TABLE`, `SECRETS_PREFIX`, `PUBLISH_TRIGGER`, `WEBHOOK_URL` |
| `RepositoriesResource` / `RepositoriesMethod` | `POST /repositories`, `ApiKeyRequired: true`, `AWS_PROXY` integration |
| `RegistrationInvokePermission` | lets API Gateway invoke the Lambda |
| `RegistrationApiKey` / `ApiUsagePlan` / `ApiUsagePlanKey` | the API key + usage plan shared with `/process` |
| Output `RegistrationUrl` / `RegistrationApiKeyId` | endpoint URL + key id |

## 10. Deployment

No dedicated step — the `serverless` stack ships it (see
[Deployment §3–4](./deployment.md#3-build-the-lambda-functions)). After deploy,
fetch `RegistrationUrl` + the API key (§1) and POST a repository.
