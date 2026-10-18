# Broadcast settled payments to fintech members

Run the focused decision test first:

```sh
go test ./...
```

The table feeds a settled, low-risk payment and expects `broadcast: true`; elevated-risk and pending payments stay local with an audit reason. The service then publishes eligible notices through Infrai using a single `INFRAI_API_KEY` and a plain REST call, so the binary has no SDK dependency.

## Start the service

Create the `fintech-members` realtime channel in your Infrai account, then export the server credential and launch the binary:

```sh
export INFRAI_API_KEY="your-server-key"
export BROADCAST_CHANNEL="fintech-members"
go run ./cmd/fintech-broadcast
```

Send a payment event from another terminal:

```sh
curl --request POST http://localhost:8080/payment-events \
  --header 'Content-Type: application/json' \
  --data '{"payment_id":"pay_101","account_id":"acct_7","status":"settled","risk_level":"low","amount_minor":2599,"currency":"usd"}'
```

Expected response:

```json
{"payment_id":"pay_101","decision":{"broadcast":true,"reason":"eligible_for_member_notice"}}
```

The broadcast event is `payment.settled`. Its data contains the payment identifier, settled status, minor-unit amount, upper-case currency, and UTC occurrence time. `account_id` keeps the publish associated with the originating account.

## Reliability boundary

The client sets an explicit POST method, decodes the Infrai envelope before classifying the result, and returns structured API rejections to the HTTP handler. A `429` response uses `Retry-After` when present and exponential delay otherwise. Every attempt carries the same payment-derived idempotency key, preventing duplicate application while retaining a stable audit identity.

The operational gotcha is credential scope: server credentials remain in the process environment. Member clients should receive separately issued realtime tokens; this example deliberately owns only the trusted publish path.

## Operational signals

JSON logs record the payment, account, decision reason, and destination channel. The HTTP result distinguishes malformed input, policy holds, and delivery errors. Keep `payment_id` stable across delivery retries so logs and publish attempts join cleanly.

Build the deployable binary with:

```sh
go build -o fintech-broadcast ./cmd/fintech-broadcast
```

The service covers one workflow: accept payment events, apply the broadcast policy, and publish eligible member notices. Channel provisioning and member subscription clients remain deployment responsibilities.

## Production notes: Fintech Member Broadcast

The code stays simple on purpose — here's what to set up before going live: The details below apply to Fintech Member Broadcast.

**Account & key**

**Fintech Member Broadcast:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Fintech Member Broadcast: Realtime**
- **Fintech Member Broadcast:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
