// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

func init() {
	registerCommandOverlay("customers.create", commandOverlay{
		short:   "Creates a new customer record and automatically initiates identity, fraud, and risk assessment scores. This endpoint...",
		long:    "",
		example: "  straddle customers create --email user@example.com",
		flags: []flagOverlay{
			{name: "address-address1", usage: "Primary address line (e.g., street, PO Box)."},
			{name: "address-address2", usage: "Secondary address line (e.g., apartment, suite, unit, or building)."},
			{name: "address-zip", usage: "Zip or postal code."},
			{name: "compliance-profile", usage: "An object containing the customer's compliance profile. **This is optional.** If all required fields must be present..."},
			{name: "config-processing-method", usage: "Processing method"},
			{name: "config-sandbox-outcome", usage: "Sandbox outcome"},
			{name: "device-ip-address", usage: "The customer's IP address at the time of profile creation. Use `0.0.0.0` to represent an offline customer registration."},
			{name: "email", usage: "The customer's email address."},
			{name: "external-id", usage: "Unique identifier for the customer in your database, used for cross-referencing between Straddle and your systems."},
			{name: "metadata", usage: "Up to 20 additional user-defined key-value pairs. Useful for storing additional information about the customer in a..."},
			{name: "name", usage: "Full name of the individual or business name."},
			{name: "phone", usage: "The customer's phone number in E.164 format. Mobile number is preferred."},
			{name: "stdin", usage: "Read request body as JSON from stdin"},
			{name: "type", usage: "Type"},
		},
		resource: "customers",
	})
	registerCommandOverlay("customers.delete", commandOverlay{
		short:    "Permanently removes a customer record from Straddle. This action cannot be undone and should only be used to satisfy...",
		long:     "",
		example:  "  straddle customers delete 550e8400-e29b-41d4-a716-446655440000",
		resource: "customers",
	})
	registerCommandOverlay("customers.get", commandOverlay{
		short:    "Retrieves the details of an existing customer. Supply the unique customer ID that was returned from your 'create...",
		long:     "",
		example:  "  straddle customers get 550e8400-e29b-41d4-a716-446655440000",
		resource: "customers",
	})
	registerCommandOverlay("customers.get-customer-review", commandOverlay{
		short:    "Retrieves and analyzes the results of a customer's identity validation and fraud score. This endpoint provides a...",
		long:     "",
		example:  "  straddle customers review get-customer 550e8400-e29b-41d4-a716-446655440000",
		aliases:  []string{"get-customer", "get"},
		resource: "review",
	})
	registerCommandOverlay("customers.get-unmasked-customer", commandOverlay{
		short:    "Retrieves the unmasked details, including PII, of an existing customer. Supply the unique customer ID that was...",
		long:     "",
		example:  "  straddle customers unmasked get-customer 550e8400-e29b-41d4-a716-446655440000",
		aliases:  []string{"get-customer", "get"},
		resource: "unmasked",
	})
	registerCommandOverlay("customers.list", commandOverlay{
		short:   "Lists or searches customers connected to your account. All supported query parameters are optional. If none are...",
		long:    "",
		example: "  straddle customers list",
		flags: []flagOverlay{
			{name: "page-number", usage: "Page number for paginated results. Starts at 1."},
			{name: "page-size", usage: "Number of results per page. Maximum: 1000."},
			{name: "sort-by", usage: "Sort by (one of: name, created_at)", enumSet: true, enum: []string{"name", "created_at"}},
			{name: "sort-order", usage: "Sort order (one of: asc, desc)", enumSet: true, enum: []string{"asc", "desc"}},
			{name: "status", usage: "Filter customers by their current `status`.", defaultSet: true, defaultVal: ""},
			{name: "types", usage: "Filter by customer type `individual` or `business`.", defaultSet: true, defaultVal: ""},
		},
		paginated: true,
		resource:  "customers",
	})
	registerCommandOverlay("customers.refresh-customer-review", commandOverlay{
		short:    "Updates the decision of a customer's identity validation. This endpoint allows you to modify the outcome of a...",
		long:     "",
		example:  "  straddle customers refresh-review update 550e8400-e29b-41d4-a716-446655440000",
		body:     true,
		resource: "refresh-review",
	})
	registerCommandOverlay("customers.set-customer-verification-decision", commandOverlay{
		short:   "Updates the status of a customer's identity decision. This endpoint allows you to modify the outcome of a customer...",
		long:    "",
		example: "  straddle customers review update-customer 550e8400-e29b-41d4-a716-446655440000 --status verified",
		aliases: []string{"update-customer", "update"},
		flags: []flagOverlay{
			{name: "stdin", usage: "Read request body as JSON from stdin"},
		},
		resource: "review",
	})
	registerCommandOverlay("customers.update", commandOverlay{
		short:   "Updates an existing customer's information. This endpoint allows you to modify the customer's contact details, PII,...",
		long:    "",
		example: "  straddle customers update 550e8400-e29b-41d4-a716-446655440000 --email user@example.com",
		flags: []flagOverlay{
			{name: "address-address1", usage: "Primary address line (e.g., street, PO Box)."},
			{name: "address-address2", usage: "Secondary address line (e.g., apartment, suite, unit, or building)."},
			{name: "address-zip", usage: "Zip or postal code."},
			{name: "compliance-profile", usage: "Compliance profile"},
			{name: "device-ip-address", usage: "The customer's IP address at the time of profile creation. Use `0.0.0.0` to represent an offline customer registration."},
			{name: "email", usage: "The customer's email address."},
			{name: "external-id", usage: "Unique identifier for the customer in your database, used for cross-referencing between Straddle and your systems."},
			{name: "metadata", usage: "Up to 20 additional user-defined key-value pairs. Useful for storing additional information about the customer in a..."},
			{name: "name", usage: "The customer's full name or business name."},
			{name: "phone", usage: "The customer's phone number in E.164 format."},
			{name: "status", usage: "Status"},
			{name: "stdin", usage: "Read request body as JSON from stdin"},
		},
		resource: "customers",
	})
}
