# Repository Registration (`/repositories`)

The onboarding API. It validates repository access with a GitHub PAT and stores
the repository's PAT in **one shared Secrets Manager secret** shared by all
repositories. (GitHub webhook ingress has been removed, so registration no longer
creates a GitHub webhook — unless a legacy `WebhookURL` is configured.) A
companion `DELETE` deregisters a repository.

Related: [Deployment → Register a Repository](./deployment.md#6-register-a-repository) ·
[Manual Trigger](./manual-trigger.md) · [Security](./security.md) ·
[Infrastructure](./infrastructure.md).

---

## 1. Shared credentials secret

Instead of two Secrets Manager secrets per repository, the platform keeps a
**single** secret — `blog-gen/github/repositories` — whose value is a JSON
object keyed by `"<owner>/<name>"`:

```json
{
  "octocat/widget": { "pat": "ghp_xxxxxxxx", "webhook_secret": "xxxxxxxx" },
  "acme/gadget":    { "pat": "ghp_yyyyyyyy", "webhook_secret": "yyyyyyyy" }
}
```

- **Registration** read-modify-writes this document (add/update an entry). The
  `webhook_secret` field is optional and only used when a legacy `WebhookURL` is
  configured; the PAT is the field that matters.
- **Deletion** removes an entry, leaving the rest intact.
- **Lookups** (the worker) read the secret and index by `"<owner>/<name>"` to get
  the PAT for cloning and opening pull requests.

This keeps the number of Secrets Manager resources constant (one) regardless of
how many repositories are registered.

### Concurrency

Writes are **read-modify-write**, so concurrent registrations could otherwise
lose updates. Safeguards:

1. The store **retries** the read-modify-write on transient Secrets Manager
   errors (always on).
2. **Optional single-writer serialization** via reserved concurrency: set the
   `RegistrationReservedConcurrency` parameter to `1` to make the registration
   Lambda a single writer. It is **0 (off) by default** because reserving
   concurrency requires the account's Lambda concurrency limit to leave ≥ 10
   unreserved — on low-limit accounts a value of 1 fails to deploy. Enable it
   once your account has headroom for the strongest guarantee.

Registration is a rare, typically-serial admin operation, so the retry alone is
adequate in practice. Readers are unaffected (concurrent reads are safe).

---

## 2. Endpoint & auth

```text
POST   /repositories     register or update a repository
DELETE /repositories     deregister a repository
Host: https://<api-id>.execute-api.<region>.amazonaws.com/<stage>
Content-Type: application/json
x-api-key: <api key>
```

Both methods require an **API key** (`x-api-key`) via the shared usage plan;
without it API Gateway returns **403** before the Lambda runs. Resolve the URL +
key from stack outputs:

```bash
REG=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationUrl'].OutputValue" --output text)
KEY_ID=$(aws cloudformation describe-stacks --stack-name blog-gen-serverless \
  --query "Stacks[0].Outputs[?OutputKey=='RegistrationApiKeyId'].OutputValue" --output text)
API_KEY=$(aws apigateway get-api-key --api-key "$KEY_ID" --include-value --query value --output text)
```

## 3. GitHub PAT (prerequisite)

Create a **fine-grained** PAT scoped to the target repo with **Contents: Read**,
**Pull requests: Read and write**, **Metadata: Read**. The PAT is stored in the
shared secret and never returned, logged, or written to DynamoDB.

## 4. Register — `POST /repositories`

```json
{
  "owner": "octocat",
  "repository": "designing-an-ai-agent-platform-on-aws",
  "pat": "ghp_xxxxxxxxxxxxxxxxx",
  "webhook_secret": "myWebhookSecret",
  "trigger_pattern": "regex:^(blog|post):",
  "update": false
}
```

| Field | Required | Notes |
| --- | --- | --- |
| `owner` | **yes** | repository owner/org |
| `repository` | **yes** | repository name (without owner) |
| `pat` | **yes** | fine-grained GitHub PAT; stored, never returned |
| `webhook_secret` | no | legacy HMAC secret, only used when a `WebhookURL` is configured; unused by default |
| `trigger_pattern` | no | per-repo trigger pattern stored in metadata (literal / `regex:` / `[literal]`); validated |
| `update` | no | must be `true` to overwrite an already-registered repo |

Behaviour:

- **New repo** → validate access, add the PAT entry to the shared secret, write
  metadata. → `registered`.
- **Existing repo + `update: true`** → validate access, overwrite the entry,
  update metadata. → `updated`.
- **Existing repo, `update` not set** → **409** duplicate; nothing changes.

### Responses

| Status | Body |
| --- | --- |
| **200** register | `{"status":"success","message":"Repository registered successfully.","repository":"octocat/…"}` |
| **200** update | `{"status":"success","message":"Repository credentials updated successfully.","repository":"octocat/…"}` |
| **409** duplicate | `{"status":"error","message":"Repository is already registered."}` |
| **400** validation | `{"status":"error","message":"missing required field(s): owner, pat"}` |
| **401 / 404** | `{"status":"error","message":"…"}` (GitHub rejected the token / repo not found) |
| **403** | missing/invalid API key (API Gateway) |

```bash
curl -sS -X POST "$REG" -H "Content-Type: application/json" -H "x-api-key: $API_KEY" \
  -d '{"owner":"octocat","repository":"widget","pat":"ghp_x"}'
```

## 5. Update credentials (rotation)

Re-send the `POST` with `"update": true` and the new `pat`.

```bash
curl -sS -X POST "$REG" -H "Content-Type: application/json" -H "x-api-key: $API_KEY" \
  -d '{"owner":"octocat","repository":"widget","pat":"ghp_new","update":true}'
```

## 6. Delete — `DELETE /repositories`

```json
{ "owner": "octocat", "repository": "widget" }
```

Removes the repository's **entry** in the shared secret (others untouched) and
its **metadata** (and the GitHub webhook, best-effort, if one was ever created).

| Status | Body |
| --- | --- |
| **200** | `{"status":"success","message":"Repository deleted successfully.","repository":"octocat/widget"}` |
| **404** | `{"status":"error","message":"repository is not registered"}` |

```bash
curl -sS -X DELETE "$REG" -H "Content-Type: application/json" -H "x-api-key: $API_KEY" \
  -d '{"owner":"octocat","repository":"widget"}'
```

## 7. Lookup (how credentials are retrieved)

The worker never fetches a per-repo secret. It:

1. reads the shared secret `blog-gen/github/repositories`,
2. parses the JSON,
3. indexes by `"<owner>/<name>"` (stored as the repo's `SecretRef` in metadata),
4. uses `.pat` (cloning and opening pull requests).

## 8. Logging

The Lambda logs `owner`, `repository`, `action` (registered/updated/deleted),
`secret_status`, and `outcome` (success/failure) — **never** the PAT or request body.

## 9. IAM & CloudFormation

- **Shared secret** `RepoCredentialsSecret` (`blog-gen/github/repositories`),
  created with `{}` — CloudFormation never rewrites the value on later updates,
  so entries persist.
- **Registration role**: `secretsmanager:GetSecretValue` + `PutSecretValue` on
  the **one** shared secret; `dynamodb:PutItem/UpdateItem/GetItem/DeleteItem`.
  (The old per-repo `CreateSecret`/`…/repos/*` grants are gone.)
- **Worker role**: `secretsmanager:GetSecretValue` on the **one** shared secret only.
- **Registration function**: env `REPO_SECRET_ID`; optional
  `ReservedConcurrentExecutions` via the `RegistrationReservedConcurrency`
  parameter (default 0 = off; set 1 for single-writer, needs quota headroom).
- **API**: `POST` and `DELETE` on `/repositories` (both API-key-required) →
  the registration Lambda, which routes on HTTP method. Output
  `RepoCredentialsSecretArn` is exported for the compute stack.

## 10. Security considerations

- One secret, least-privilege access: registration read-writes it; readers get
  read-only; per-ARN scoping (no wildcards) — see [Security](./security.md).
- The PAT is never returned by the API, logged, or stored in DynamoDB (only the
  `"<owner>/<name>"` reference is).
- Optional reserved concurrency (parameter) gives a single-writer guarantee
  when enabled; the store's retry is the default best-effort safeguard.

## 11. Migration from per-repo secrets

Existing deployments have per-repo secrets under `blog-gen/repos/…`. Move them
into the shared secret with [`scripts/migrate-shared-secret.sh`](../scripts/migrate-shared-secret.sh):

```bash
REGION=us-east-1 scripts/migrate-shared-secret.sh          # dry run (masked preview)
REGION=us-east-1 APPLY=1 scripts/migrate-shared-secret.sh  # write the shared secret
```

It reads both historical layouts (`…/pat` + `…/webhook-secret`, or a single JSON
per repo), merges them into `blog-gen/github/repositories`, and prints how to
delete the old secrets once you've verified the platform still works.
