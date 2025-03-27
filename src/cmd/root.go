package cmd

import (
	"fmt"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/metadata"
	"github.com/illikainen/git-remote-bundle/src/sandbox"

	"github.com/illikainen/go-utils/src/flag"
	"github.com/illikainen/go-utils/src/logging"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootOpts struct {
	verbosity logging.LogLevel
}

var rootCmd = &cobra.Command{
	Use:     metadata.Name(),
	Version: metadata.Version(),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := rootPreRun(cmd, args)
		if err != nil {
			log.Tracef("%+v", err)
			log.Fatalf("%s", err)
		}
	},
}

func Command() *cobra.Command {
	return rootCmd
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.SortFlags = false

	levels := []string{}
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}

	flags.Var(&rootOpts.verbosity, "verbosity", fmt.Sprintf("Verbosity (%s)", strings.Join(levels, ", ")))
}

func rootPreRun(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()

	if err := flag.SetFallback(flags, "verbosity", "info"); err != nil {
		return err
	}

	return sandbox.Exec(cmd.CalledAs(), flags)
}
