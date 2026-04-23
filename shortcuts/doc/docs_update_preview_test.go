// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"strings"
	"testing"
)

func TestPreviewAtxHeadingLevel(t *testing.T) {
	t.Parallel()

	cases := map[string]int{
		"# H1":          1,
		"## H2":         2,
		"###### H6":     6,
		"####### x":     0,
		"##":            2,
		"##\tTitle":     2,
		"##no-space":    0,
		"":              0,
		"  ## indented": 2,
		"   ## edge":    2,
		"    ## code":   0,
		"\t## code":     0,
		"plain prose":   0,
	}
	for input, want := range cases {
		if got := previewAtxHeadingLevel(input); got != want {
			t.Errorf("previewAtxHeadingLevel(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestFindSelectionWithEllipsis(t *testing.T) {
	t.Parallel()

	md := strings.Join([]string{
		"# Doc title",
		"",
		"intro paragraph",
		"",
		"## Section One",
		"",
		"content of section one spanning",
		"multiple lines and ending here.",
		"",
		"## Section Two",
		"",
		"other content.",
	}, "\n")

	t.Run("plain substring single-line", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionWithEllipsis(md, "intro paragraph")
		if len(matches) != 1 {
			t.Fatalf("want 1 match, got %d", len(matches))
		}
		if matches[0].StartLine != 3 || matches[0].EndLine != 3 {
			t.Errorf("want lines 3..3, got %d..%d", matches[0].StartLine, matches[0].EndLine)
		}
	})

	t.Run("ellipsis spans multiple lines", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionWithEllipsis(md, "content of section one...ending here")
		if len(matches) != 1 {
			t.Fatalf("want 1 match, got %d", len(matches))
		}
		if matches[0].StartLine != 7 || matches[0].EndLine != 8 {
			t.Errorf("want lines 7..8, got %d..%d", matches[0].StartLine, matches[0].EndLine)
		}
		if !strings.Contains(matches[0].Snippet, "content of section one") {
			t.Errorf("snippet missing start text: %q", matches[0].Snippet)
		}
	})

	t.Run("selection that does not match returns no matches", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionWithEllipsis(md, "nonexistent string")
		if len(matches) != 0 {
			t.Errorf("want 0 matches, got %d", len(matches))
		}
	})

	t.Run("ellipsis with empty side returns no matches", func(t *testing.T) {
		t.Parallel()
		if m := findSelectionWithEllipsis(md, "...end"); len(m) != 0 {
			t.Errorf("empty prefix should not match")
		}
		if m := findSelectionWithEllipsis(md, "start..."); len(m) != 0 {
			t.Errorf("empty suffix should not match")
		}
	})
}

func TestFindSelectionByTitle(t *testing.T) {
	t.Parallel()

	md := strings.Join([]string{
		"# Top",
		"",
		"intro",
		"",
		"## Section One",
		"",
		"section one body",
		"",
		"### Subsection",
		"",
		"subsection body",
		"",
		"## Section Two",
		"",
		"section two body",
	}, "\n")

	t.Run("matches h2 and captures full section", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionByTitle(md, "## Section One")
		if len(matches) != 1 {
			t.Fatalf("want 1 match, got %d", len(matches))
		}
		if matches[0].StartLine != 5 || matches[0].EndLine != 12 {
			t.Errorf("want lines 5..12, got %d..%d", matches[0].StartLine, matches[0].EndLine)
		}
		if !strings.Contains(matches[0].Snippet, "section one body") {
			t.Errorf("snippet should include section body")
		}
		if !strings.Contains(matches[0].Snippet, "### Subsection") {
			t.Errorf("snippet should include deeper subsection")
		}
	})

	t.Run("heading level mismatch yields no match", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionByTitle(md, "# Section One")
		if len(matches) != 0 {
			t.Errorf("want 0 matches for level-1 targeting level-2 heading, got %d", len(matches))
		}
	})

	t.Run("nonexistent heading yields no match", func(t *testing.T) {
		t.Parallel()
		if m := findSelectionByTitle(md, "## Section Three"); len(m) != 0 {
			t.Errorf("want 0 matches, got %d", len(m))
		}
	})

	t.Run("deeper heading captures only until next shallower", func(t *testing.T) {
		t.Parallel()
		matches := findSelectionByTitle(md, "### Subsection")
		if len(matches) != 1 {
			t.Fatalf("want 1 match, got %d", len(matches))
		}
		if matches[0].StartLine != 9 || matches[0].EndLine != 12 {
			t.Errorf("want lines 9..12, got %d..%d", matches[0].StartLine, matches[0].EndLine)
		}
	})
}

func TestBuildPreviewResult(t *testing.T) {
	t.Parallel()

	before := strings.Join([]string{
		"# Doc",
		"",
		"## Overview",
		"old intro",
		"",
		"## Next",
		"other content",
	}, "\n")

	t.Run("append uses note with line count", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("append", "DOC", "new paragraph\nsecond line", "", "", "", before)
		if res.PayloadLines != 2 {
			t.Errorf("want 2 payload lines, got %d", res.PayloadLines)
		}
		if !strings.Contains(res.Note, "append") {
			t.Errorf("note should mention append, got %q", res.Note)
		}
	})

	t.Run("overwrite warns about discard", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("overwrite", "DOC", "new doc content", "", "", "", before)
		if !strings.Contains(res.Note, "discarded") {
			t.Errorf("note should mention discard, got %q", res.Note)
		}
	})

	t.Run("replace_range with unique selection records single match", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("replace_range", "DOC", "new intro", "old intro", "", "", before)
		if res.MatchCount != 1 {
			t.Fatalf("want 1 match, got %d; note=%s", res.MatchCount, res.Note)
		}
		if !strings.Contains(res.Note, "exactly one") {
			t.Errorf("single-match note unexpected: %q", res.Note)
		}
	})

	t.Run("replace_range with no match surfaces warning", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("replace_range", "DOC", "new", "nonexistent", "", "", before)
		if res.MatchCount != 0 {
			t.Errorf("want 0 matches, got %d", res.MatchCount)
		}
		if !strings.Contains(res.Note, "did not match") {
			t.Errorf("expected not-matched note, got %q", res.Note)
		}
	})

	t.Run("selection-by-title finds section", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("replace_range", "DOC", "new section body", "", "## Overview", "", before)
		if res.MatchCount != 1 {
			t.Fatalf("want 1 match, got %d; note=%s", res.MatchCount, res.Note)
		}
	})

	t.Run("replace_all with multi-match emits tailored note", func(t *testing.T) {
		t.Parallel()
		md := "foo bar\nbar foo\nfoo again"
		res := buildPreviewResult("replace_all", "DOC", "qux", "foo", "", "", md)
		if res.MatchCount != 3 {
			t.Fatalf("want 3 matches, got %d", res.MatchCount)
		}
		if !strings.Contains(res.Note, "replace_all would apply to all") {
			t.Errorf("note should call out replace_all semantics, got %q", res.Note)
		}
	})

	t.Run("new_title is propagated to preview", func(t *testing.T) {
		t.Parallel()
		res := buildPreviewResult("append", "DOC", "body", "", "", "Renamed Doc", before)
		if res.NewTitle != "Renamed Doc" {
			t.Errorf("want NewTitle=Renamed Doc, got %q", res.NewTitle)
		}
	})
}

func TestFindSelectionWithEllipsisSameLineOrdering(t *testing.T) {
	t.Parallel()

	// "B...A" should not match a line where A appears before B.
	md := "the quick brown fox"
	matches := findSelectionWithEllipsis(md, "fox...quick")
	if len(matches) != 0 {
		t.Errorf("expected no match for reversed same-line ellipsis, got %d", len(matches))
	}

	// Same line, correct order, still works.
	matches = findSelectionWithEllipsis(md, "quick...fox")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for same-line quick...fox, got %d", len(matches))
	}
	if matches[0].StartLine != 1 || matches[0].EndLine != 1 {
		t.Errorf("want 1..1, got %d..%d", matches[0].StartLine, matches[0].EndLine)
	}
}

func TestFindSelectionByTitleIgnoresFencedHeadings(t *testing.T) {
	t.Parallel()

	md := strings.Join([]string{
		"# Real H1",
		"",
		"```markdown",
		"## Fake Heading",
		"```",
		"",
		"## Real H2",
		"body",
	}, "\n")

	// Fenced "## Fake Heading" must not match.
	matches := findSelectionByTitle(md, "## Fake Heading")
	if len(matches) != 0 {
		t.Errorf("fenced heading should not match, got %d match(es)", len(matches))
	}

	// And the fenced line must not be treated as a section terminator for
	// outer headings either — searching for H1 should capture everything.
	matches = findSelectionByTitle(md, "# Real H1")
	if len(matches) != 1 {
		t.Fatalf("want 1 match for H1, got %d", len(matches))
	}
	if !strings.Contains(matches[0].Snippet, "body") {
		t.Errorf("H1 section should reach through fenced code to final body, got snippet:\n%s", matches[0].Snippet)
	}
}
