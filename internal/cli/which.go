// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// whichEntry is one row of the curated capability index. The index is
// seeded at generation time from the same NovelFeature list that drives
// the SKILL.md feature section, so the command a `which` query returns
// is guaranteed to exist and to match what the skill advertises.
type whichEntry struct {
	Command      string `json:"command"`
	Description  string `json:"description"`
	Group        string `json:"group,omitempty"`
	WhyItMatters string `json:"why_it_matters,omitempty"`
}

// whichIndex is the curated list of capabilities this CLI advertises as
// its hero features. Endpoint-level commands are discoverable via
// `--help`; `which` exists to resolve a natural-language capability
// query to one of the commands the skill says matter most.
var whichIndex = []whichEntry{
	{Command: "reconcile", Description: "Match synced charges and payouts to their funding events locally, showing what has settled to your account and what is still outstanding.", Group: "Settlement & cashflow", WhyItMatters: "Reach for this to answer 'which charges funded this deposit' or 'what is still unsettled' without paging the API per payment."},
	{Command: "pipeline", Description: "Group synced charges and payouts by lifecycle status and flag which are still cancelable (created/scheduled/on_hold) versus locked once they reach pending.", Group: "Payment lifecycle control", WhyItMatters: "Use before a cutoff to find every payment you can still stop, since pending and later states cannot be cancelled."},
	{Command: "returns", Description: "Surface failed and reversed payments with their ACH reason codes and rank repeat-offender paykeys and customers from the local store.", Group: "Payment lifecycle control", WhyItMatters: "Use to spot accounts that keep returning (R01 NSF, R02 closed, R05 dispute) so you can block or re-verify them."},
	{Command: "review-queue", Description: "List customers and paykeys sitting in review status, oldest first, with age-in-queue so the KYC backlog is triageable.", Group: "Risk & identity ops", WhyItMatters: "Use to clear the identity backlog: these are the items blocking downstream charges and payouts from releasing."},
	{Command: "cashflow", Description: "Aggregate synced charge volume in versus payout volume out over a date window, including zero-activity days, with net flow per day or week.", Group: "Settlement & cashflow", WhyItMatters: "Use to see money in versus money out at a glance, including the days nothing moved, without summing payments by hand."},
	{Command: "expiring", Description: "List paykeys approaching their expires_at and blocked paykeys that are unblock-eligible, so payments do not fail on stale tokens.", Group: "Risk & identity ops", WhyItMatters: "Use to find paykeys to refresh or unblock before recurring charges fail against an expired or blocked token."},
	{Command: "sandbox", Description: "Print the deterministic sandbox_outcome values for customers, paykeys, charges, and payouts plus the sandbox test bank values so test scenarios are scriptable.", Group: "Sandbox testing", WhyItMatters: "Use when writing sandbox tests to pick the exact sandbox_outcome (paid, failed_insufficient_funds, reversed_customer_dispute) that triggers the state you want."},
}

// whichMatch pairs an index entry with its ranking score for a query.
// Higher score means stronger match. The ranker is naive (exact token
// then substring then group tag) because 20-40 entries do not need
// semantic retrieval - a ranker upgrade is a future change that would
// not break this contract.
type whichMatch struct {
	Entry whichEntry `json:"entry"`
	Score int        `json:"score"`
}

// rankWhich returns up to `limit` best matches for `query` against the
// index, sorted by descending score. Score breakdown:
//
//	+3  exact token match on the command's leaf or full path
//	+2  substring match on the command (any part)
//	+2  substring match on the description
//	+1  group tag contains the query as a word
//
// Ties break on declaration order in the index. An empty query returns
// every entry at score 0 in declaration order - this is the "list all"
// behavior the skill documents for broad agent discovery.
func rankWhich(index []whichEntry, query string, limit int) []whichMatch {
	if limit <= 0 {
		limit = 3
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]whichMatch, 0, len(index))
		for _, e := range index {
			out = append(out, whichMatch{Entry: e, Score: 0})
		}
		return out
	}
	qTokens := strings.Fields(q)

	scored := make([]whichMatch, 0, len(index))
	for i, e := range index {
		score := whichScoreEntry(e, q, qTokens)
		scored = append(scored, whichMatch{Entry: e, Score: score})
		_ = i
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	// Drop zero-score matches when the query was non-empty; agents
	// branching on exit code rely on "no match" meaning no confidence.
	filtered := scored[:0]
	for _, m := range scored {
		if m.Score > 0 {
			filtered = append(filtered, m)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func whichScoreEntry(e whichEntry, query string, qTokens []string) int {
	score := 0
	cmd := strings.ToLower(e.Command)
	cmdTokens := strings.Fields(cmd)
	desc := strings.ToLower(e.Description)
	group := strings.ToLower(e.Group)

	// Exact token match on the command path (any token).
	for _, qt := range qTokens {
		for _, ct := range cmdTokens {
			if qt == ct {
				score += 3
				break
			}
		}
	}
	// Substring match on the full command (covers hyphenated leaves).
	if strings.Contains(cmd, query) {
		score += 2
	}
	// Substring match on the description.
	if strings.Contains(desc, query) {
		score += 2
	}
	// Group tag match.
	if group != "" {
		for _, qt := range qTokens {
			if strings.Contains(group, qt) {
				score += 1
				break
			}
		}
	}
	return score
}

func newWhichCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "which [query]",
		Short: "Find the command that implements a capability",
		Annotations: map[string]string{
			"straddle:typed-exit-codes": "0,2",
		},
		Long: `which resolves a natural-language capability query (for example, "search messages" or "stale tickets") to the best matching command from this CLI's curated feature index.

Exit codes:
  0  at least one match found
  2  no confident match - the query did not score against any indexed capability; fall back to '--help' or 'search' if this CLI has one`,
		Example: `  straddle which "stale tickets"
  straddle which "bottleneck"
  straddle which --limit 1 "send message"
  straddle which                                # list the full capability index`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(whichIndex) == 0 {
				return usageErr(fmt.Errorf("this CLI has no curated capability index; run '--help' to see every command"))
			}
			query := strings.Join(args, " ")
			matches := rankWhich(whichIndex, query, limit)

			// Empty query returns the whole index at score 0 (listing mode).
			if strings.TrimSpace(query) == "" {
				return renderWhich(cmd, flags, rankWhichAll(whichIndex))
			}

			if len(matches) == 0 {
				// Under --json, return an empty matches envelope at exit 0
				// so agents can branch on `matches.length == 0` instead of
				// parsing a usage error message. Non-JSON keeps the typed
				// exit-2 path so terminal users see the help hint.
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"matches": []whichMatch{},
					}, flags)
				}
				return usageErr(fmt.Errorf("no match for %q; try '%s --help' for the full command list", query, cmd.Root().Name()))
			}
			return renderWhich(cmd, flags, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 3, "Maximum number of matches to return")
	return cmd
}

// rankWhichAll is a narrow helper used by the "empty query lists the
// index" path. It returns every entry in declaration order at score 0
// so the render path treats them uniformly.
func rankWhichAll(index []whichEntry) []whichMatch {
	out := make([]whichMatch, 0, len(index))
	for _, e := range index {
		out = append(out, whichMatch{Entry: e, Score: 0})
	}
	return out
}

func renderWhich(cmd *cobra.Command, flags *rootFlags, matches []whichMatch) error {
	w := cmd.OutOrStdout()
	// Output shape follows the same rule as every other generated
	// command: JSON when the caller asked for it OR when stdout is not
	// a terminal; table when a human is looking.
	asJSON := flags.asJSON
	if !asJSON && !isTerminal(w) {
		asJSON = true
	}
	if asJSON {
		// JSON envelope: {matches: [...]}. The wrap is critical:
		// printJSONFiltered's --compact path uses compactListFields
		// (allowlist) for top-level arrays, which would strip
		// entry/score keys; routing through compactObjectFields
		// (blocklist) via an object envelope preserves them.
		if matches == nil {
			matches = []whichMatch{}
		}
		return printJSONFiltered(w, map[string]any{"matches": matches}, flags)
	}
	fmt.Fprintf(w, "%-24s  %-8s  %s\n", "COMMAND", "SCORE", "DESCRIPTION")
	for _, m := range matches {
		fmt.Fprintf(w, "%-24s  %-8d  %s\n", m.Entry.Command, m.Score, m.Entry.Description)
	}
	return nil
}
