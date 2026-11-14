package keep

import (
	"errors"
	"fmt"
	"strings"
)

// Select returns the kept repos matching names, in keep list order. A name
// matches a full repo name (owner/repo) or, when unique, just the repo part.
// No names selects every kept repo. Every unknown or ambiguous name is
// reported in the returned error, and nothing is selected.
func Select(repos []KeptRepo, names []string) ([]KeptRepo, error) {
	if len(names) == 0 {
		return repos, nil
	}
	chosen := make([]bool, len(repos))
	var errs []error
	for _, name := range names {
		idx, err := match(repos, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		chosen[idx] = true
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	var selected []KeptRepo
	for i, c := range chosen {
		if c {
			selected = append(selected, repos[i])
		}
	}
	return selected, nil
}

func match(repos []KeptRepo, name string) (int, error) {
	for i, r := range repos {
		if strings.EqualFold(r.Name, name) {
			return i, nil
		}
	}
	if !strings.Contains(name, "/") {
		var hits []int
		for i, r := range repos {
			if strings.EqualFold(r.ShortName(), name) {
				hits = append(hits, i)
			}
		}
		switch len(hits) {
		case 1:
			return hits[0], nil
		case 0:
		default:
			full := make([]string, len(hits))
			for j, h := range hits {
				full[j] = repos[h].Name
			}
			return -1, fmt.Errorf("%q is ambiguous: use one of %s", name, strings.Join(full, ", "))
		}
	}
	return -1, fmt.Errorf("%q matches no kept repo (see `tig keep list`)", name)
}

// ShortName returns the repo part of the repo name, e.g. "tig" for
// "owner/tig".
func (r KeptRepo) ShortName() string {
	_, short, _ := strings.Cut(r.Name, "/")
	return short
}
