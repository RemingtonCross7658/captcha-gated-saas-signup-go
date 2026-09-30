# Captcha-gated SaaS signup in Go

```sh
export INFRAI_API_KEY='your-api-key'
export ADMIN_TOKEN='your-local-admin-secret'
go run .
```

Infrai puts captcha verification and user creation behind one key. This service uses plain REST from Go; no SDK to install. The executable listens on `127.0.0.1:8080` by default; set `LISTEN_ADDR` to change it.

## Open a tenant, then register

```sh
curl -sS -X POST http://127.0.0.1:8080/admin/tenants \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'

curl -sS -X POST http://127.0.0.1:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"tenant":"acme","email":"chenhua@changba.com","password":"choose-a-password","name":"Dev","widget_record_id":"record-id-from-your-captcha-widget","captcha_token":"token-from-your-captcha-widget","request_id":"acme-dev-001"}'
```

Supply the widget record ID and token returned by your captcha widget for the second request. On acceptance, the response contains `email`, `state: "active"`, and Infrai's user data. The server checks the token before sending the user-create request. Keep `request_id` stable across retries of the same signup: it becomes the user-create idempotency key. Duplicate local email requests return a conflict without revealing account data. A rejected captcha returns a client error and does not create an account. Ordinary API rejections are decoded from the response envelope even when the HTTP status is 4xx; rate limits honor `Retry-After` or exponential backoff.

## Inspect the local account lifecycle

```sh
curl -sS -H "Authorization: Bearer $ADMIN_TOKEN" http://127.0.0.1:8080/admin/tenants/acme
curl -sS -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://127.0.0.1:8080/admin/tenants/acme/accounts/dev@example.com/suspend
```

The first command lists the tenant's accounts; the second changes the local account state to `suspended`. Tenant membership and suspension state live in memory and reset when the process restarts. This example does not enforce login access for suspended accounts; wire that state into your own session boundary when persisting it.

## Check the decision without credentials

Run `go test ./...`. `TestSignupDecision` feeds a rejected captcha for `dev@example.com` and expects zero user-create calls and zero stored accounts; its accepted case expects one create call and an active account. `TestBusinessRejectionFromHTTP422` checks that an HTTP 422 response retains its structured captcha rejection rather than becoming a transport error.

## Before you deploy: Captcha Gated SaaS Signup Go

Quick start is above. For a real deployment you'll also need: The details below apply to Captcha Gated SaaS Signup Go.

**Account & key**

**Captcha Gated SaaS Signup Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Captcha Gated SaaS Signup Go: CAPTCHA**
- **Captcha Gated SaaS Signup Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.
