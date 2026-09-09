// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

func init() {
	registerCommandOverlay("paykeys.cancel", commandOverlay{
		short:   "Update",
		long:    "",
		example: "  straddle paykeys cancel update 550e8400-e29b-41d4-a716-446655440000",
		flags: []flagOverlay{
			{name: "reason", usage: "Reason"},
			{name: "stdin", usage: "Read request body as JSON from stdin"},
		},
		resource: "cancel",
	})
	registerCommandOverlay("paykeys.get", commandOverlay{
		short:    "Retrieves the details of an existing paykey. Supply the unique paykey `id` and Straddle will return the...",
		long:     "",
		example:  "  straddle paykeys get 550e8400-e29b-41d4-a716-446655440000",
		resource: "paykeys",
	})
	registerCommandOverlay("paykeys.get-paykey-review", commandOverlay{
		short:    "Get additional details about a paykey.",
		long:     "",
		example:  "  straddle paykeys review get 550e8400-e29b-41d4-a716-446655440000",
		resource: "review",
	})
	registerCommandOverlay("paykeys.get-unmasked-paykey", commandOverlay{
		short:    "Retrieves the unmasked details of an existing paykey. Supply the unique paykey `id` and Straddle will return the...",
		long:     "",
		example:  "  straddle paykeys unmasked get-paykey 550e8400-e29b-41d4-a716-446655440000",
		aliases:  []string{"get-paykey", "get"},
		resource: "unmasked",
	})
	registerCommandOverlay("paykeys.list", commandOverlay{
		short:   "Returns a list of paykeys associated with a Straddle account. This endpoint supports advanced sorting and filtering...",
		long:    "",
		example: "  straddle paykeys list",
		flags: []flagOverlay{
			{name: "page-number", usage: "Page number for paginated results. Starts at 1."},
			{name: "page-size", usage: "Number of results per page. Maximum: 1000."},
			{name: "sort-by", usage: "Sort by (one of: institution_name, expires_at, created_at)", enumSet: true, enum: []string{"institution_name", "expires_at", "created_at"}},
			{name: "sort-order", usage: "Sort order (one of: asc, desc)", enumSet: true, enum: []string{"asc", "desc"}},
			{name: "source", usage: "Filter paykeys by their source.", defaultSet: true, defaultVal: ""},
			{name: "status", usage: "Filter paykeys by their current status.", defaultSet: true, defaultVal: ""},
			{name: "unblock-eligible", usage: "Filter paykeys by unblock eligibility. When true, returns only blocked paykeys eligible for client-initiated..."},
		},
		paginated: true,
		resource:  "paykeys",
	})
	registerCommandOverlay("paykeys.refresh-paykey-balance", commandOverlay{
		short:    "Updates the balance of a paykey. This endpoint allows you to refresh the balance of a paykey.",
		long:     "",
		example:  "  straddle paykeys refresh-balance update 550e8400-e29b-41d4-a716-446655440000",
		body:     true,
		resource: "refresh-balance",
	})
	registerCommandOverlay("paykeys.refresh-paykey-review", commandOverlay{
		short:    "Updates the decision of a paykey's review validation. This endpoint allows you to refresh the outcome of a paykey's...",
		long:     "",
		example:  "  straddle paykeys refresh-review update 550e8400-e29b-41d4-a716-446655440000",
		body:     true,
		resource: "refresh-review",
	})
	registerCommandOverlay("paykeys.reveal", commandOverlay{
		short:    "Retrieves the details of a paykey that has previously been created. Supply the unique paykey ID that was returned...",
		long:     "",
		example:  "  straddle paykeys reveal get 550e8400-e29b-41d4-a716-446655440000",
		resource: "reveal",
	})
	registerCommandOverlay("paykeys.set-paykey-verification-decision", commandOverlay{
		short:   "Update the status of a paykey when in review status",
		long:    "",
		example: "  straddle paykeys review update 550e8400-e29b-41d4-a716-446655440000 --status active",
		flags: []flagOverlay{
			{name: "status", usage: "Status (one of: active, rejected)", enumSet: true, enum: []string{"active", "rejected"}},
			{name: "stdin", usage: "Read request body as JSON from stdin"},
		},
		resource: "review",
	})
	registerCommandOverlay("paykeys.unblock-paykey", commandOverlay{
		short:   "Unblocks a paykey that was previously blocked due to an R29 return code. Only paykeys blocked with R29 returns that...",
		long:    "",
		example: "  straddle paykeys unblock update 550e8400-e29b-41d4-a716-446655440000",
		flags: []flagOverlay{
			{name: "stdin", usage: "Read request body as JSON from stdin"},
		},
		resource: "unblock",
	})
}
