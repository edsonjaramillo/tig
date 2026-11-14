package cmd

import (
	"github.com/spf13/cobra"
)

// newKeepEditCmd is the old name of `tig config edit`, kept as a hidden
// alias: the file it edits is the whole tig config, not just the keep list.
func newKeepEditCmd(a *app) *cobra.Command {
	c := newConfigEditCmd(a)
	c.Deprecated = "use `tig config edit`"
	return c
}
