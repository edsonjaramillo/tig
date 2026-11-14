package git

import "testing"

func TestIsURL(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"owner/repo", false},
		{"gitlab.com/owner/repo", false},
		{"https://github.com/owner/repo.git", true},
		{"ssh://git@github.com/owner/repo", true},
		{"file:///tmp/owner/repo.git", true},
		{"git@github.com:owner/repo.git", true},
		{"github.com:owner/repo", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsURL(tt.in); got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseShorthand(t *testing.T) {
	tests := []struct {
		in      string
		want    Remote
		wantErr bool
	}{
		{"owner/repo", Remote{Host: "github.com", Path: "owner/repo"}, false},
		{"owner/repo.git", Remote{Host: "github.com", Path: "owner/repo"}, false},
		{"GitLab.com/someone/thing", Remote{Host: "gitlab.com", Path: "someone/thing"}, false},
		{"repo", Remote{}, true},
		{"a/b/c/d", Remote{}, true},
		{"owner//repo", Remote{}, true},
		{"owner/re po", Remote{}, true},
		{"../repo", Remote{}, true},
	}
	for _, tt := range tests {
		got, err := ParseShorthand(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseShorthand(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseShorthand(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseURL(t *testing.T) {
	tests := []struct {
		in        string
		key       string
		name      string
		transport string
		wantErr   bool
	}{
		{"https://github.com/Owner/Repo.git", "github.com/owner/repo", "Owner/Repo", "https", false},
		{"https://user:tok@github.com/owner/repo/", "github.com/owner/repo", "owner/repo", "https", false},
		{"http://example.com/owner/repo", "example.com/owner/repo", "owner/repo", "https", false},
		{"ssh://git@github.com:22/owner/repo.git", "github.com/owner/repo", "owner/repo", "ssh", false},
		{"git@github.com:owner/repo.git", "github.com/owner/repo", "owner/repo", "ssh", false},
		{"github.com:owner/repo", "github.com/owner/repo", "owner/repo", "ssh", false},
		{"https://gitlab.com/group/sub/repo.git", "gitlab.com/group/sub/repo", "sub/repo", "https", false},
		{"file:///tmp/x/owner/repo.git", "/tmp/x/owner/repo", "owner/repo", "file", false},
		{"https://github.com/", "", "", "", true},
		{"owner/repo", "", "", "", true},
		{"/abs/path", "", "", "", true},
	}
	for _, tt := range tests {
		got, err := ParseURL(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseURL(%q) err = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if got.Key() != tt.key || got.Name() != tt.name || got.Transport() != tt.transport {
			t.Errorf("ParseURL(%q) key=%q name=%q transport=%q, want %q %q %q",
				tt.in, got.Key(), got.Name(), got.Transport(), tt.key, tt.name, tt.transport)
		}
	}
}

func TestSameRepoAcrossProtocols(t *testing.T) {
	a, _ := ParseURL("https://github.com/edsonjaramillo/tig.git")
	b, _ := ParseURL("git@github.com:edsonjaramillo/tig.git")
	s, _ := ParseShorthand("edsonjaramillo/tig")
	if a.Key() != b.Key() || a.Key() != s.Key() {
		t.Fatalf("keys differ: %q %q %q", a.Key(), b.Key(), s.Key())
	}
	if s.HTTPSURL() != "https://github.com/edsonjaramillo/tig.git" {
		t.Errorf("HTTPSURL = %q", s.HTTPSURL())
	}
	if s.SSHURL() != "git@github.com:edsonjaramillo/tig.git" {
		t.Errorf("SSHURL = %q", s.SSHURL())
	}
}
