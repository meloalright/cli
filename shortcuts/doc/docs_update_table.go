// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/larksuite/cli/shortcuts/common"
)

// ── Table command routing ──

var validTableCommands = map[string]bool{
	"table_insert_rows":     true,
	"table_insert_cols":     true,
	"table_delete_rows":     true,
	"table_delete_cols":     true,
	"table_merge_cells":     true,
	"table_unmerge_cells":   true,
	"table_update_property": true,
}

func isTableCommand(cmd string) bool {
	return validTableCommands[cmd]
}

// tableUpdateFlags returns the flag definitions for table operations.
// All flags are Hidden: true following the v2 pattern — visible in versioned help only.
func tableUpdateFlags() []common.Flag {
	return []common.Flag{
		{Name: "table-block-id", Desc: "table block ID (from +fetch --detail with-ids)", Hidden: true},
		{Name: "cell", Desc: "cell coordinate in A1 notation (e.g. A1, B3)", Hidden: true},
		{Name: "range", Desc: "cell range in A1 notation, inclusive (e.g. A1:C3)", Hidden: true},
		{Name: "col", Desc: "column letter (A, B, ...) or 0=before-first, -1=append", Hidden: true},
		{Name: "row-index", Type: "int", Desc: "row position (0-indexed, -1=end)", Hidden: true},
		{Name: "row-start", Type: "int", Desc: "row start index (1-based, inclusive; e.g. 1=first row)", Hidden: true},
		{Name: "row-end", Type: "int", Desc: "row end index (1-based, exclusive; e.g. 2=up to but not including second row)", Hidden: true},
		{Name: "col-start", Desc: "column range start letter (inclusive)", Hidden: true},
		{Name: "col-end", Desc: "column range end letter (inclusive)", Hidden: true},
		{Name: "col-width", Type: "int", Desc: "column width in px", Hidden: true},
		{Name: "header-row", Type: "bool", Desc: "set first row as header", Hidden: true},
		{Name: "header-column", Type: "bool", Desc: "set first column as header", Hidden: true},
		{Name: "background-color", Desc: "cell background color (named or rgb/rgba)", Hidden: true},
		{Name: "vertical-align", Desc: "cell vertical alignment: top|middle|bottom", Hidden: true, Enum: []string{"top", "middle", "bottom"}},
	}
}

// ── A1 notation parsing ──

// parseColLetter converts a column letter (A=0, B=1, ..., Z=25, AA=26) to 0-based index.
// Special values: "0" = 0 (before-first), "-1" = -1 (append).
func parseColLetter(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty column")
	}
	if s == "0" {
		return 0, nil
	}
	if s == "-1" {
		return -1, nil
	}
	s = strings.ToUpper(s)
	col := 0
	for _, c := range s {
		if c < 'A' || c > 'Z' {
			return 0, fmt.Errorf("invalid column letter %q", s)
		}
		col = col*26 + int(c-'A'+1)
	}
	return col - 1, nil // convert to 0-based
}

// parseA1Cell parses "B3" → (row=2, col=1). Row is 1-based in input, 0-based in output.
func parseA1Cell(s string) (row, col int, err error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty cell reference")
	}
	s = strings.ToUpper(s)
	// Find boundary between letters and digits
	i := 0
	for i < len(s) && unicode.IsLetter(rune(s[i])) {
		i++
	}
	if i == 0 || i == len(s) {
		return 0, 0, fmt.Errorf("invalid cell %q: expected format like A1, B3", s)
	}
	colPart := s[:i]
	rowPart := s[i:]
	col, err = parseColLetter(colPart)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid cell %q: %w", s, err)
	}
	rowNum, err := strconv.Atoi(rowPart)
	if err != nil || rowNum < 1 {
		return 0, 0, fmt.Errorf("invalid cell %q: row must be >= 1", s)
	}
	return rowNum - 1, col, nil // convert to 0-based
}

// parseA1Range parses "A1:C3" (inclusive) → half-open (rowStart, rowEnd, colStart, colEnd).
func parseA1Range(s string) (rowStart, rowEnd, colStart, colEnd int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q: expected format like A1:C3", s)
	}
	r1, c1, err := parseA1Cell(parts[0])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	r2, c2, err := parseA1Cell(parts[1])
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if r2 < r1 || c2 < c1 {
		return 0, 0, 0, 0, fmt.Errorf("invalid range %q: start must be before end", s)
	}
	// Convert inclusive → half-open
	return r1, r2 + 1, c1, c2 + 1, nil
}

// ── Validation ──

func validateTableUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	cmd := runtime.Str("command")
	if !validTableCommands[cmd] {
		return common.FlagErrorf("invalid table command %q", cmd)
	}

	if runtime.Str("table-block-id") == "" {
		return common.FlagErrorf("--table-block-id is required for %s", cmd)
	}

	switch cmd {
	case "table_insert_rows":
		if runtime.Int("row-index") < -1 {
			return common.FlagErrorf("--row-index must be >= -1")
		}
	case "table_insert_cols":
		col := runtime.Str("col")
		if col == "" {
			return common.FlagErrorf("--col is required for table_insert_cols")
		}
		if _, err := parseColLetter(col); err != nil {
			return common.FlagErrorf("--col: %v", err)
		}
	case "table_delete_rows":
		if runtime.Int("row-start") < 0 {
			return common.FlagErrorf("--row-start must be >= 0")
		}
		if runtime.Int("row-end") <= runtime.Int("row-start") {
			return common.FlagErrorf("--row-end must be > --row-start")
		}
	case "table_delete_cols":
		colStart := runtime.Str("col-start")
		colEnd := runtime.Str("col-end")
		if colStart == "" || colEnd == "" {
			return common.FlagErrorf("--col-start and --col-end are required for table_delete_cols")
		}
		s, err := parseColLetter(colStart)
		if err != nil {
			return common.FlagErrorf("--col-start: %v", err)
		}
		e, err := parseColLetter(colEnd)
		if err != nil {
			return common.FlagErrorf("--col-end: %v", err)
		}
		if e < s {
			return common.FlagErrorf("--col-end must be >= --col-start")
		}
	case "table_merge_cells":
		rangeStr := runtime.Str("range")
		if rangeStr == "" {
			return common.FlagErrorf("--range is required for table_merge_cells")
		}
		if _, _, _, _, err := parseA1Range(rangeStr); err != nil {
			return common.FlagErrorf("--range: %v", err)
		}
	case "table_unmerge_cells":
		cellStr := runtime.Str("cell")
		if cellStr == "" {
			return common.FlagErrorf("--cell is required for table_unmerge_cells")
		}
		if _, _, err := parseA1Cell(cellStr); err != nil {
			return common.FlagErrorf("--cell: %v", err)
		}
	case "table_update_property":
		if runtime.Str("cell") != "" {
			// Cell-level mode
			if _, _, err := parseA1Cell(runtime.Str("cell")); err != nil {
				return common.FlagErrorf("--cell: %v", err)
			}
			if runtime.Str("background-color") == "" && runtime.Str("vertical-align") == "" {
				return common.FlagErrorf("cell-level update requires --background-color or --vertical-align")
			}
		} else {
			// Table-level mode
			if runtime.Int("col-width") != 0 && runtime.Str("col") == "" {
				return common.FlagErrorf("--col is required when --col-width is set")
			}
			if runtime.Str("col") != "" {
				if _, err := parseColLetter(runtime.Str("col")); err != nil {
					return common.FlagErrorf("--col: %v", err)
				}
			}
		}
	}
	return nil
}

// ── Request body construction ──

func buildTableRequestBody(runtime *common.RuntimeContext, cmd string) map[string]interface{} {
	return buildTableSingleBody(runtime, cmd)
}

// buildTableSingleBody packs the table-op request.
//
// The OpenAPI gateway maps top-level JSON keys 1:1 onto OpenDocsAIUpdateDocumentRequest's
// typed Thrift fields (command, format, revision_id, block_id, …). Table-specific
// parameters without dedicated Thrift fields travel as a JSON-encoded string under
// "extra_param" (field 10, *string), which ai_edit's handler decodes back into a
// struct. Keep the "extra_param" key names snake_case and in sync with the
// updateExtraParam struct in ai_edit/biz/handler/open_docs_ai.go.
//
// The table target block id is sent as the top-level "block_id" — same field used by
// block_* commands. ai_edit validates it in buildOpenDocsAIUpdateInput for table_*
// commands. The --table-block-id CLI flag name is kept for backwards compatibility.
func buildTableSingleBody(runtime *common.RuntimeContext, cmd string) map[string]interface{} {
	extra := map[string]interface{}{}

	// Everything cli produces here is a pure passthrough of what the user typed ——
	// no A1 splitting, no case-fold, no letter→index math. The SDK owns all of those
	// transforms (pkg/util.ParseA1Cell / ParseA1Range / ParseColLetter); cli forwarding
	// the same conversion would risk doubling it. "cell" and "range" travel as raw
	// A1 strings; ai_edit uses the SDK helpers to expand them before hitting GMFCommand.
	switch cmd {
	case "table_insert_rows":
		// Only include row_index when the caller explicitly set it. Omitting the key
		// lets the SDK's *int tri-state default to -1 (append at end), which matches
		// the --row-index help text ("0-indexed, -1=end"). The residual limitation is
		// that --row-index=0 (insert at the very top) is indistinguishable from
		// omitting the flag — acceptable, since the documented sentinel for "no row
		// specified" is -1 and an empty table makes top-vs-end equivalent anyway.
		if v := runtime.Int("row-index"); v != 0 {
			extra["row_index"] = v
		}
	case "table_insert_cols":
		extra["column_index"] = runtime.Str("col")
	case "table_delete_rows":
		// row_start / row_end are 1-based A1-style indices; 0 is never a valid
		// value, so treat it as "unset" and let ai_edit's validator reject the
		// request rather than silently forwarding row_start_index=0 downstream.
		if v := runtime.Int("row-start"); v != 0 {
			extra["row_start_index"] = v
		}
		if v := runtime.Int("row-end"); v != 0 {
			extra["row_end_index"] = v
		}
	case "table_delete_cols":
		extra["column_start_index"] = runtime.Str("col-start")
		extra["column_end_index"] = runtime.Str("col-end")
	case "table_merge_cells":
		extra["range"] = runtime.Str("range")
	case "table_unmerge_cells":
		extra["cell"] = runtime.Str("cell")
	case "table_update_property":
		if v := runtime.Str("cell"); v != "" {
			// Cell-level
			extra["cell"] = v
			if bg := runtime.Str("background-color"); bg != "" {
				extra["background_color"] = bg
			}
			if va := runtime.Str("vertical-align"); va != "" {
				extra["vertical_align"] = va
			}
		} else {
			// Table-level
			if v := runtime.Str("col"); v != "" {
				extra["column_index"] = v
			}
			if v := runtime.Int("col-width"); v != 0 {
				extra["column_width"] = v
			}
			if v := runtime.Str("header-row"); v != "" {
				extra["header_row"] = v == "true"
			}
			if v := runtime.Str("header-column"); v != "" {
				extra["header_column"] = v == "true"
			}
		}
	}

	// json.Marshal over a map[string]interface{} is deterministic only across Go versions
	// that sort map keys during marshal (all modern releases). We don't care about the
	// exact byte order; ai_edit decodes by key name, not position.
	extraJSON, _ := json.Marshal(extra)

	body := map[string]interface{}{
		"command":     cmd,
		"format":      "xml",
		"extra_param": string(extraJSON),
	}
	if v := runtime.Str("table-block-id"); v != "" {
		body["block_id"] = v
	}
	if v := runtime.Int("revision-id"); v != 0 {
		body["revision_id"] = v
	}
	return body
}

// ── Execution ──

func executeTableUpdate(_ context.Context, runtime *common.RuntimeContext) error {
	ref, err := parseDocumentRef(runtime.Str("doc"))
	if err != nil {
		return err
	}

	cmd := runtime.Str("command")
	body := buildTableRequestBody(runtime, cmd)

	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s", ref.Token)
	data, err := doDocAPI(runtime, "PUT", apiPath, body)
	if err != nil {
		return err
	}
	runtime.OutRaw(data, nil)
	return nil
}

func dryRunTableUpdate(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	ref, err := parseDocumentRef(runtime.Str("doc"))
	if err != nil {
		return common.NewDryRunAPI().Desc(fmt.Sprintf("error: %v", err))
	}
	cmd := runtime.Str("command")
	body := buildTableRequestBody(runtime, cmd)
	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s", ref.Token)
	return common.NewDryRunAPI().
		PUT(apiPath).
		Desc(fmt.Sprintf("OpenAPI: table operation %s", cmd)).
		Body(body).
		Set("document_id", ref.Token)
}
