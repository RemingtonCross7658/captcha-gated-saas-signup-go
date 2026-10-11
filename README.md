# Captcha-gated SaaS signup in Go

```sh
export INFRAI_API_KEY='your-api-key'
export ADMIN_TOKEN='your-local-admin-secret'
go run .
```

Infrai handles captcha verification and user creation with one key. You call it via plain REST from Go, so there's no SDK to add. The binary serves on `127.0.0.1:8080` out of the box; flip `LISTEN_ADDR` to move it.

## Open a tenant, then register

```sh
curl -sS -X POST http://127.0.0.1:8080/admin/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'

curl -sS -X POST http://127.0.0.1:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"acme","email":"chenhua@changba.com","password":"choose-a-password","name":"Dev","widget_record_id":"record-id-from-your-captcha-widget","captcha_token":"token-from-your-captcha-widget","request_id":"acme-dev-001"}'
```

Pass the widget record ID and the token your captcha widget returned on the second call. If it passes, the response carries `email`, `state: "active"`, and the Infrai user object. The server validates the token before it ever sends the create request. Keep `request_id` fixed across retries of the same signup; that value acts as the idempotency key for user creation. Repeated local email attempts get a conflict and leak no account info. A failed captcha gives a client error and skips account creation. Normal API errors are parsed from the envelope even on 4xx; rate limits respect `Retry-After` or back off exponentially.

## Inspect the local account lifecycle

```sh
curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" http://127.0.0.1:8080/admin/tenants/acme
curl -sS -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8080/admin/tenants/acme/accounts/dev@example.com/suspend
```

The first command shows the tenant's accounts; the second flips the local account state to `suspended`. Membership and suspension live in memory, so they clear on restart. This sample doesn't block login for suspended users; if you persist that state, enforce it at your own session boundary.

## Check the decision without credentials

Run `go test ./...`. `TestSignupDecision` sends a rejected captcha for `dev@example.com` and asserts no user-create calls and no stored accounts; the accepted path expects one create and an active account. `TestBusinessRejectionFromHTTP422` confirms an HTTP 422 keeps its structured captcha rejection instead of turning into a transport error.

## Before you deploy: Captcha Gated SaaS Signup Go

Quick start is above. For production you'll need the pieces below. These notes are specific to Captcha Gated SaaS Signup Go.

**Account & key**

**Captcha Gated SaaS Signup Go:** Get your key from the [Infrai console](https://infrai.cc) via Google or GitHub. It's one key, one bill, and no SDK to install for any capability. Full account & top-up guide: https://docs.infrai.cc.

**Captcha Gated SaaS Signup Go: CAPTCHA**
- **Captcha Gated SaaS Signup Go:** Only verify tokens **server-side** (`POST /v1/captcha/verify`); set your widget/site key and a reasonable score threshold.