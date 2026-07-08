# Straddle CLI

**Every Straddle API operation, plus a local payments ledger, offline search, and settlement and return analytics no stateless tool can do.**

A full CLI for Straddle's Pay by Bank and Embed APIs that also keeps a local SQLite copy of your charges, payouts, customers, paykeys, and funding events. On top of the synced store it adds reconciliation, a cancel-window payment pipeline, return analysis, and cashflow analytics that the official stateless CLI cannot offer.

Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press); maintained and released from this repo.

## Install

### Homebrew (macOS)

```bash
brew install straddle-build/tap/straddle
```

> Available with the next patch release — the tap's publish credential is being provisioned. Use the shell installer below meanwhile.

### Shell installer (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/straddle-build/cli/main/install.sh | sh
```

Installs the latest release to `~/.local/bin` (override with `STRADDLE_INSTALL_DIR`) after verifying its checksum against the release's `checksums.txt`.

### npm / npx

```bash
npx @straddleio/cli doctor   # run without installing
npm i -g @straddleio/cli     # install the straddle binary globally
```

> Publishing with the next patch release — the npm token is being provisioned. Use the shell installer above meanwhile.

### Pre-built binaries

Download an archive for your platform from the [releases page](https://github.com/straddle-build/cli/releases). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```bash
go install github.com/straddle-build/cli/cmd/straddle@latest
```

### From source

```bash
git clone https://github.com/straddle-build/cli && cd cli && make build   # -> bin/straddle
```

### Agent skill

The repo-root [`SKILL.md`](SKILL.md) teaches coding agents (Claude Code, Codex, Cursor, and friends) how to drive this CLI. Install it with the [`skills`](https://github.com/vercel-labs/skills) CLI:

```bash
npx skills add straddle-build/cli
```

The legacy `npx -y @mvanhorn/printing-press install straddle` installer also still works.

## Authentication

Straddle uses a Bearer JWT API key. Set STRADDLE_API_KEY (or pass --api-key) and choose your environment with --environment sandbox|production; sandbox keys only work against sandbox.straddle.com and production keys only against production.straddle.com. The default environment is sandbox so you never hit live money movement by accident. Platform (Embed) integrators must scope account-specific calls with the Straddle-Account-Id header via --account-id or STRADDLE_ACCOUNT_ID: a SaaS platform sets it on customer, paykey, charge, payout, review, and onboarding calls; a marketplace sets it only on charge and payout (and onboarding) calls, not on customer or paykey calls; a direct account never sets it. Platform ID, Organization ID, and Account ID are three different identifiers, do not interchange them.

Get your API key from the [Straddle dashboard](https://dashboard.straddle.com) (Developer → API Keys); see the [authentication docs](https://docs.straddle.com/api-reference/authentication) for details.

## Quick Start

```bash
# Confirm STRADDLE_API_KEY is set and the chosen environment is reachable before anything else.
straddle doctor


# Pull charges, payouts, customers, paykeys, and funding events into the local store so search and analytics work offline.
straddle sync


# Search across charges and payouts in one call; the unified payments view is the fastest way to see recent activity.
straddle payments --json


# See which synced payments have not yet settled to a funding event.
straddle reconcile --outstanding --json


# Find payments you can still cancel before they reach the locked pending state.
straddle pipeline --cancelable --json

```

## Unique Features

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

## Usage

Run `straddle --help` for the full command reference and flag list.

## Commands

### account-settings

Manage account settings

- **`straddle account-settings <account_id>`** - Get all resolved settings for the specified account, including inherited values from organization, platform, and system defaults.

### accounts

Accounts represent businesses using Straddle through your platform. Each account must complete automated verification before processing payments. Use accounts to manage your users' payment capabilities, track verification status, and control access to features. Accounts can be instantly created in sandbox and require additional verification for production access.

- **`straddle accounts create`** - Creates a new account associated with your Straddle platform integration. This endpoint allows you to set up an account with specified details, including business information and access levels.
- **`straddle accounts get`** - Retrieves the details of an account that has previously been created. Supply the unique account ID that was returned from your previous request, and Straddle will return the corresponding account information.
- **`straddle accounts list`** - Returns a list of accounts associated with your Straddle platform integration. The accounts are returned sorted by creation date, with the most recently created accounts appearing first. This endpoint supports advanced sorting and filtering options.
- **`straddle accounts update`** - Updates an existing account's information. This endpoint allows you to update various account details during onboarding or after the account has been created.

### bridge

Bridge provides a comprehensive suite of tools for connecting customer bank accounts. Use it to generate secure widget sessions for instant account verification, accept tokens from major providers like Plaid and Finicity, or verify accounts directly via our API. Bridge handles all sensitive banking credentials and ensures secure, compliant connections with support for 90% of US bank accounts.

- **`straddle bridge create`** - Creates a new paykey using a Quiltt token as the source. This endpoint allows you to create a secure payment token linked to a bank account authenticated through Quiltt.
- **`straddle bridge create-bank-account-paykey`** - Use Bridge to create a new paykey using a bank routing and account number as the source. This endpoint allows you to create a secure payment token linked to a specific bank account.
- **`straddle bridge create-plaid-paykey`** - Use Bridge to create a new paykey using a Plaid token as the source. This endpoint allows you to create a secure payment token linked to a bank account authenticated through Plaid.
- **`straddle bridge create-speedchex`** - Creates a new paykey using a Speedchex token as the source. This endpoint allows you to create a secure payment token linked to a bank account authenticated through Speedchex.
- **`straddle bridge create-tan`** - Create tan
- **`straddle bridge create-token`** - Use this endpoint to generate a session token for use in the Bridge widget.

### charges

Charges represent attempts to debit money from a customer's bank account using a Paykey. Each charge includes automatic balance verification, real-time fraud screening, and multi-rail optimization and detailed status tracking throughout the payment lifecycle. Use charges to accept bank payments with confidence knowing every transaction is protected.

- **`straddle charges create`** - Use charges to collect money from a customer for the sale of goods or services.
- **`straddle charges get`** - Retrieves the details of an existing charge. Supply the unique charge `id`, and Straddle will return the corresponding charge information.
- **`straddle charges update`** - Change the values of parameters associated with a charge prior to processing. The status of the charge must be `created`, `scheduled`, or `on_hold`.

### customers

Customers represent the end users who send or receive payments through your integration. Each customer undergoes automatic identity verification and fraud screening upon creation. Use customers to track payment history, manage bank account connections, and maintain a secure record of all transactions associated with a user. Customers can be either individuals or businesses with appropriate compliance checks for each type.

- **`straddle customers create`** - Creates a new customer record and automatically initiates identity, fraud, and risk assessment scores. This endpoint allows you to create a customer profile and associate it with paykeys and payments.
- **`straddle customers delete`** - Permanently removes a customer record from Straddle. This action cannot be undone and should only be used to satisfy regulatory requirements or for privacy compliance.
- **`straddle customers get`** - Retrieves the details of an existing customer. Supply the unique customer ID that was returned from your 'create customer' request, and Straddle will return the corresponding customer information.
- **`straddle customers list`** - Lists or searches customers connected to your account. All supported query parameters are optional. If none are provided, the response will include all customers connected to your account. This endpoint supports advanced sorting and filtering options.
- **`straddle customers update`** - Updates an existing customer's information. This endpoint allows you to modify the customer's contact details, PII, and metadata.

### funding-event-payments

Manage funding event payments

- **`straddle funding-event-payments <id>`** - All the payments that made up the funding event

### funding-events

Funding events represent all money movement between Straddle and an Account's external bank accounts. They are automatically generated when charges settle or payouts are initiated. Each event provides detailed tracking of settlement status, fee breakdowns, and reconciliation data across both incoming and outgoing transfers. Use funding events to monitor your platform's entire money movement lifecycle.

- **`straddle funding-events create`** - Simulate a funding event for testing. This endpoint can only be used in the sandbox environment.
- **`straddle funding-events get`** - Retrieves the details of an existing funding event. Supply the unique funding event `id`, and Straddle will return the individual transaction items that make up the funding event.
- **`straddle funding-events list`** - Retrieves a list of funding events for your account. This endpoint supports advanced sorting and filtering options.

### linked-bank-accounts

Linked bank accounts connect your platform users' external bank accounts to Straddle for settlements and payment funding. Each linked account undergoes automated verification and continuous monitoring. Use linked accounts to manage where clients receive deposits, fund payouts, and track settlement preferences.

- **`straddle linked-bank-accounts create`** - Creates a new linked bank account associated with a Straddle account. This endpoint allows you to associate external bank accounts with a Straddle account for various payment operations such as payment deposits, payout withdrawals, and more.
- **`straddle linked-bank-accounts get`** - Retrieves the details of a linked bank account that has previously been created. Supply the unique linked bank account `id`, and Straddle will return the corresponding information. The response includes masked account details for security purposes.
- **`straddle linked-bank-accounts list`** - Returns a list of bank accounts associated with a specific Straddle account. The linked bank accounts are returned sorted by creation date, with the most recently created appearing first. This endpoint supports pagination to handle accounts with multiple linked bank accounts.
- **`straddle linked-bank-accounts update`** - Updates an existing linked bank account's information. This can be used to update account details during onboarding or to update metadata associated with the linked account. The linked bank account must be in 'created' or 'onboarding' status.

### organizations

Organizations are a powerful feature in Straddle that allow you to manage multiple accounts under a single umbrella. This hierarchical structure is particularly useful for businesses with complex operations, multiple departments, or legally related entities.

- **`straddle organizations create`** - Creates a new organization related to your Straddle integration. Organizations can be used to group related accounts and manage permissions across multiple users.
- **`straddle organizations get-by-id`** - Retrieves the details of an Organization that has previously been created. Supply the unique organization ID that was returned from your previous request, and Straddle will return the corresponding organization information.
- **`straddle organizations list`** - Retrieves a list of organizations associated with your Straddle integration. The organizations are returned sorted by creation date, with the most recently created organizations appearing first. This endpoint supports advanced sorting and filtering options to help you find specific organizations.

### paykeys

Paykeys are secure tokens that link verified customer identities to their bank accounts. Each Paykey includes built-in balance checking, fraud detection through LSTM machine learning models, and can be reused for subscriptions and recurring payments without storing sensitive data. Paykeys eliminate fraud by ensuring the person initiating payment owns the funding account.

- **`straddle paykeys get`** - Retrieves the details of an existing paykey. Supply the unique paykey `id` and Straddle will return the corresponding paykey record , including the `paykey` token value and masked bank account details.
- **`straddle paykeys list`** - Returns a list of paykeys associated with a Straddle account. This endpoint supports advanced sorting and filtering options.

### payments

Payments provide endpoints to filter both Charges and Payouts with multiple different parameters.

- **`straddle payments`** - Search for payments, including `charges` and `payouts`, using a variety of criteria. This endpoint supports advanced sorting and filtering options.

### payouts

Payouts represent transfers from Straddle to customer bank accounts. Create payouts to handle disbursements, process refunds, or manage marketplace settlements. Use payouts to send money quickly and securely with the most cost-effective rail automatically selected.

- **`straddle payouts create`** - Use payouts to send money to your customers.
- **`straddle payouts get`** - Retrieves the details of an existing payout. Supply the unique payout `id` to retrieve the corresponding payout information.
- **`straddle payouts update`** - Update the details of a payout prior to processing. The status of the payout must be `created`, `scheduled`, or `on_hold`.

### reports

Manage reports

- **`straddle reports`** - Create

### representatives

Representatives are individuals who have legal authority or significant responsibility within a business entity associated with a Straddle account. Each representative undergoes automated verification as part of KYC/KYB compliance. Use representatives to collect and verify beneficial owners, control persons, and authorized signers required for account onboarding. Representatives also determine who can legally operate the account and make important changes.

- **`straddle representatives create`** - Creates a new representative associated with an account. Representatives are individuals who have legal authority or significant responsibility within the business.
- **`straddle representatives get`** - Retrieves the details of an existing representative. Supply the unique representative ID, and Straddle will return the corresponding representative information.
- **`straddle representatives list`** - Returns a list of representatives associated with a specific account or organization. The representatives are returned sorted by creation date, with the most recently created representatives appearing first. This endpoint supports advanced sorting and filtering options.
- **`straddle representatives update`** - Updates an existing representative's information. This can be used to update personal details, contact information, or the relationship to the account or organization.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
straddle accounts list

# JSON for scripting and agents
straddle accounts list --json

# Filter to specific fields
straddle accounts list --json --select id,name,status

# Dry run — show the request without sending
straddle accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
straddle accounts list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `STRADDLE_ENVIRONMENT` resolves `{environment}`

Base URL: `https://{environment}.straddle.com`

## Health Check

```bash
straddle doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/straddle/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `STRADDLE_ENVIRONMENT` | endpoint | Yes |  |
| `STRADDLE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `straddle doctor` to check credentials
- Verify the environment variable is set: `echo $STRADDLE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Set STRADDLE_API_KEY to a key for the environment you target; sandbox keys do not work against production and vice versa.
- **Calls hit the wrong environment** — Pass --environment sandbox or --environment production (default is sandbox); the base URL switches between sandbox.straddle.com and production.straddle.com.
- **A charge cannot be cancelled or held** — Once a payment reaches pending it is locked; run pipeline --cancelable to see which payments are still in created/scheduled/on_hold and can be acted on.
- **Charges fail with an expired paykey** — Run expiring to list paykeys near expires_at, then refresh or re-bridge the bank account before retrying.
- **search or reconcile returns nothing** — Run sync first; the local store is empty until you populate it.
- **Platform calls return the wrong account's data or 403** — Set --account-id (or STRADDLE_ACCOUNT_ID) to the embedded account: SaaS platforms scope customer/paykey/charge/payout/review calls, marketplaces scope only charge/payout calls; direct accounts omit it.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**straddle-cli**](https://github.com/straddleio/straddle-cli) — Go
- [**straddle-go**](https://github.com/straddleio/straddle-go) — Go
- [**straddle-node**](https://github.com/straddleio/straddle-node) — TypeScript
- [**straddle-python**](https://github.com/straddleio/straddle-python) — Python

