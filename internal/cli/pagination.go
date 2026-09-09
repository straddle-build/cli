// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
)

// Bound APIs that ignore page_number or continually grow while being read.
const maxNumberedPages = 10000

type numberedPager struct {
	query        url.Values
	page         int
	pageSize     int
	fetched      int
	previousData json.RawMessage
}

type numberedPage struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		PageNumber *int `json:"page_number"`
		PageSize   *int `json:"page_size"`
		TotalPages *int `json:"total_pages"`
		TotalItems *int `json:"total_items"`
		TotalRows  *int `json:"total_rows"`
	} `json:"meta"`
	items []json.RawMessage
}

func newNumberedPager(query url.Values) (*numberedPager, error) {
	p := &numberedPager{query: query, page: 1}
	if raw := query.Get("page_number"); raw != "" {
		page, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid page_number: %w", err)
		}
		p.page = max(1, page)
	}
	if raw := query.Get("page_size"); raw != "" {
		size, err := strconv.Atoi(raw)
		if err != nil || size <= 0 {
			return nil, fmt.Errorf("page_size must be a positive integer")
		}
		p.pageSize = size
	}
	p.query.Set("page_number", strconv.Itoa(p.page))
	return p, nil
}

func (p *numberedPager) advance(data json.RawMessage) ([]json.RawMessage, bool, error) {
	page, err := readNumberedPage(data)
	if err != nil {
		return nil, false, err
	}
	if page.Meta.PageNumber != nil && *page.Meta.PageNumber != p.page {
		return nil, false, fmt.Errorf("pagination did not advance: requested page %d, received page %d", p.page, *page.Meta.PageNumber)
	}
	if len(page.items) > 0 && bytes.Equal(page.Data, p.previousData) {
		return nil, false, fmt.Errorf("pagination did not advance: page %d repeated the previous page", p.page)
	}
	if page.Meta.PageSize != nil {
		if p.fetched > 0 && p.pageSize > 0 && *page.Meta.PageSize != p.pageSize {
			return nil, false, fmt.Errorf("pagination page_size changed from %d to %d", p.pageSize, *page.Meta.PageSize)
		}
		p.pageSize = *page.Meta.PageSize
	}
	if err := page.validateCount(p.page, p.pageSize); err != nil {
		return nil, false, err
	}
	more, err := page.hasMore(p.page, p.pageSize)
	if err != nil {
		return nil, false, err
	}
	if more && len(page.items) == 0 {
		return nil, false, fmt.Errorf("pagination returned an empty page %d before the advertised end", p.page)
	}
	p.fetched++
	if more {
		if p.fetched >= maxNumberedPages || p.page == math.MaxInt {
			return nil, false, fmt.Errorf("pagination exceeded its page limit; increase --page-size or narrow the filters")
		}
		p.page++
		p.query.Set("page_number", strconv.Itoa(p.page))
	}
	p.previousData = page.Data
	return page.items, more, nil
}

func readNumberedPage(data json.RawMessage) (numberedPage, error) {
	var page numberedPage
	if err := json.Unmarshal(data, &page); err != nil {
		return page, fmt.Errorf("invalid paginated response: %w", err)
	}
	raw := bytes.TrimSpace(page.Data)
	if len(raw) == 0 || raw[0] != '[' {
		return page, fmt.Errorf("paginated response must contain a data array")
	}
	if err := json.Unmarshal(raw, &page.items); err != nil {
		return page, fmt.Errorf("invalid paginated data: %w", err)
	}
	for _, field := range []struct {
		name  string
		value *int
		min   int
	}{
		{"page_number", page.Meta.PageNumber, 1},
		{"page_size", page.Meta.PageSize, 1},
		{"total_pages", page.Meta.TotalPages, 0},
		{"total_items", page.Meta.TotalItems, 0},
		{"total_rows", page.Meta.TotalRows, 0},
	} {
		if field.value != nil && *field.value < field.min {
			return page, fmt.Errorf("invalid pagination meta.%s: %d", field.name, *field.value)
		}
	}
	return page, nil
}

func (p numberedPage) validateCount(page, size int) error {
	total := p.Meta.TotalItems
	if total == nil {
		total = p.Meta.TotalRows
	}
	if total != nil && *total == 0 && len(p.items) != 0 {
		return fmt.Errorf("pagination returned records for an advertised zero total")
	}
	if total != nil && size > 0 {
		pages := 0
		if *total > 0 {
			pages = 1 + (*total-1)/size
		}
		if p.Meta.TotalPages != nil && *p.Meta.TotalPages != pages {
			return fmt.Errorf("pagination total_pages disagrees with the total count and page_size")
		}
		expected := 0
		if *total > 0 && page <= pages {
			expected = min(size, *total-(page-1)*size)
		}
		if len(p.items) != expected {
			return fmt.Errorf("pagination page %d returned %d records, expected %d from the total count", page, len(p.items), expected)
		}
	}
	if p.Meta.TotalPages != nil {
		if page <= *p.Meta.TotalPages && (len(p.items) == 0 || page < *p.Meta.TotalPages && size > 0 && len(p.items) != size) {
			return fmt.Errorf("pagination page %d is incomplete for the advertised total_pages", page)
		}
		if page > *p.Meta.TotalPages && len(p.items) != 0 {
			return fmt.Errorf("pagination returned records beyond the advertised total_pages")
		}
	}
	return nil
}

func (p numberedPage) hasMore(requestedPage, requestedSize int) (bool, error) {
	page, size := requestedPage, requestedSize
	if p.Meta.PageNumber != nil {
		page = *p.Meta.PageNumber
	}
	if p.Meta.PageSize != nil {
		size = *p.Meta.PageSize
	}
	if p.Meta.TotalPages != nil {
		return page < *p.Meta.TotalPages, nil
	}
	total := p.Meta.TotalItems
	if total == nil {
		total = p.Meta.TotalRows
	}
	if total != nil {
		if *total == 0 {
			return false, nil
		}
		if size <= 0 {
			return false, fmt.Errorf("pagination needs a positive page_size to interpret the total count")
		}
		return page < 1+(*total-1)/size, nil
	}
	// Embed totals are optional. A short page ends the read when its size is
	// known; missing metadata alone must never mean all results were fetched.
	if len(p.items) == 0 {
		return false, nil
	}
	if size > 0 {
		return len(p.items) >= size, nil
	}
	return false, fmt.Errorf("cannot fetch all pages: response has no pagination total or page size")
}

func emitNumberedTruncationWarning(data json.RawMessage, query url.Values) {
	page, err := readNumberedPage(data)
	if err != nil || page.Meta.TotalPages == nil && page.Meta.TotalItems == nil && page.Meta.TotalRows == nil {
		return
	}
	requestedPage, _ := strconv.Atoi(query.Get("page_number"))
	requestedSize, _ := strconv.Atoi(query.Get("page_size"))
	more, err := page.hasMore(max(1, requestedPage), requestedSize)
	if err != nil || !more {
		return
	}
	if humanFriendly {
		fmt.Fprintln(os.Stderr, "warning: results truncated; more pages available. Re-run with --all to fetch every page.")
	} else {
		fmt.Fprintln(os.Stderr, `{"event":"truncated","hint":"pass --all to fetch every page"}`)
	}
}
