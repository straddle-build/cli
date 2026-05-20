// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
)

// finishHumanOrOutput is the tail of printOutputWithFlags: it renders a
// single-object human detail card when the gate allows (--human-friendly on a
// TTY, no machine-format flags), otherwise it delegates to the normal
// printOutput path. renderDetailCard declines arrays/scalars/empty objects, so
// list output and every agent/JSON/piped path stay byte-identical.
//
// It lives in its own file (not appended to printOutputWithFlags) so the card
// hook adds no lines to the oversized, generated helpers.go.
func finishHumanOrOutput(w io.Writer, data json.RawMessage, flags *rootFlags) error {
	if shouldRenderDetailCard(w, flags) && renderDetailCard(w, data) {
		return nil
	}
	return printOutput(w, data, flags.asJSON)
}
