# Source map

This page is the fastest way to find the code behind a command.

## Primary packages

- `cmd/straddle/main.go` — CLI entrypoint; hands off to the Cobra command tree.
- `internal/cli/root.go` — Cobra root command, persistent flags, exit behavior, and shared output rules.
- `internal/cli/straddle_setup.go` — persisted integration type (`account`, `saas`, `marketplace`) and current embedded account helpers.
- `internal/cli/straddle_*.go` — hand-authored analytics, workflow, and reference commands that extend the generated API surface.
- `internal/store/store.go` — SQLite store, migrations, FTS, schema versioning.
- `internal/straddleacct/policy.go` — `Straddle-Account-Id` decision engine.
- `internal/client/` — HTTP client and response handling.
- `internal/config/` — config loading/saving.

## Command groups worth knowing

### Generated API families

These command families mirror Straddle API resources. In general:
- `list` retrieves collections
- `get` retrieves one object by id
- `create` sends a new object to Straddle
- `update` modifies an existing object before it is finalized
- resource-specific verbs such as `cancel`, `hold`, `release`, `resubmit`, `review`, `unmask`, and `reveal` expose lifecycle and access-control operations that only make sense for that resource

#### `accounts*`
Manage platform accounts and onboarding.
- `accounts create` — create a new account for a platform integration.
- `accounts get` — fetch one account by id.
- `accounts list` — list accounts for the current integration.
- `accounts update` — update account details.
- `accounts onboard` / `accounts onboard account` — onboarding flow helpers for account activation.
- `accounts simulate` / `accounts simulate create` — sandbox-only simulation helpers for account-related flows.
- `accounts capability-requests list` / `create` — inspect or create capability request records tied to an account.

#### `bridge*`
Create paykeys and bridge sessions from external bank-verification sources.
- `bridge create` — create a paykey from a Quiltt token.
- `bridge create-bank-account-paykey` — create a paykey directly from routing/account numbers.
- `bridge create-plaid-paykey` — create a paykey from a Plaid token.
- `bridge create-speedchex` — create a paykey from a Speedchex token.
- `bridge create-tan` — create a TAN-related bridge object.
- `bridge create-token` — generate a token for the Bridge widget/session.

#### `charges*`
Create and manage debit attempts against customer bank accounts.
- `charges create` — initiate a charge.
- `charges get` — fetch one charge by id.
- `charges list` — list charges.
- `charges update` — update a charge before processing.
- `charges cancel` / `charges cancel charge` — cancel a charge when it is still in a cancelable state.
- `charges hold` / `charges hold charge` — place a charge on hold.
- `charges release` / `charges release charge` — release a held charge.
- `charges resubmit` / `charges resubmit create` — resubmit a charge.
- `charges unmask` / `charges unmask charges-v1-get` — access an unmasked charge variant.

#### `customers*`
Manage customer identities and review-state operations.
- `customers create` — create a customer and trigger review/risk processing.
- `customers get` — fetch one customer.
- `customers list` — list/search customers.
- `customers update` — update customer details.
- `customers delete` — delete a customer.
- `customers review` / `customers review get-customer` / `update-customer` — review workflow operations.
- `customers refresh-review` / `customers refresh-review update` — refresh review state.
- `customers unmasked` / `customers unmasked get-customer` — access unmasked customer data.

#### `funding-events*`
Inspect settlement and reconciliation records.
- `funding-events create` — sandbox simulation helper for a funding event.
- `funding-events get` — fetch one funding event.
- `funding-events list` — list funding events.

#### `linked-bank-accounts*`
Manage bank accounts linked to an account.
- `linked-bank-accounts create` — create a linked bank account.
- `linked-bank-accounts get` — fetch one linked bank account.
- `linked-bank-accounts list` — list linked bank accounts.
- `linked-bank-accounts update` — update a linked bank account.
- `linked-bank-accounts cancel` / `linked-bank-accounts cancel update` — cancel a linked bank account or change its cancel state.
- `linked-bank-accounts unmask` / `linked-bank-accounts unmask get-linked-bank-account-unmasked` — access an unmasked variant.

#### `organizations*`
Manage organization records.
- `organizations create` — create an organization.
- `organizations get-by-id` — fetch one organization by id.
- `organizations list` — list organizations.

#### `paykeys*`
Manage reusable payment tokens and their lifecycle.
- `paykeys get` — fetch one paykey.
- `paykeys list` — list paykeys.
- `paykeys cancel` / `paykeys cancel update` — cancel or update cancel state.
- `paykeys refresh-balance` / `paykeys refresh-balance update` — refresh balance data.
- `paykeys refresh-review` / `paykeys refresh-review update` — refresh review state.
- `paykeys review` / `paykeys review get` / `paykeys review update` — review workflow operations.
- `paykeys reveal` / `paykeys reveal get` — reveal a paykey when the API allows it.
- `paykeys unblock` / `paykeys unblock update` — unblock a paykey.
- `paykeys unmasked` / `paykeys unmasked get-paykey` — access an unmasked paykey variant.

#### `payouts*`
Create and manage outgoing money movement.
- `payouts create` — initiate a payout.
- `payouts get` — fetch one payout.
- `payouts list` — list payouts.
- `payouts update` — update a payout before processing.
- `payouts cancel` / `payouts cancel payout` — cancel a payout when cancelable.
- `payouts hold` / `payouts hold payout` — place a payout on hold.
- `payouts release` / `payouts release payout` — release a held payout.
- `payouts resubmit` / `payouts resubmit create` — resubmit a payout.
- `payouts unmask` / `payouts unmask payouts-v1-get` — access an unmasked payout variant.

#### `representatives*`
Manage beneficial owners, control persons, and signers for KYC/KYB.
- `representatives create` — create a representative record.
- `representatives get` — fetch one representative.
- `representatives list` — list representatives.
- `representatives update` — update representative details.
- `representatives unmask` / `representatives unmask get` — access an unmasked variant.

### Hand-authored product commands

These are the command families that make this repo more than a direct API wrapper.

- `doctor` — checks config, auth, environment variables, store access, and platform state.
- `sync` — pulls API data into the local SQLite store for offline search and analysis.
- `search` — full-text search over synced data, or API search when the resource supports it.
- `payments` — unified view across charges and payouts for searching/filtering payment activity.
- `reconcile` — match synced payments to funding events and show settled vs outstanding.
- `pipeline` — group synced payments by status and identify cancelable items.
- `returns` — identify failed or reversed payments and rank repeat offenders.
- `review-queue` — show customers and paykeys waiting for review, oldest first.
- `expiring` — show paykeys nearing expiry or blocked-but-unblockable.
- `cashflow` — aggregate money in vs money out over time with daily or weekly buckets.
- `sandbox` — print deterministic sandbox outcomes and test bank details.
- `setup` — set the integration type that controls `Straddle-Account-Id` scoping.
- `use-account` — set or clear the current embedded account for platform calls.
- `sql` — run read-only SQL against the local SQLite database.
- `which` — resolve a natural-language capability query to the best matching command.
- `auth` — manage saved authentication state.
- `profile` — save, load, list, show, and delete local CLI profiles.
- `feedback` — collect or list feedback records.
- `tail` — stream or inspect recent event output.
- `workflow` / `workflow archive` / `workflow status` — channel/workflow helpers.
- `api` / `api discovery` — inspect API capabilities and discovery behavior.
- `import` — import data snapshots into the local store.
- `deliver` — route command output to alternate sinks such as files or webhooks.
- `analytics` — umbrella entrypoint for analytics-related helpers.
- `agent-context` — expose runtime context to agents.
- `channel` and `promoted*` commands — extra workflow/reporting surfaces exposed in the generated tree.

## Supporting directories

- `demo/` — demo recordings and scripts.
- `build/` — build/release-related assets.
- `spec.json` — OpenAPI source spec.
- `manifest.json` — package manifest metadata.
- `.printing-press*.json` — generator and patch metadata.

## How to use this map

If you need to change a command, start with the command's file under `internal/cli/`, then follow its helpers into `internal/store/`, `internal/client/`, or `internal/straddleacct/` as needed.

If you need a fast orientation for a future edit, this page should tell you which file family to inspect first.
