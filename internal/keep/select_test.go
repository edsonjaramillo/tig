package keep

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestSelect(t *testing.T) {
	repos := []KeptRepo{
		{Name: "me/tig"},
		{Name: "me/dotfiles"},
		{Name: "you/dotfiles"},
		{Name: "org/api"},
	}
	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr []string // substrings of the error; nil means no error
	}{
		{"all", nil, []string{"me/tig", "me/dotfiles", "you/dotfiles", "org/api"}, nil},
		{"full name", []string{"you/dotfiles"}, []string{"you/dotfiles"}, nil},
		{"case-insensitive", []string{"ME/TIG"}, []string{"me/tig"}, nil},
		{"unique short", []string{"api", "tig"}, []string{"me/tig", "org/api"}, nil},
		{"dedupe", []string{"tig", "me/tig"}, []string{"me/tig"}, nil},
		{"ambiguous is an error", []string{"dotfiles", "tig"}, nil, []string{`"dotfiles" is ambiguous: use one of me/dotfiles, you/dotfiles`}},
		{"unknown is an error", []string{"nope", "org/api"}, nil, []string{`"nope" matches no kept repo`}},
		{"all problems reported", []string{"nope", "dotfiles", "tig"}, nil, []string{`"nope"`, `"dotfiles"`}},
	}
	if _, err := Select(nil, []string{"tig"}); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty keep list: err = %v, want ErrEmpty", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(repos, tt.args)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("err = %v", err)
			}
			for _, w := range tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), w) {
					t.Fatalf("err = %v, want containing %q", err, w)
				}
			}
			var names []string
			for _, r := range got {
				names = append(names, r.Name)
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("selected %v, want %v", names, tt.want)
			}
		})
	}
}
