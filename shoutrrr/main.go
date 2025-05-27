package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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

func init() {
	viper.AutomaticEnv()
	cobraCmd.AddCommand(verify.Cmd)
	cobraCmd.AddCommand(generate.Cmd)
	cobraCmd.AddCommand(send.Cmd)
	cobraCmd.AddCommand(docs.Cmd)

	cobraCmd.Version = fmt.Sprintf(
		"%s (Built on %s from Git SHA %s)",
		meta.GetVersion(),
		meta.GetDate(),
		meta.GetCommit(),
	)
}

func main() {
	if err := cobraCmd.Execute(); err != nil {
		os.Exit(cmd.ExUsage)
	}
}
