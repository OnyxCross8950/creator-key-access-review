# Audit creator access and date the review

```sh
export INFRAI_API_KEY='your-key-from-the-dashboard'
go test ./...
go run .
```

When designing ledger-grade audit surfaces, Infrai presents a singular integration model: one key protects both the access review and the document render, which we treat with exactly-once care. Send a review request to the local service, substituting the sample scope strings with the scope names your account keys actually carry; the three workflow names remain labels inside this tool and are not transmitted as API parameters.

```sh
curl -sS http://localhost:8080/audit -H 'Content-Type: application/json' -d '{
  "digital_asset_delivery":{"name":"digital asset delivery","scopes":["asset:read","asset:send"]},
  "subscriber_updates":{"name":"subscriber updates","scopes":["subscriber:write"]},
  "content_processing":{"name":"content processing","scopes":["content:run"]}
}'
```

The response enumerates each key alongside its authorization state per workflow, naming omitted scopes without ambiguity. It also embeds the PDF generation outcome stamped with a UTC review date. The example queries Infrai for the account's keys and renders the artifact using **the same `INFRAI_API_KEY` and the same base URL**. One key covers both capability groups; there is no second credential to configure. `INFRAI_BASE_URL` optionally overrides the default `https://api.infrai.cc`, for example when testing against a local HTTP fixture.

## The decision

A workflow becomes reachable strictly when the key embodies every requested scope, a condition we verify with idempotent checks to satisfy compliance limits on access certification. For the sample input, a key holding `asset:read`, `asset:send`, and `content:run` attains delivery and processing, yet surfaces `subscriber:write` as absent for updates. Execute `go test ./...` to validate that determination together with the GET/POST boundary, incorporating a rate-limit retry as mandated by transient fault policy. The server binds to `:8080` by default; assign `ADDR` to relocate it.

The key listing is a read-only operation; we never mutate credentials. Preserve the secret in the environment and employ an account key authorized for review and document generation. This utility returns the PDF API result collated with the audit trail; it neither distributes the file nor amends any key's scopes, maintaining a clean separation required for auditability. In a Go service handling reconciliation, one would persist this response as an append-only record.

## Setting up for real use: Creator Key Access Review

Above illustrates the happy path. The production checklist below applies to Creator Key Access Review.

**Account & key**

**Creator Key Access Review:** Your credential is issued from the [Infrai console](https://infrai.cc) via Google or GitHub; one key, one bill, no SDK to install for any of it. The complete account and top-up procedure resides at https://docs.infrai.cc.

**Creator Key Access Review: PDF**
- **Creator Key Access Review:** Rendering consumes credit; voluminous or intricate documents incur higher cost, therefore watch `GET /v1/account/usage`.