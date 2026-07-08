---
name: straddle
description: "Every Straddle API operation, plus a local payments ledger, offline search, and settlement and return analytics no... Trigger phrases: `reconcile straddle payments`, `list straddle charges`, `check a straddle paykey`, `straddle review queue`, `straddle settlement report`, `use straddle`, `run straddle`."
author: "hello-keith"
license: "Apache-2.0"
argument-hint: "<command> [args] | install"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - straddle
---

# Straddle CLI

## Prerequisites: Install the CLI

This skill drives the `straddle` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first (in preference order):

1. Homebrew (macOS): `brew install straddle-build/tap/straddle`
2. Shell installer (macOS/Linux): `curl -fsSL https://raw.githubusercontent.com/straddle-build/cli/main/install.sh | sh`
3. npm: `npm i -g @straddleio/cli` (or run ad hoc via `npx @straddleio/cli <command>`)

Verify: `straddle --version`

If `--version` reports "command not found" after install, the install directory is not on `$PATH` (the shell installer uses `~/.local/bin`). Add it to `$PATH` and re-verify. Do not proceed with skill commands until verification succeeds.

A full CLI for Straddle's Pay by Bank and Embed APIs that also keeps a local SQLite copy of your charges, payouts, customers, paykeys, and funding events. On top of the synced store it adds reconciliation, a cancel-window payment pipeline, return analysis, and cashflow analytics that the official stateless CLI cannot offer.

## When to Use This CLI

Use this CLI when you operate or debug a Straddle integration from the terminal or as an agent: checking payment status, reconciling settlement, triaging the KYC review queue, hunting ACH returns, or scripting sandbox test scenarios. It is the right choice when you want offline querying and cross-payment analytics rather than one-off API calls.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Settlement & cashflow
- **`reconcile`** — Match synced charges and payouts to their funding events locally, showing what has settled to your account and what is still outstanding.

  _Reach for this to answer 'which charges funded this deposit' or 'what is still unsettled' without paging the API per payment._

  ```bash
  straddle reconcile --outstanding --json
  ```
- **`cashflow`** — Aggregate synced charge volume in versus payout volume out over a date window, including zero-activity days, with net flow per day or week.

  _Use to see money in versus money out at a glance, including the days nothing moved, without summing payments by hand._

  ```bash
  straddle cashflow --days 30 --json
  ```

### Payment lifecycle control
- **`pipeline`** — Group synced charges and payouts by lifecycle status and flag which are still cancelable (created/scheduled/on_hold) versus locked once they reach pending.

  _Use before a cutoff to find every payment you can still stop, since pending and later states cannot be cancelled._

  ```bash
  straddle pipeline --cancelable --json
  ```
- **`returns`** — Surface failed and reversed payments with their ACH reason codes and rank repeat-offender paykeys and customers from the local store.

  _Use to spot accounts that keep returning (R01 NSF, R02 closed, R05 dispute) so you can block or re-verify them._

  ```bash
  straddle returns --days 30 --repeat-offenders --json
  ```

### Risk & identity ops
- **`review-queue`** — List customers and paykeys sitting in review status, oldest first, with age-in-queue so the KYC backlog is triageable.

  _Use to clear the identity backlog: these are the items blocking downstream charges and payouts from releasing._

  ```bash
  straddle review-queue --json
  ```
- **`expiring`** — List paykeys approaching their expires_at and blocked paykeys that are unblock-eligible, so payments do not fail on stale tokens.

  _Use to find paykeys to refresh or unblock before recurring charges fail against an expired or blocked token._

  ```bash
  straddle expiring --days 14 --json
  ```

### Sandbox testing
- **`sandbox`** — Print the deterministic sandbox_outcome values for customers, paykeys, charges, and payouts plus the sandbox test bank values so test scenarios are scriptable.

  _Use when writing sandbox tests to pick the exact sandbox_outcome (paid, failed_insufficient_funds, reversed_customer_dispute) that triggers the state you want._

  ```bash
  straddle sandbox outcomes --json
  ```

## Command Reference

**account-settings** — Manage account settings

- `straddle account-settings <account_id>` — Get all resolved settings for the specified account, including inherited values from organization, platform, and...

**accounts** — Accounts represent businesses using Straddle through your platform. Each account must complete automated verification before processing payments. Use accounts to manage your users' payment capabilities, track verification status, and control access to features. Accounts can be instantly created in sandbox and require additional verification for production access.

- `straddle accounts create` — Creates a new account associated with your Straddle platform integration. This endpoint allows you to set up an...
- `straddle accounts get` — Retrieves the details of an account that has previously been created. Supply the unique account ID that was returned...
- `straddle accounts list` — Returns a list of accounts associated with your Straddle platform integration. The accounts are returned sorted by...
- `straddle accounts update` — Updates an existing account's information. This endpoint allows you to update various account details during...

**bridge** — Bridge provides a comprehensive suite of tools for connecting customer bank accounts. Use it to generate secure widget sessions for instant account verification, accept tokens from major providers like Plaid and Finicity, or verify accounts directly via our API. Bridge handles all sensitive banking credentials and ensures secure, compliant connections with support for 90% of US bank accounts.

- `straddle bridge create` — Creates a new paykey using a Quiltt token as the source. This endpoint allows you to create a secure payment token...
- `straddle bridge create-bank-account-paykey` — Use Bridge to create a new paykey using a bank routing and account number as the source. This endpoint allows you to...
- `straddle bridge create-plaid-paykey` — Use Bridge to create a new paykey using a Plaid token as the source. This endpoint allows you to create a secure...
- `straddle bridge create-speedchex` — Creates a new paykey using a Speedchex token as the source. This endpoint allows you to create a secure payment...
- `straddle bridge create-tan` — Create tan
- `straddle bridge create-token` — Use this endpoint to generate a session token for use in the Bridge widget.

**charges** — Charges represent attempts to debit money from a customer's bank account using a Paykey. Each charge includes automatic balance verification, real-time fraud screening, and multi-rail optimization and detailed status tracking throughout the payment lifecycle. Use charges to accept bank payments with confidence knowing every transaction is protected.

- `straddle charges create` — Use charges to collect money from a customer for the sale of goods or services.
- `straddle charges get` — Retrieves the details of an existing charge. Supply the unique charge `id`, and Straddle will return the...
- `straddle charges update` — Change the values of parameters associated with a charge prior to processing. The status of the charge must be...

**customers** — Customers represent the end users who send or receive payments through your integration. Each customer undergoes automatic identity verification and fraud screening upon creation. Use customers to track payment history, manage bank account connections, and maintain a secure record of all transactions associated with a user. Customers can be either individuals or businesses with appropriate compliance checks for each type.

- `straddle customers create` — Creates a new customer record and automatically initiates identity, fraud, and risk assessment scores. This endpoint...
- `straddle customers delete` — Permanently removes a customer record from Straddle. This action cannot be undone and should only be used to satisfy...
- `straddle customers get` — Retrieves the details of an existing customer. Supply the unique customer ID that was returned from your 'create...
- `straddle customers list` — Lists or searches customers connected to your account. All supported query parameters are optional. If none are...
- `straddle customers update` — Updates an existing customer's information. This endpoint allows you to modify the customer's contact details, PII,...

**funding-event-payments** — Manage funding event payments

- `straddle funding-event-payments <id>` — All the payments that made up the funding event

**funding-events** — Funding events represent all money movement between Straddle and an Account's external bank accounts. They are automatically generated when charges settle or payouts are initiated. Each event provides detailed tracking of settlement status, fee breakdowns, and reconciliation data across both incoming and outgoing transfers. Use funding events to monitor your platform's entire money movement lifecycle.

- `straddle funding-events create` — Simulate a funding event for testing. This endpoint can only be used in the sandbox environment.
- `straddle funding-events get` — Retrieves the details of an existing funding event. Supply the unique funding event `id`, and Straddle will return...
- `straddle funding-events list` — Retrieves a list of funding events for your account. This endpoint supports advanced sorting and filtering options.

**linked-bank-accounts** — Linked bank accounts connect your platform users' external bank accounts to Straddle for settlements and payment funding. Each linked account undergoes automated verification and continuous monitoring. Use linked accounts to manage where clients receive deposits, fund payouts, and track settlement preferences.

- `straddle linked-bank-accounts create` — Creates a new linked bank account associated with a Straddle account. This endpoint allows you to associate external...
- `straddle linked-bank-accounts get` — Retrieves the details of a linked bank account that has previously been created. Supply the unique linked bank...
- `straddle linked-bank-accounts list` — Returns a list of bank accounts associated with a specific Straddle account. The linked bank accounts are returned...
- `straddle linked-bank-accounts update` — Updates an existing linked bank account's information. This can be used to update account details during onboarding...

**organizations** — Organizations are a powerful feature in Straddle that allow you to manage multiple accounts under a single umbrella. This hierarchical structure is particularly useful for businesses with complex operations, multiple departments, or legally related entities.

- `straddle organizations create` — Creates a new organization related to your Straddle integration. Organizations can be used to group related accounts...
- `straddle organizations get-by-id` — Retrieves the details of an Organization that has previously been created. Supply the unique organization ID that...
- `straddle organizations list` — Retrieves a list of organizations associated with your Straddle integration. The organizations are returned sorted...

**paykeys** — Paykeys are secure tokens that link verified customer identities to their bank accounts. Each Paykey includes built-in balance checking, fraud detection through LSTM machine learning models, and can be reused for subscriptions and recurring payments without storing sensitive data. Paykeys eliminate fraud by ensuring the person initiating payment owns the funding account.

- `straddle paykeys get` — Retrieves the details of an existing paykey. Supply the unique paykey `id` and Straddle will return the...
- `straddle paykeys list` — Returns a list of paykeys associated with a Straddle account. This endpoint supports advanced sorting and filtering...

**payments** — Payments provide endpoints to filter both Charges and Payouts with multiple different parameters.

- `straddle payments` — Search for payments, including `charges` and `payouts`, using a variety of criteria. This endpoint supports advanced...

**payouts** — Payouts represent transfers from Straddle to customer bank accounts. Create payouts to handle disbursements, process refunds, or manage marketplace settlements. Use payouts to send money quickly and securely with the most cost-effective rail automatically selected.

- `straddle payouts create` — Use payouts to send money to your customers.
- `straddle payouts get` — Retrieves the details of an existing payout. Supply the unique payout `id` to retrieve the corresponding payout...
- `straddle payouts update` — Update the details of a payout prior to processing. The status of the payout must be `created`, `scheduled`, or...

**reports** — Manage reports

- `straddle reports` — Create

**representatives** — Representatives are individuals who have legal authority or significant responsibility within a business entity associated with a Straddle account. Each representative undergoes automated verification as part of KYC/KYB compliance. Use representatives to collect and verify beneficial owners, control persons, and authorized signers required for account onboarding. Representatives also determine who can legally operate the account and make important changes.

- `straddle representatives create` — Creates a new representative associated with an account. Representatives are individuals who have legal authority or...
- `straddle representatives get` — Retrieves the details of an existing representative. Supply the unique representative ID, and Straddle will return...
- `straddle representatives list` — Returns a list of representatives associated with a specific account or organization. The representatives are...
- `straddle representatives update` — Updates an existing representative's information. This can be used to update personal details, contact information,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
straddle which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Reconcile a deposit

```bash
straddle reconcile --funding-event fe_123 --json
```

List the synced charges and payouts that rolled into one funding event so you can tie a bank deposit back to its payments.

### Find cancelable payments before a cutoff

```bash
straddle pipeline --cancelable --json --select id,type,status,amount
```

Narrow the pipeline to only payments still in a modifiable state and project just the fields you need.

### Rank repeat ACH returners

```bash
straddle returns --days 90 --repeat-offenders --json
```

Group failed and reversed payments by paykey to find accounts that keep bouncing.

### Clear the identity backlog

```bash
straddle review-queue --json
```

See every customer and paykey waiting on a review decision, oldest first.

### Pick a sandbox outcome

```bash
straddle sandbox outcomes --json
```

Look up the exact sandbox_outcome value to pass on a create call to force paid, failed, or reversed in tests.

## Auth Setup

Straddle uses a Bearer JWT API key. Set STRADDLE_API_KEY (or pass --api-key) and choose your environment with --environment sandbox|production; sandbox keys only work against sandbox.straddle.com and production keys only against production.straddle.com. The default environment is sandbox so you never hit live money movement by accident. Platform (Embed) integrators declare an integration type once and the CLI then scopes calls to the right embedded account automatically — see **Platform scoping (Embed)** below. Platform ID, Organization ID, and Account ID are three different identifiers, do not interchange them.

Run `straddle doctor` to verify setup.

## Platform scoping (Embed)

If you build on Embed, declare your integration type once; the CLI then sends the `Straddle-Account-Id` header only where it belongs and refuses to misattribute a payment.

1. Set your integration type:
   ```bash
   straddle setup --type account|saas|marketplace
   ```
   - `account` — a single business acting for itself; the header is never sent.
   - `saas` — your embedded clients own their customers; calls are scoped to a client account.
   - `marketplace` — you own the customer relationship; only payments are scoped to a seller account.

2. Pick the acting embedded account (sticky until you change it):
   ```bash
   straddle use-account acct_01h...   # set the current account
   straddle use-account               # show current account + integration type
   straddle use-account --clear       # unset it
   ```

3. Override per call with `--account acct_...` on any command; it beats the sticky value for that one call.

The CLI applies the header for you per this model:

| Operation | account | saas | marketplace |
|---|---|---|---|
| charges / payouts (create) | — | required | required |
| customers / paykeys / bridge | — | scoped | platform owns (no header) |
| funding events / payments | — | scoped | scoped |
| accounts / orgs / reps / linked banks / onboarding | — | — | — |

A `saas` or `marketplace` charge or payout with no account set fails fast with a clear error rather than creating a misattributed payment. Onboarding calls carry the account in the body or path (`representatives create --account-id`, `accounts onboard <account_id>`), so they never use the header.

**Agents:** call the `use-account` tool once to set the acting account; every later endpoint call is scoped to it automatically.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  straddle accounts list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
straddle feedback "the --since flag is inclusive but docs say exclusive"
straddle feedback --stdin < notes.txt
straddle feedback list --json --limit 10
```

Entries are stored locally at `~/.straddle/feedback.jsonl`. They are never POSTed unless `STRADDLE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STRADDLE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
straddle profile save briefing --json
straddle --profile briefing accounts list
straddle profile list --json
straddle profile show briefing
straddle profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `straddle --help` output
2. **Starts with `install`** → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## Direct Use

1. Check if installed: `which straddle`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   straddle <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `straddle <command> --help`.
