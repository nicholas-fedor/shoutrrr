package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd/docs"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd/generate"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd/send"
	"github.com/nicholas-fedor/shoutrrr/shoutrrr/cmd/verify"
)

var cobraCmd = &cobra.Command{
	Use:   "shoutrrr",
	Short: "Shoutrrr CLI",
}

// init registers the subcommands and the version string on the root command.
// The subcommands read unset flags from SHOUTRRR_ environment variables
// themselves, before cobra checks required flags.
func init() {
	cobraCmd.AddCommand(verify.Cmd)
	cobraCmd.AddCommand(generate.Cmd)
	cobraCmd.AddCommand(send.Cmd)
	cobraCmd.AddCommand(docs.Cmd)

	cobraCmd.Version = meta.GetMetaStr()
}

func main() {
	if err := cobraCmd.Execute(); err != nil {
		os.Exit(cmd.ExUsage)
	}
}
