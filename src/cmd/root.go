package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-netutils/src/sshx"
	"github.com/illikainen/go-utils/src/process"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var rootOpts struct {
	Sandbox   sandbox.Sandbox
	sandbox   string
	url       string
	cacheDir  string
	verbosity string
}

var rootCmd = &cobra.Command{
	Use:     metadata.Name(),
	Version: metadata.Version(),
	Args:    cobra.ExactArgs(2),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		err := rootPreRun(cmd, args)
		if err != nil {
			log.Tracef("%+v", err)
			log.Fatalf("%s", err)
		}
	},
	RunE: rootRun,
}

func Command() *cobra.Command {
	return rootCmd
}

func init() {
	flags := rootCmd.PersistentFlags()
	flags.SortFlags = false

	flags.StringVarP(&rootOpts.url, "url", "", "", "URL")
	lo.Must0(flags.MarkHidden("url"))

	flags.StringVarP(&rootOpts.cacheDir, "cache-dir", "", lo.Must1(git.CacheDir()), "Cache directory")

	levels := []string{}
	for _, level := range log.AllLevels {
		levels = append(levels, level.String())
	}
	flags.StringVarP(&rootOpts.verbosity, "verbosity", "", lo.Must1(git.Verbosity()),
		fmt.Sprintf("Verbosity (%s)", strings.Join(levels, ", ")))

	flags.StringVarP(&rootOpts.sandbox, "sandbox", "", "", "Sandbox backend")
}

func rootPreRun(_ *cobra.Command, _ []string) error {
	level, err := log.ParseLevel(rootOpts.verbosity)
	if err != nil {
		return err
	}
	log.SetLevel(level)

	backend, err := sandbox.Backend(rootOpts.sandbox)
	if err != nil {
		return err
	}

	ro, rw, err := git.SandboxPaths()
	if err != nil {
		return err
	}

	switch backend {
	case sandbox.BubblewrapSandbox:
		rootOpts.Sandbox, err = sandbox.NewBubblewrap(&sandbox.BubblewrapOptions{
			ReadOnlyPaths:    ro,
			ReadWritePaths:   rw,
			Tmpfs:            true,
			Devtmpfs:         true,
			Procfs:           true,
			AllowCommonPaths: true,
			Stdin:            io.Reader(nil),
			Stdout:           process.LogrusOutput,
			Stderr:           process.LogrusOutput,
		})
		if err != nil {
			return err
		}
	case sandbox.NoSandbox:
		rootOpts.Sandbox, err = sandbox.NewNoop()
		if err != nil {
			return err
		}
	}

	return nil
}

// This function is reached when invoked through `git` or if the user manually
// executes `git-remote-bundle` on the CLI without specifying a subcommand.
func rootRun(_ *cobra.Command, args []string) error {
	// The GIT_DIR and GIT_EXEC_PATH environment variables are set when Git
	// executes the helper.
	if os.Getenv("GIT_DIR") == "" || os.Getenv("GIT_EXEC_PATH") == "" {
		return errors.Errorf("not invoked as a remote helper by git")
	}

	ro, rw, err := sshx.SandboxPaths()
	if err != nil {
		return err
	}

	uri, err := url.Parse(args[1])
	if err != nil {
		return err
	}

	if uri.Scheme == "file" {
		rw = append(rw, uri.Path)
	}

	err = rootOpts.Sandbox.AddReadOnlyPath(ro...)
	if err != nil {
		return err
	}

	err = rootOpts.Sandbox.AddReadWritePath(rw...)
	if err != nil {
		return err
	}

	rootOpts.Sandbox.SetStdin(os.Stdin)
	rootOpts.Sandbox.SetStdout(process.UnsafeByteOutput)
	rootOpts.Sandbox.SetShareNet(true)

	err = rootOpts.Sandbox.Confine()
	if err != nil {
		return err
	}

	return git.Communicate(uri, rootOpts.cacheDir)
}
