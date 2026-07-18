// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newAccountsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyBusinessProfileAddressCity string
	var bodyBusinessProfileAddressCountry string
	var bodyBusinessProfileAddressLine1 string
	var bodyBusinessProfileAddressLine2 string
	var bodyBusinessProfileAddressPostalCode string
	var bodyBusinessProfileAddressState string
	var bodyBusinessProfileDescription string
	var bodyBusinessProfileIndustryCategory string
	var bodyBusinessProfileIndustryMcc string
	var bodyBusinessProfileIndustrySector string
	var bodyBusinessProfileLegalName string
	var bodyBusinessProfileName string
	var bodyBusinessProfilePhone string
	var bodyBusinessProfileSupportChannelsEmail string
	var bodyBusinessProfileSupportChannelsPhone string
	var bodyBusinessProfileSupportChannelsUrl string
	var bodyBusinessProfileTaxId string
	var bodyBusinessProfileUseCase string
	var bodyBusinessProfileWebsite string
	var bodyExternalId string
	var bodyMetadata string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <account_id>",
		Short:       "Updates an existing account's information. This endpoint allows you to update various account details during...",
		Example:     "  straddle accounts update 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"straddle:endpoint": "accounts.update", "straddle:method": "PUT", "straddle:path": "/v1/accounts/{account_id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if !stdinBody {
				if !cmd.Flags().Changed("business-profile-address-city") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-address-city")
				}
				if !cmd.Flags().Changed("business-profile-address-line1") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-address-line1")
				}
				if !cmd.Flags().Changed("business-profile-address-postal-code") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-address-postal-code")
				}
				if !cmd.Flags().Changed("business-profile-address-state") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-address-state")
				}
				if !cmd.Flags().Changed("business-profile-name") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-name")
				}
				if !cmd.Flags().Changed("business-profile-website") && !flags.dryRun {
					return fmt.Errorf("required flag \"%s\" not set", "business-profile-website")
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/v1/accounts/{account_id}"
			path = replacePathParam(path, "account_id", args[0])
			params := map[string]string{}
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				body = map[string]any{}
				{
					nestedBusinessProfile := map[string]any{}
					{
						nestedBusinessProfileAddress := map[string]any{}
						if bodyBusinessProfileAddressCity != "" {
							nestedBusinessProfileAddress["city"] = bodyBusinessProfileAddressCity
						}
						if bodyBusinessProfileAddressCountry != "" {
							nestedBusinessProfileAddress["country"] = bodyBusinessProfileAddressCountry
						}
						if bodyBusinessProfileAddressLine1 != "" {
							nestedBusinessProfileAddress["line1"] = bodyBusinessProfileAddressLine1
							nestedBusinessProfileAddress["address1"] = bodyBusinessProfileAddressLine1
						}
						if bodyBusinessProfileAddressLine2 != "" {
							nestedBusinessProfileAddress["line2"] = bodyBusinessProfileAddressLine2
						}
						if bodyBusinessProfileAddressPostalCode != "" {
							nestedBusinessProfileAddress["postal_code"] = bodyBusinessProfileAddressPostalCode
							nestedBusinessProfileAddress["zip"] = bodyBusinessProfileAddressPostalCode
						}
						if bodyBusinessProfileAddressState != "" {
							nestedBusinessProfileAddress["state"] = bodyBusinessProfileAddressState
						}
						if len(nestedBusinessProfileAddress) > 0 {
							nestedBusinessProfile["address"] = nestedBusinessProfileAddress
						}
					}
					if bodyBusinessProfileDescription != "" {
						nestedBusinessProfile["description"] = bodyBusinessProfileDescription
					}
					{
						nestedBusinessProfileIndustry := map[string]any{}
						if bodyBusinessProfileIndustryCategory != "" {
							nestedBusinessProfileIndustry["category"] = bodyBusinessProfileIndustryCategory
						}
						if bodyBusinessProfileIndustryMcc != "" {
							nestedBusinessProfileIndustry["mcc"] = bodyBusinessProfileIndustryMcc
						}
						if bodyBusinessProfileIndustrySector != "" {
							nestedBusinessProfileIndustry["sector"] = bodyBusinessProfileIndustrySector
						}
						if len(nestedBusinessProfileIndustry) > 0 {
							nestedBusinessProfile["industry"] = nestedBusinessProfileIndustry
						}
					}
					if bodyBusinessProfileLegalName != "" {
						nestedBusinessProfile["legal_name"] = bodyBusinessProfileLegalName
					}
					if bodyBusinessProfileName != "" {
						nestedBusinessProfile["name"] = bodyBusinessProfileName
					}
					if bodyBusinessProfilePhone != "" {
						nestedBusinessProfile["phone"] = bodyBusinessProfilePhone
					}
					{
						nestedBusinessProfileSupportChannels := map[string]any{}
						if bodyBusinessProfileSupportChannelsEmail != "" {
							nestedBusinessProfileSupportChannels["email"] = bodyBusinessProfileSupportChannelsEmail
						}
						if bodyBusinessProfileSupportChannelsPhone != "" {
							nestedBusinessProfileSupportChannels["phone"] = bodyBusinessProfileSupportChannelsPhone
						}
						if bodyBusinessProfileSupportChannelsUrl != "" {
							nestedBusinessProfileSupportChannels["url"] = bodyBusinessProfileSupportChannelsUrl
						}
						if len(nestedBusinessProfileSupportChannels) > 0 {
							nestedBusinessProfile["support_channels"] = nestedBusinessProfileSupportChannels
						}
					}
					if bodyBusinessProfileTaxId != "" {
						nestedBusinessProfile["tax_id"] = bodyBusinessProfileTaxId
					}
					if bodyBusinessProfileUseCase != "" {
						nestedBusinessProfile["use_case"] = bodyBusinessProfileUseCase
					}
					if bodyBusinessProfileWebsite != "" {
						nestedBusinessProfile["website"] = bodyBusinessProfileWebsite
					}
					if len(nestedBusinessProfile) > 0 {
						body["business_profile"] = nestedBusinessProfile
					}
				}
				if bodyExternalId != "" {
					body["external_id"] = bodyExternalId
				}
				if bodyMetadata != "" {
					var parsedMetadata any
					if err := json.Unmarshal([]byte(bodyMetadata), &parsedMetadata); err != nil {
						return fmt.Errorf("parsing --metadata JSON: %w", err)
					}
					body["metadata"] = parsedMetadata
				}
			}
			data, statusCode, err := c.PutWithParams(path, params, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Inspect the mutate response body for a partial-failure-shaped
			// field (e.g. Google Ads `partialFailureError`). Several Google
			// APIs return 200 OK with a partial-failure field when some
			// operations in the batch failed; ignoring it silently swallows
			// real failures. Detection runs before output-mode selection so
			// the exit code is consistent regardless of how stdout is
			// rendered. --dry-run short-circuits because no real request
			// was sent.
			var partialFailure *partialFailureReport
			if !flags.dryRun && statusCode >= 200 && statusCode < 300 {
				partialFailure = detectPartialFailure(data)
				if partialFailure != nil {
					fmt.Fprintf(os.Stderr, "warning: partial failure detected in %s response: %s\n", "accounts", partialFailure.Message)
					if len(partialFailure.ResourceNames) > 0 {
						fmt.Fprintf(os.Stderr, "         succeeded: %d operation(s)\n", len(partialFailure.ResourceNames))
					}
				}
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						if partialFailure != nil && !flags.allowPartialFailure {
							return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "accounts", partialFailure.Message))
						}
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							if partialFailure != nil && !flags.allowPartialFailure {
								return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "accounts", partialFailure.Message))
							}
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					if partialFailure != nil && !flags.allowPartialFailure {
						return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "accounts", partialFailure.Message))
					}
					return nil
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "accounts",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300 && (partialFailure == nil || flags.allowPartialFailure),
				}
				if partialFailure != nil {
					envelope["partial_failure"] = partialFailure
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				// Verify-mode synthetic envelope detection runs against RAW data
				// (before --compact/--select filtering) so the sentinel field is
				// guaranteed to be visible even if the operator passes a filter
				// flag that would otherwise strip it. Surfaces a top-level
				// verify_noop signal + flips success to false. Mirrors the dry_run
				// shape above.
				if len(data) > 0 {
					var rawParsed any
					if err := json.Unmarshal(data, &rawParsed); err == nil {
						if m, ok := rawParsed.(map[string]any); ok {
							if v, ok := m["__straddle_verify_synthetic__"].(bool); ok && v {
								envelope["verify_noop"] = true
								envelope["success"] = false
							}
						}
					}
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				if perr := printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true); perr != nil {
					return perr
				}
				if partialFailure != nil && !flags.allowPartialFailure {
					return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "accounts", partialFailure.Message))
				}
				return nil
			}
			// Fall-through for mutate paths that did not hit the table or
			// asJSON branches: --quiet, --csv, --plain, and default terminal
			// raw output. printOutputWithFlags renders the body, then the
			// typed partial-failure exit fires unless --allow-partial-failure
			// downgrades it. Without this guard a partial failure would exit
			// 0 for these output modes — the exact silent-swallow regression
			// the surrounding patch is preventing for asJSON / piped output.
			if perr := printOutputWithFlags(cmd.OutOrStdout(), data, flags); perr != nil {
				return perr
			}
			if partialFailure != nil && !flags.allowPartialFailure {
				return partialFailureErr(fmt.Errorf("partial failure in %s response: %s", "accounts", partialFailure.Message))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bodyBusinessProfileAddressCity, "business-profile-address-city", "", "City, district, suburb, town, or village.")
	cmd.Flags().StringVar(&bodyBusinessProfileAddressCountry, "business-profile-address-country", "", "The country of the address, in ISO 3166-1 alpha-2 format.")
	cmd.Flags().StringVar(&bodyBusinessProfileAddressLine1, "business-profile-address-line1", "", "Primary address line (e.g., street, PO Box).")
	cmd.Flags().StringVar(&bodyBusinessProfileAddressLine2, "business-profile-address-line2", "", "Secondary address line (e.g., apartment, suite, unit, or building).")
	cmd.Flags().StringVar(&bodyBusinessProfileAddressPostalCode, "business-profile-address-postal-code", "", "Postal or ZIP code.")
	cmd.Flags().StringVar(&bodyBusinessProfileAddressState, "business-profile-address-state", "", "Two-letter state code.")
	cmd.Flags().StringVar(&bodyBusinessProfileDescription, "business-profile-description", "", "A brief description of the business and its products or services.")
	cmd.Flags().StringVar(&bodyBusinessProfileIndustryCategory, "business-profile-industry-category", "", "The general category of the industry. Required if not providing MCC.")
	cmd.Flags().StringVar(&bodyBusinessProfileIndustryMcc, "business-profile-industry-mcc", "", "The Merchant Category Code (MCC) that best describes the business. Optional.")
	cmd.Flags().StringVar(&bodyBusinessProfileIndustrySector, "business-profile-industry-sector", "", "The specific sector within the industry category. Required if not providing MCC.")
	cmd.Flags().StringVar(&bodyBusinessProfileLegalName, "business-profile-legal-name", "", "The official registered name of the business.")
	cmd.Flags().StringVar(&bodyBusinessProfileName, "business-profile-name", "", "The operating or trade name of the business.")
	cmd.Flags().StringVar(&bodyBusinessProfilePhone, "business-profile-phone", "", "The primary contact phone number for the business.")
	cmd.Flags().StringVar(&bodyBusinessProfileSupportChannelsEmail, "business-profile-support-channels-email", "", "The email address for customer support inquiries.")
	cmd.Flags().StringVar(&bodyBusinessProfileSupportChannelsPhone, "business-profile-support-channels-phone", "", "The phone number for customer support.")
	cmd.Flags().StringVar(&bodyBusinessProfileSupportChannelsUrl, "business-profile-support-channels-url", "", "The URL of the business's customer support page or contact form.")
	cmd.Flags().StringVar(&bodyBusinessProfileTaxId, "business-profile-tax-id", "", "The business's tax identification number (e.g., EIN in the US).")
	cmd.Flags().StringVar(&bodyBusinessProfileUseCase, "business-profile-use-case", "", "A description of how the business intends to use Straddle's services.")
	cmd.Flags().StringVar(&bodyBusinessProfileWebsite, "business-profile-website", "", "URL of the business's primary marketing website.")
	cmd.Flags().StringVar(&bodyExternalId, "external-id", "", "Unique identifier for the account in your database, used for cross-referencing between Straddle and your systems.")
	cmd.Flags().StringVar(&bodyMetadata, "metadata", "", "Up to 20 additional user-defined key-value pairs. Useful for storing additional information about the account in a...")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
