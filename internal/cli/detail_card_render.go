// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

// Box-drawing for the human detail card. Hand-rolled (no TUI lib). All width,
// padding, and truncation math goes through go-runewidth so CJK/emoji (2
// cells) line up. Color is applied per-segment after the plain width is
// measured, so no line is ever measured with ANSI bytes in it; the box stays
// the same width whether or not color is on.

package cli

import (
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

// cardCtx holds the resolved geometry for one card render.
type cardCtx struct {
	width  int // outer box width including both borders
	body   int // inner content width: width - 4 ("│ " ... " │")
	labelW int // label column width (0 means no label column, ultra-narrow)
	valueW int // value column width
	color  bool
}

const (
	cardLabelCap = 28 // labels never eat more than this many columns
	cardGap      = 2  // spaces between label and value columns
)

// renderCard assembles the full framed card as a single string (no trailing
// newline). width is the outer box width; it is guarded so even tiny widths
// do not panic or produce negative padding.
func renderCard(title string, blocks []cardBlock, width int, color bool) string {
	if width < 8 {
		width = 8
	}
	c := &cardCtx{width: width, body: width - 4, color: color}
	c.resolveColumns(blocks)

	lines := make([]string, 0, len(blocks)+2)
	lines = append(lines, c.topBorder(title))
	for _, b := range blocks {
		lines = append(lines, c.renderBlock(b)...)
	}
	lines = append(lines, c.bottomBorder())
	return strings.Join(lines, "\n")
}

// resolveColumns sizes the label and value columns from the widest label
// (including indented section children), capped and guarded so the value
// column keeps at least one cell.
func (c *cardCtx) resolveColumns(blocks []cardBlock) {
	maxL := 0
	consider := func(s string) {
		if w := dispWidth(s); w > maxL {
			maxL = w
		}
	}
	for _, b := range blocks {
		consider(b.label)
		if b.kind == blockSection {
			for _, r := range b.rows {
				consider("  " + r.label)
			}
		}
	}
	labelW := maxL
	if labelW > cardLabelCap {
		labelW = cardLabelCap
	}
	// Keep room for the gap and at least one value cell.
	if labelW > c.body-cardGap-1 {
		labelW = c.body - cardGap - 1
	}
	if labelW < 0 {
		labelW = 0
	}
	c.labelW = labelW
	if labelW > 0 {
		c.valueW = c.body - labelW - cardGap
	} else {
		c.valueW = c.body
	}
	if c.valueW < 1 {
		c.valueW = 1
	}
}

func (c *cardCtx) renderBlock(b cardBlock) []string {
	switch b.kind {
	case blockSection:
		out := []string{c.blankLine(), c.bodyLine(b.label, "", true)}
		for _, r := range b.rows {
			out = append(out, c.renderRow("  "+r.label, r.value)...)
		}
		return out
	case blockTable:
		return c.renderTable(b)
	default: // blockRow
		return c.renderRow(b.label, b.value)
	}
}

// renderRow renders a label:value pair, wrapping a long value across multiple
// physical lines with the label shown only on the first.
func (c *cardCtx) renderRow(label, value string) []string {
	vlines := wrapByWidth(value, c.valueW)
	out := make([]string, 0, len(vlines))
	for i, vl := range vlines {
		lab := ""
		if i == 0 {
			lab = label
		}
		out = append(out, c.bodyLine(lab, vl, true))
	}
	return out
}

// renderTable renders a nested array of objects as an indented sub-table: a
// bold field-name header, a bold column header row, then one line per element.
func (c *cardCtx) renderTable(b cardBlock) []string {
	out := []string{c.blankLine(), c.bodyLine(b.label, "", true)}
	avail := c.body - 2 // 2-space indent
	if avail < 4 {
		// Too narrow for columns; degrade to a count.
		return append(out, c.renderRow("  items", strconv.Itoa(len(b.cells)))...)
	}
	colW := layoutColumns(b.headers, b.cells, avail)
	out = append(out, c.rawBodyLine("  "+renderColumnsRow(b.headers, colW), true))
	for _, row := range b.cells {
		out = append(out, c.rawBodyLine("  "+renderColumnsRow(row, colW), false))
	}
	return out
}

// bodyLine builds one physical content line: "│ " + label column + gap +
// value column + " │". Padding is computed on the plain text; bold is wrapped
// around the already-padded plain label so the visible width is unchanged.
func (c *cardCtx) bodyLine(label, value string, boldLabel bool) string {
	var b strings.Builder
	b.WriteString("│ ")
	if c.labelW > 0 {
		lp := padRight(label, c.labelW)
		if c.color && boldLabel && strings.TrimSpace(label) != "" {
			b.WriteString(ansiBold(lp))
		} else {
			b.WriteString(lp)
		}
		b.WriteString(strings.Repeat(" ", cardGap))
	}
	b.WriteString(padRight(value, c.valueW))
	b.WriteString(" │")
	return b.String()
}

// rawBodyLine wraps a pre-built body string (already indented) in borders,
// padding it to the full content width. Bold, when requested, wraps the padded
// plain string so width math is unaffected.
func (c *cardCtx) rawBodyLine(s string, boldLine bool) string {
	p := padRight(s, c.body)
	if c.color && boldLine {
		p = ansiBold(p)
	}
	return "│ " + p + " │"
}

func (c *cardCtx) blankLine() string {
	return "│" + strings.Repeat(" ", c.width-2) + "│"
}

func (c *cardCtx) bottomBorder() string {
	return "└" + strings.Repeat("─", c.width-2) + "┘"
}

// topBorder draws "┌─ Title ───────┐", truncating the title if needed and
// always leaving at least one trailing dash. Ultra-narrow widths fall back to
// a plain top rule.
func (c *cardCtx) topBorder(title string) string {
	const prefixW = 3                                      // "┌─ "
	avail := c.width - prefixW - 1 /*space*/ - 1 /*┐*/ - 1 /*min dash*/
	if avail < 1 {
		return "┌" + strings.Repeat("─", c.width-2) + "┐"
	}
	t := truncateCell(title, avail)
	dashes := c.width - prefixW - dispWidth(t) - 1 /*space*/ - 1 /*┐*/
	if dashes < 1 {
		dashes = 1
	}
	mid := t
	if c.color && t != "" {
		mid = ansiBold(t)
	}
	return "┌─ " + mid + " " + strings.Repeat("─", dashes) + "┐"
}

// ansiBold wraps s in SGR bold. Used only when color is enabled and after the
// plain width of s has already been accounted for.
func ansiBold(s string) string {
	return "\033[1m" + s + "\033[0m"
}

// dispWidth is the visible cell width of s (CJK/emoji count as 2).
func dispWidth(s string) int {
	return runewidth.StringWidth(s)
}

// padRight pads s with spaces to exactly w cells, or truncates it if it is
// wider. Negative or zero w yields an empty string.
func padRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := dispWidth(s)
	if sw == w {
		return s
	}
	if sw < w {
		return s + strings.Repeat(" ", w-sw)
	}
	// Truncating can land on a wide-rune boundary and come back narrower than
	// w (the wide rune is dropped); pad the remainder so the cell is exactly w.
	t := runewidth.Truncate(s, w, "")
	if tw := dispWidth(t); tw < w {
		t += strings.Repeat(" ", w-tw)
	}
	return t
}

// truncateCell shortens s to fit w cells, appending an ellipsis when there is
// room for one.
func truncateCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispWidth(s) <= w {
		return s
	}
	if w == 1 {
		return runewidth.Truncate(s, w, "")
	}
	return runewidth.Truncate(s, w, "…")
}

// wrapByWidth word-wraps s to lines no wider than w cells, hard-breaking
// tokens longer than w. A single rune that overflows w (a wide rune in a
// 1-cell column) is emitted on its own line rather than looping forever.
// Always returns at least one line.
func wrapByWidth(s string, w int) []string {
	if w <= 0 {
		return []string{""}
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		cur := ""
		flushLong := func(word string) string {
			for dispWidth(word) > w {
				head := runewidth.Truncate(word, w, "")
				if head == "" { // can't fit even one cell; force one rune
					r := []rune(word)
					lines = append(lines, string(r[0]))
					word = string(r[1:])
					continue
				}
				lines = append(lines, head)
				word = word[len(head):]
			}
			return word
		}
		for _, word := range words {
			switch {
			case cur == "":
				cur = flushLong(word)
			case dispWidth(cur)+1+dispWidth(word) <= w:
				cur += " " + word
			default:
				lines = append(lines, cur)
				cur = flushLong(word)
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// layoutColumns computes per-column widths from header and cell content,
// shrinking the widest columns when the natural layout exceeds avail.
func layoutColumns(headers []string, cells [][]string, avail int) []int {
	n := len(headers)
	colW := make([]int, n)
	for i, h := range headers {
		colW[i] = dispWidth(h)
	}
	for _, row := range cells {
		for i := 0; i < n && i < len(row); i++ {
			if w := dispWidth(row[i]); w > colW[i] {
				colW[i] = w
			}
		}
	}
	gaps := 0
	if n > 1 {
		gaps = 2 * (n - 1)
	}
	total := gaps
	for _, w := range colW {
		total += w
	}
	// Shrink the currently-widest column one cell at a time until it fits or
	// nothing can reasonably shrink further (floor of 4 cells per column).
	for total > avail {
		mi := 0
		for i := 1; i < n; i++ {
			if colW[i] > colW[mi] {
				mi = i
			}
		}
		if colW[mi] <= 4 {
			break
		}
		colW[mi]--
		total--
	}
	return colW
}

// renderColumnsRow joins values into a fixed-width row using the given column
// widths, truncating overlong cells and padding short ones.
func renderColumnsRow(values []string, colW []int) string {
	parts := make([]string, len(colW))
	for i := range colW {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts[i] = padRight(truncateCell(v, colW[i]), colW[i])
	}
	return strings.Join(parts, "  ")
}
