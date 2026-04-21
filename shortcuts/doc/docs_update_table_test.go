// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT
package doc

import (
	"testing"
)

// ── A1 notation parsing tests ──

func TestParseColLetter(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"A", 0, false},
		{"B", 1, false},
		{"Z", 25, false},
		{"AA", 26, false},
		{"AB", 27, false},
		{"a", 0, false},   // case-insensitive
		{"0", 0, false},   // special: before-first
		{"-1", -1, false}, // special: append
		{"", 0, true},
		{"2", 0, true},  // numeric (not 0/-1) is invalid
		{"A1", 0, true}, // contains digit
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseColLetter(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseColLetter(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parseColLetter(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseA1Cell(t *testing.T) {
	tests := []struct {
		input   string
		wantRow int
		wantCol int
		wantErr bool
	}{
		{"A1", 0, 0, false},
		{"B3", 2, 1, false},
		{"AA1", 0, 26, false},
		{"Z26", 25, 25, false},
		{"c4", 3, 2, false}, // case-insensitive
		{"1A", 0, 0, true},  // wrong order
		{"A0", 0, 0, true},  // row must be >= 1
		{"A", 0, 0, true},   // missing row
		{"1", 0, 0, true},   // missing col
		{"", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			row, col, err := parseA1Cell(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseA1Cell(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if err == nil {
				if row != tt.wantRow || col != tt.wantCol {
					t.Errorf("parseA1Cell(%q) = (%d,%d), want (%d,%d)",
						tt.input, row, col, tt.wantRow, tt.wantCol)
				}
			}
		})
	}
}

func TestParseA1Range(t *testing.T) {
	tests := []struct {
		input                    string
		wantRowStart, wantRowEnd int // half-open
		wantColStart, wantColEnd int // half-open
		wantErr                  bool
	}{
		{"A1:C3", 0, 3, 0, 3, false}, // inclusive A1:C3 → half-open [0,3)×[0,3)
		{"B2:D5", 1, 5, 1, 4, false},
		{"A1:A1", 0, 1, 0, 1, false}, // single cell
		{"C3:A1", 0, 0, 0, 0, true},  // start after end
		{"A1:", 0, 0, 0, 0, true},
		{"A1", 0, 0, 0, 0, true}, // missing colon
		{"", 0, 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			rs, re, cs, ce, err := parseA1Range(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseA1Range(%q) err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if err == nil {
				if rs != tt.wantRowStart || re != tt.wantRowEnd || cs != tt.wantColStart || ce != tt.wantColEnd {
					t.Errorf("parseA1Range(%q) = (%d,%d,%d,%d), want (%d,%d,%d,%d)",
						tt.input, rs, re, cs, ce,
						tt.wantRowStart, tt.wantRowEnd, tt.wantColStart, tt.wantColEnd)
				}
			}
		})
	}
}

// ── Command routing tests ──

func TestIsTableCommand(t *testing.T) {
	tableCommands := []string{
		"table_insert_rows", "table_insert_cols",
		"table_delete_rows", "table_delete_cols",
		"table_merge_cells", "table_unmerge_cells",
		"table_update_property",
	}
	for _, cmd := range tableCommands {
		if !isTableCommand(cmd) {
			t.Errorf("isTableCommand(%q) = false, want true", cmd)
		}
	}
	nonTableCommands := []string{"str_replace", "block_replace", "overwrite", "append", "table_batch", ""}
	for _, cmd := range nonTableCommands {
		if isTableCommand(cmd) {
			t.Errorf("isTableCommand(%q) = true, want false", cmd)
		}
	}
}
