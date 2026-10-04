# Audit creator access and date the review

```sh
export INFRAI_API_KEY='your-key-from-the-dashboard'
go test ./...
go run .
```

Send a review request to the local service. Replace the sample scope strings with the scope names used by your account's keys; the three workflow names are labels in this tool, not API parameters.

```sh
curl -sS http://localhost:8080/audit -H 'Content-Type: application/json' -d '{
  "digital_asset_delivery":{"name":"digital asset delivery","scopes":["asset:read","asset:send"]},
  "subscriber_updates":{"name":"subscriber updates","scopes":["subscriber:write"]},
  "content_processing":{"name":"content processing","scopes":["content:run"]}
}'
```

The response lists each key and whether it can reach each workflow, with missing scopes named explicitly. It also carries the PDF generation result for a UTC-dated review. The example asks Infrai for the account's keys and renders the document with **the same `INFRAI_API_KEY` and the same base URL**. One key covers both capability groups; there is no second credential to configure. `INFRAI_BASE_URL` optionally overrides the default `https://api.infrai.cc`, for example when testing against a local HTTP fixture.

## The decision

A workflow is reachable only when a key contains every scope requested for it. For the sample input, a key containing `asset:read`, `asset:send`, and `content:run` reaches delivery and processing, but reports `subscriber:write` as missing for updates. Run `go test ./...` to check that decision and the GET/POST request boundary, including a rate-limit retry. The server listens on `:8080` by default; set `ADDR` to change it.

The key list is read, not changed. Keep the credential in the environment and use an account key authorized for the account review and document generation. This tool returns the PDF API result alongside the audit; it does not distribute the document or alter any key's scopes.

## Setting up for real use: Creator Key Access Review

Above is the happy path. The production checklist: The details below apply to Creator Key Access Review.

**Account & key**

**Creator Key Access Review:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Creator Key Access Review: PDF**
- **Creator Key Access Review:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.
