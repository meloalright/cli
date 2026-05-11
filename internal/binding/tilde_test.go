// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package binding

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// setFakeOSHome makes vfs.UserHomeDir resolve to dir (or fail, when dir is "")
// regardless of the host OS. os.UserHomeDir reads HOME on Unix and USERPROFILE
// on Windows; setting only one would leave the test sensitive to the runner's
// real environment on the other platform.
func setFakeOSHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestOpenClawHome(t *testing.T) {
	homeDir := t.TempDir()
	explicit := t.TempDir()
	setFakeOSHome(t, homeDir)

	tests := []struct {
		name        string
		openclawEnv string
		want        string
	}{
		{"unset falls back to OS home", "", homeDir},
		{"undefined literal treated as unset", "undefined", homeDir},
		{"null literal treated as unset", "null", homeDir},
		{"whitespace-only treated as unset", "   ", homeDir},
		{"explicit absolute path used verbatim", explicit, explicit},
		{"explicit absolute path is trimmed", "  " + explicit + "  ", explicit},
		{"bare tilde resolves to OS home", "~", homeDir},
		{"tilde-prefixed value recurses through OS home", "~/custom", filepath.Join(homeDir, "custom")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OPENCLAW_HOME", tc.openclawEnv)
			got := openClawHome()
			if got != tc.want {
				t.Errorf("openClawHome() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOpenClawHome_RelativeIsAbsolutized(t *testing.T) {
	// A relative OPENCLAW_HOME is resolved against the process cwd, mirroring
	// Node's path.resolve behaviour in OpenClaw.
	t.Setenv("OPENCLAW_HOME", filepath.FromSlash("relative/dir"))
	got := openClawHome()

	if !filepath.IsAbs(got) {
		t.Fatalf("openClawHome() = %q, want absolute path", got)
	}
	wantSuffix := filepath.FromSlash("relative/dir")
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("openClawHome() = %q, want suffix %q", got, wantSuffix)
	}
}

func TestOpenClawHome_TildeOpenClawHomeWithoutOSHome(t *testing.T) {
	// Pins the deliberate divergence from OpenClaw: a tilde-form
	// OPENCLAW_HOME with no OS home yields "" rather than falling back
	// to cwd. See osHome / openClawHome docstrings.
	setFakeOSHome(t, "")
	t.Setenv("OPENCLAW_HOME", "~/custom")
	if got := openClawHome(); got != "" {
		t.Errorf("openClawHome() = %q, want empty (OS home unresolvable)", got)
	}
}

func TestOpenClawHome_NoFallbackToCwd(t *testing.T) {
	// Pins the deliberate divergence from OpenClaw's
	// resolveRequiredHomeDir, which falls back to process.cwd() when
	// every env candidate is unresolvable. We return "" instead so the
	// downstream absolute-path audit can reject the still-tilded input
	// rather than silently routing it through cwd. See osHome docstring.
	setFakeOSHome(t, "")
	t.Setenv("OPENCLAW_HOME", "")
	got := openClawHome()
	if got != "" {
		t.Errorf("openClawHome() = %q, want empty (must not fall back to cwd)", got)
	}
}

func TestExpandTildePath(t *testing.T) {
	fakeHome := t.TempDir()
	absFixture := filepath.Join(fakeHome, "abs.json")
	setFakeOSHome(t, fakeHome)
	t.Setenv("OPENCLAW_HOME", "")

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"bare tilde", "~", fakeHome},
		{"tilde slash", "~/", fakeHome},
		{"tilde with file", "~/secret.json", filepath.Join(fakeHome, "secret.json")},
		{"tilde with nested path", "~/.openclaw/secret.json", filepath.Join(fakeHome, ".openclaw/secret.json")},
		{"absolute unchanged", absFixture, absFixture},
		{"relative unchanged", "foo/bar", "foo/bar"},
		{"dot relative unchanged", "../foo", "../foo"},
		{"tilde user form unchanged", "~root/foo", "~root/foo"},
		{"tilde without separator unchanged", "~foo", "~foo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandTildePath(tc.in)
			if got != tc.want {
				t.Errorf("expandTildePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandTildePath_RespectsOpenClawHome(t *testing.T) {
	homeDir := t.TempDir()
	clawHome := t.TempDir()
	setFakeOSHome(t, homeDir)
	t.Setenv("OPENCLAW_HOME", clawHome)

	got := expandTildePath("~/secret.json")
	want := filepath.Join(clawHome, "secret.json")
	if got != want {
		t.Errorf("expandTildePath(%q) = %q, want %q (should use OPENCLAW_HOME)", "~/secret.json", got, want)
	}
	if got == filepath.Join(homeDir, "secret.json") {
		t.Errorf("expandTildePath unexpectedly used OS home %q instead of OPENCLAW_HOME %q", homeDir, clawHome)
	}
}

func TestExpandTildePath_HomeUnavailable(t *testing.T) {
	// Pins the deliberate divergence from OpenClaw's read pipeline
	// (which falls back to cwd via resolveRequiredHomeDir). We pass the
	// input through unchanged so the downstream audit surfaces a clean
	// "path must be absolute" error rather than silently producing a
	// cwd-relative path. See osHome / openClawHome docstrings.
	setFakeOSHome(t, "")
	t.Setenv("OPENCLAW_HOME", "")
	got := expandTildePath("~/foo")
	if got != "~/foo" {
		t.Errorf("expandTildePath returned %q on UserHomeDir failure, want passthrough %q", got, "~/foo")
	}
}

// TestOpenClawHome_OSHomeNormalization pins the HOME → USERPROFILE → stdlib
// fallback chain, including OpenClaw's sentinel normalisation (the literals
// "undefined" / "null" / blank are treated as unset at every step). Without
// this, an accidentally-stringified env (`HOME=undefined`) would silently
// resolve `~/secret` to `<cwd>/undefined/secret`.
func TestOpenClawHome_OSHomeNormalization(t *testing.T) {
	userProfileDir := t.TempDir()

	tests := []struct {
		name        string
		home        string
		userProfile string
		want        string
	}{
		{"HOME=undefined falls through to USERPROFILE", "undefined", userProfileDir, userProfileDir},
		{"HOME=null falls through to USERPROFILE", "null", userProfileDir, userProfileDir},
		{"HOME=whitespace falls through to USERPROFILE", "   ", userProfileDir, userProfileDir},
		{"HOME wins over USERPROFILE when both are valid", t.TempDir(), userProfileDir, ""}, // want set below
	}
	tests[3].want = tests[3].home

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", tc.home)
			t.Setenv("USERPROFILE", tc.userProfile)
			t.Setenv("OPENCLAW_HOME", "")
			if got := openClawHome(); got != tc.want {
				t.Errorf("openClawHome() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExpandTildePath_BackslashPreservedOnPOSIX pins that `~\secret.json`
// expands by replacing only the `~` byte, leaving the backslash literally
// as part of the filename — matching OpenClaw's regex-replace semantics
// (`/^~(?=$|[\\/])/`) rather than going through filepath.Join (which would
// drop the backslash on POSIX). On Windows backslash is a real separator,
// so the literal-byte invariant doesn't apply.
func TestExpandTildePath_BackslashPreservedOnPOSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows; invariant only applies on POSIX")
	}
	fakeHome := t.TempDir()
	setFakeOSHome(t, fakeHome)
	t.Setenv("OPENCLAW_HOME", "")

	got := expandTildePath(`~\secret.json`)
	want := fakeHome + `\secret.json`
	if got != want {
		t.Errorf("expandTildePath(%q) = %q, want %q (backslash should be preserved as filename byte)", `~\secret.json`, got, want)
	}
}
