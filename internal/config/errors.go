package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/edsonjaramillo/tig/internal/keep"
)

// section describes a Go type decoded from the config in user terms.
type section struct {
	name string // subject for type errors, e.g. "keep.repos"
	in   string // location for unknown keys, e.g. "in keep"
	want string // what it must be, e.g. "a list of repos"
	hint string // optional fix suggestion
	keys []string
}

// sections is keyed by Go type name as yaml.v3 prints it in decode errors.
// Keys are derived from the types themselves so renames cannot silently
// break the lookup.
var sections = map[string]section{
	typeOf(file{}).String():                {in: "at the top level", want: "a mapping", keys: yamlKeys(typeOf(file{}))},
	typeOf(keep.KeepList{}).String():       {name: "keep", in: "in keep", want: "a mapping with protocol and repos", keys: yamlKeys(typeOf(keep.KeepList{}))},
	keptRepoSpec.String():                  {in: "in a keep.repos entry", want: "a mapping with repo, destination, ...", keys: yamlKeys(keptRepoSpec)},
	reflect.SliceOf(keptRepoSpec).String(): {name: "keep.repos", want: "a list of repos", hint: `start each repo with "- ", e.g. "- repo: owner/repo"`},
	typeOf([]string{}).String():            {name: "post_clone/post_pull", want: "a list of commands", hint: "e.g. [task build] or one \"- command\" per line"},
	typeOf("").String():                    {want: "a single value"},
}

// keptRepoSpec is the (unexported) element type of keep.KeepList.Repos.
var keptRepoSpec = fieldType(keep.KeepList{}, "Repos").Elem()

func typeOf(v any) reflect.Type { return reflect.TypeOf(v) }

func fieldType(v any, name string) reflect.Type {
	f, ok := reflect.TypeOf(v).FieldByName(name)
	if !ok {
		panic(fmt.Sprintf("%T has no field %s", v, name))
	}
	return f.Type
}

var (
	unknownField = regexp.MustCompile(`^line (\d+): field (\S+) not found in type (\S+)$`)
	wrongType    = regexp.MustCompile("^line (\\d+): cannot unmarshal !!(\\w+)(?: `.*`)? into (\\S+)$")
)

var tagNames = map[string]string{
	"map": "a mapping", "seq": "a list", "str": "a string", "int": "a number",
	"float": "a number", "bool": "a boolean", "null": "empty", "timestamp": "a timestamp",
}

// humanize rewrites yaml.v3 decode errors, which mention Go types, into
// messages about the config file's keys.
func humanize(err error) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}
	msgs := make([]string, len(te.Errors))
	for i, e := range te.Errors {
		msgs[i] = humanizeLine(e)
	}
	return errors.New(strings.Join(msgs, "\n"))
}

func humanizeLine(msg string) string {
	if m := unknownField.FindStringSubmatch(msg); m != nil {
		s, ok := sections[m[3]]
		if !ok {
			return fmt.Sprintf("line %s: unknown key %q", m[1], m[2])
		}
		out := fmt.Sprintf("line %s: unknown key %q %s", m[1], m[2], s.in)
		if len(s.keys) > 0 {
			out += " (known keys: " + strings.Join(s.keys, ", ") + ")"
		}
		return out
	}
	if m := wrongType.FindStringSubmatch(msg); m != nil {
		got := tagNames[m[2]]
		if got == "" {
			got = m[2]
		}
		s, ok := sections[m[3]]
		if !ok {
			return fmt.Sprintf("line %s: unexpected %s", m[1], got)
		}
		where := ""
		if s.name != "" {
			where = s.name + " "
		}
		out := fmt.Sprintf("line %s: %smust be %s, not %s", m[1], where, s.want, got)
		if s.hint != "" {
			out += " (" + s.hint + ")"
		}
		return out
	}
	return msg
}

// yamlKeys lists the decodable yaml keys of a struct type.
func yamlKeys(t reflect.Type) []string {
	var keys []string
	for i := range t.NumField() {
		tag, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if tag != "" && tag != "-" {
			keys = append(keys, tag)
		}
	}
	return keys
}
