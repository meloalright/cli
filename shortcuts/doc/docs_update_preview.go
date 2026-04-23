// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"fmt"
	"strings"

	"github.com/larksuite/cli/shortcuts/common"
)

// previewAtxHeadingLevel returns the ATX heading level (1..6) of an ATX
// heading line such as "## Section", or 0 otherwise. Kept local to the
// preview module so Case 11 doesn't depend on the Case 2 branch (PR #619);
// the two implementations will converge once both land on main.
func previewAtxHeadingLevel(line string) int {
	// CommonMark §4.2: 0..3 leading spaces OK, 4+ spaces or any leading
	// tab force indented-code semantics.
	for i := 0; i < 4 && i < len(line); i++ {
		if line[i] == '\t' {
			return 0
		}
		if line[i] != ' ' {
			line = line[i:]
			goto scan
		}
	}
	if len(line) >= 4 {
		return 0
	}
	line = ""
scan:
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0
	}
	if level == len(line) {
		return level
	}
	switch line[level] {
	case ' ', '\t':
		return level
	}
	return 0
}

// fetchMarkdownForPreview calls the fetch-doc MCP tool and returns the
// markdown payload. Errors bubble up verbatim so the caller (preview mode)
// can surface them directly — a failed pre-fetch is fatal to preview
// because the whole point is to reason about the document's current state.
func fetchMarkdownForPreview(runtime *common.RuntimeContext, docID string) (string, error) {
	result, err := common.CallMCPTool(runtime, "fetch-doc", map[string]interface{}{
		"doc_id":           docID,
		"skip_task_detail": true,
	})
	if err != nil {
		return "", err
	}
	md, _ := result["markdown"].(string)
	return md, nil
}

// previewMatch captures where a selection expression matches in a document's
// markdown snapshot. Line numbers are 1-based inclusive, matching what the
// user sees in any markdown editor.
type previewMatch struct {
	StartLine int
	EndLine   int
	Snippet   string
}

// previewResult is the JSON shape returned by `docs +update --preview`.
// It captures everything the CLI can say about an update *without* calling
// the write-path: where the selection matches, how many matches were found,
// and the raw markdown payload that would be sent. The caller can then
// sanity-check the selection is unique and the payload looks right before
// re-running without --preview.
//
// Explicitly not included: the "rendered after" view. The CLI does not know
// how the server will convert markdown into Lark blocks, so simulating it
// here would mislead rather than help. The payload is reported verbatim.
type previewResult struct {
	Mode            string         `json:"mode"`
	DocID           string         `json:"doc_id"`
	MatchCount      int            `json:"match_count"`
	Matches         []previewMatch `json:"matches,omitempty"`
	PayloadMarkdown string         `json:"payload_markdown,omitempty"`
	PayloadLines    int            `json:"payload_lines,omitempty"`
	NewTitle        string         `json:"new_title,omitempty"`
	Note            string         `json:"note,omitempty"`
}

// findSelectionWithEllipsis locates the first block of markdown that starts
// with any line containing the ellipsis prefix and ends at any line
// containing the suffix. A plain-text selection (no "...") matches the
// smallest line range containing the exact substring.
//
// The matching is intentionally simpler than the server's real selection
// resolution (it operates on the raw fetched markdown rather than the block
// tree), so a preview match is advisory. A returned empty slice means "not
// found in the local markdown"; the server may still find it (e.g. in a
// block that fetch-doc renders differently), which is why preview never
// blocks the subsequent real update.
func findSelectionWithEllipsis(markdown, selection string) []previewMatch {
	lines := strings.Split(markdown, "\n")
	if !strings.Contains(selection, "...") {
		return findSingleLineMatches(lines, selection)
	}
	parts := strings.SplitN(selection, "...", 2)
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	if start == "" || end == "" {
		return nil
	}
	var matches []previewMatch
	for i, line := range lines {
		startIdx := strings.Index(line, start)
		if startIdx < 0 {
			continue
		}
		// Find the first subsequent line (or same line) containing end.
		// On the same line, end must appear strictly after start to avoid
		// a false "A...B" match when the document actually reads "B ... A".
		for j := i; j < len(lines); j++ {
			if j == i {
				after := line[startIdx+len(start):]
				if !strings.Contains(after, end) {
					continue
				}
			} else if !strings.Contains(lines[j], end) {
				continue
			}
			matches = append(matches, previewMatch{
				StartLine: i + 1,
				EndLine:   j + 1,
				Snippet:   strings.Join(lines[i:j+1], "\n"),
			})
			break
		}
	}
	return matches
}

// findSingleLineMatches returns every line whose content contains needle.
// Used for ellipsis-free selections; each match is a single-line range.
func findSingleLineMatches(lines []string, needle string) []previewMatch {
	var matches []previewMatch
	for i, line := range lines {
		if strings.Contains(line, needle) {
			matches = append(matches, previewMatch{
				StartLine: i + 1,
				EndLine:   i + 1,
				Snippet:   line,
			})
		}
	}
	return matches
}

// findSelectionByTitle locates a heading block in markdown by its ATX
// heading syntax (e.g. "## Overview"). The match range is the heading line
// itself plus the run of content lines before the next heading of equal or
// shallower level — i.e. the section the heading owns.
func findSelectionByTitle(markdown, title string) []previewMatch {
	targetLevel := previewAtxHeadingLevel(strings.TrimSpace(title))
	if targetLevel == 0 {
		return nil
	}
	targetText := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(title), "#"))
	lines := strings.Split(markdown, "\n")
	// Pre-compute fence state for each line so the outer and inner scans
	// both see a consistent "in-fence" signal. Heading-looking lines inside
	// ```…``` or ~~~…~~~ are code content, not real headings, and must not
	// start a match or terminate a matched section.
	inFence := fenceStateByLine(lines)
	var matches []previewMatch
	for i, line := range lines {
		if inFence[i] {
			continue
		}
		level := previewAtxHeadingLevel(line)
		if level != targetLevel {
			continue
		}
		headingText := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if headingText != targetText {
			continue
		}
		// Find section end: next non-fenced heading of equal or shallower level.
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if inFence[j] {
				continue
			}
			if lvl := previewAtxHeadingLevel(lines[j]); lvl != 0 && lvl <= targetLevel {
				end = j
				break
			}
		}
		matches = append(matches, previewMatch{
			StartLine: i + 1,
			EndLine:   end,
			Snippet:   strings.Join(lines[i:end], "\n"),
		})
	}
	return matches
}

// fenceStateByLine returns a bool slice where result[i] reports whether
// lines[i] sits inside a fenced code block (between a ```/~~~ opener and
// its matching closer). The opener/closer lines themselves are reported as
// inside-fence so heading matching skips them too.
func fenceStateByLine(lines []string) []bool {
	state := make([]bool, len(lines))
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			state[i] = true
			inFence = !inFence
			continue
		}
		state[i] = inFence
	}
	return state
}

// buildPreviewResult constructs the preview output object for a given
// update invocation. It never calls MCP — it operates on the already-
// fetched markdown snapshot. The caller decides when to skip the actual
// write (currently: always, when --preview is set).
func buildPreviewResult(mode, docID, markdown, selectionWithEllipsis, selectionByTitle, newTitle, beforeMarkdown string) *previewResult {
	res := &previewResult{
		Mode:     mode,
		DocID:    docID,
		NewTitle: newTitle,
	}
	res.PayloadMarkdown = markdown
	if markdown != "" {
		res.PayloadLines = len(strings.Split(markdown, "\n"))
	}

	switch mode {
	case "append":
		res.Note = fmt.Sprintf("would append %d lines to end of document (preview does not read the trailing block)", res.PayloadLines)
		return res
	case "overwrite":
		res.Note = fmt.Sprintf("would replace the entire document with %d lines (all existing blocks are discarded)", res.PayloadLines)
		return res
	}

	switch {
	case selectionWithEllipsis != "":
		res.Matches = findSelectionWithEllipsis(beforeMarkdown, selectionWithEllipsis)
	case selectionByTitle != "":
		res.Matches = findSelectionByTitle(beforeMarkdown, selectionByTitle)
	}
	res.MatchCount = len(res.Matches)
	switch {
	case res.MatchCount == 0:
		res.Note = "selection did not match any line in the fetched markdown snapshot; the real update may still succeed if the server resolves selection differently, but you probably want to narrow the selection first"
	case res.MatchCount > 1 && mode == "replace_all":
		res.Note = fmt.Sprintf("selection matches %d locations; replace_all would apply to all matches", res.MatchCount)
	case res.MatchCount > 1:
		res.Note = fmt.Sprintf("selection matches %d locations; only replace_all is defined to apply to all matches — other modes act on the first match and the result may be non-deterministic across retries", res.MatchCount)
	case res.MatchCount == 1:
		res.Note = "selection matches exactly one location"
	}
	return res
}
