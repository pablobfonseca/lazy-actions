package config

import (
	"errors"
	"testing"
)

func TestParseGitHubSlug(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"git@github.com:upscopeio/lazy-actions.git", "upscopeio/lazy-actions", false},
		{"git@github.com:upscopeio/lazy-actions", "upscopeio/lazy-actions", false},
		{"https://github.com/upscopeio/lazy-actions.git", "upscopeio/lazy-actions", false},
		{"https://github.com/upscopeio/lazy-actions", "upscopeio/lazy-actions", false},
		{"ssh://git@github.com/upscopeio/lazy-actions.git", "upscopeio/lazy-actions", false},
		{"https://github.com/upscopeio/lazy-actions/", "upscopeio/lazy-actions", false},
		{"https://gitlab.com/foo/bar.git", "", true},
		{"git@example.com:foo/bar.git", "", true},
		{"https://github.com/onlyowner", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := parseGitHubSlug(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got %q", tc.in, got)
				continue
			}
			if !errors.Is(err, ErrNotAutoDetectable) {
				t.Errorf("%q: expected ErrNotAutoDetectable, got %v", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}
