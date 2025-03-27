package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"
	"github.com/illikainen/git-remote-bundle/src/sandbox"

	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/flag"
	"github.com/illikainen/go-utils/src/logging"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootOpts struct {
	url       flag.URL
	cacheDir  flag.Path
	verbosity logging.LogLevel
}

var rootCmd = &cobra.Command{
	Use:     metadata.Name(),
	Version: metadata.Version(),
	Args:    rootArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := rootPreRun(cmd, args)
		if err != nil {
			log.Tracef("%+v", err)
			log.Fatalf("%s", err)
		}
	},
	Run: cobrax.Run(rootRun),
}

func Command() *cobra.Command {
	return rootCmd
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.SortFlags = false

	flags.Var(&rootOpts.url, "url", "URL")
	lo.Must0(flags.MarkHidden("url"))

	rootOpts.cacheDir.State = flag.MustBeDir
	rootOpts.cacheDir.Mode = flag.ReadWriteMode
	flags.Var(&rootOpts.cacheDir, "cache-dir", "Cache directory")

	levels := []string{}
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	flags.Var(&rootOpts.verbosity, "verbosity", fmt.Sprintf("Verbosity (%s)", strings.Join(levels, ", ")))
}

func rootPreRun(cmd *cobra.Command, _ []string) error {
	flags := cmd.Flags()

	cacheDir, err := git.CacheDir()
	if err != nil {
		return err
	}
	if err := flag.SetFallback(flags, "cache-dir", cacheDir); err != nil {
		return err
	}

	verbosity, err := git.Verbosity()
	if err != nil {
		return err
	}
	if err := flag.SetFallback(flags, "verbosity", verbosity); err != nil {
		return err
	}

	return sandbox.Exec(cmd.CalledAs(), flags)
}

func rootArgs(cmd *cobra.Command, args []string) error {
	err := cobra.ExactArgs(2)(cmd, args)
	if err != nil {
		return err
	}

	flags := cmd.Flags()
	uri := flags.Lookup("url")
	err = uri.Value.Set(args[1])
	if err != nil {
		return err
	}

	return nil
}

func rootRun(_ *cobra.Command, _ []string) error {
	// The GIT_DIR and GIT_EXEC_PATH environment variables are set when Git
	// executes the helper.
	//
	// Behavior observed with Git 2.39.2.
	if os.Getenv("GIT_DIR") == "" || os.Getenv("GIT_EXEC_PATH") == "" {
		return errors.Errorf("not invoked as a remote helper by git")
	}
	return git.Communicate(rootOpts.url.Value, rootOpts.cacheDir.Value)
}
