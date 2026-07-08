# Domain map

## Core business areas

This repository is organized around Straddle's payment and platform domains.

### Payments

The primary money-moving resources are:

- charges
- payouts
- funding events
- payments (a unified search view across charges and payouts)

The CLI includes commands for creating, getting, listing, updating, canceling, holding, releasing, and resubmitting some of these resources. It also adds local analytics such as `reconcile`, `pipeline`, `cashflow`, and `returns`.

### Identity and onboarding

The identity/compliance side of the product centers on:

- customers
- paykeys
- representatives
- account onboarding
- review queues
- blocked or expiring paykeys
- linked bank accounts

These concepts show up both in the API command surface and in the hand-authored workflow commands such as `review-queue`, `expiring`, `setup`, and `use-account`.

### Platform and account hierarchy

The repo distinguishes among:

- platform accounts
- organizations
- embedded accounts
- direct accounts
- SaaS integrations
- marketplace integrations

This is not just API shape; it drives whether `Straddle-Account-Id` is allowed, required, or forbidden on a given request.

## The important business rule: account scoping

`internal/straddleacct/policy.go` encodes the rule set for when platform requests must carry `Straddle-Account-Id`.

High-level behavior:

- `account` integrations never send the header.
- `saas` integrations require account scoping for selected create operations and allow it for broader customer-owned resources.
- `marketplace` integrations treat customer-owned resources differently and require account scoping for some money-moving create calls.
- account-management operations like organizations and representatives do not use the header; they carry the account in the body or path instead.

This logic is centralized so every command path applies the same rules.

## Sandbox and testing

The repo includes explicit sandbox/testing workflows. There are commands and docs for deterministic sandbox outcomes, plus demo assets under `demo/`.

## When changing domain behavior

If you change any of the following, inspect the policy layer and command wiring together:

- account scoping
- customer/paykey visibility
- payment lifecycle actions such as cancel/hold/release/resubmit
- review or unblock flows
- anything that affects human output versus agent output

## Relevant source files

- `internal/straddleacct/policy.go`
- `internal/cli/straddle_setup.go`
- `internal/cli/charges*.go`
- `internal/cli/payouts*.go`
- `internal/cli/customers*.go`
- `internal/cli/paykeys*.go`
- `internal/cli/representatives*.go`
- `internal/cli/funding-events*.go`
