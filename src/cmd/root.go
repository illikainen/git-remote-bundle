package cmd

import (
	"fmt"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/metadata"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootOpts struct {
	verbosity string
}

var rootCmd = &cobra.Command{
	Use:     metadata.Name(),
	Version: metadata.Version(),
	PreRun: func(cmd *cobra.Command, args []string) {
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

	flags.StringVarP(
		&rootOpts.verbosity,
		"verbosity",
		"",
		"info",
		fmt.Sprintf("Log level (%s)", strings.Join(levels, ", ")),
	)
}

func rootPreRun(_ *cobra.Command, _ []string) error {
	verbosity, err := log.ParseLevel(rootOpts.verbosity)
	if err != nil {
		return err
	}

	log.SetLevel(verbosity)
	return nil
}
