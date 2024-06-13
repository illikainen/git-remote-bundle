package main

import (
	"os"

	"github.com/illikainen/git-remote-bundle/src/cmd"
	"github.com/illikainen/git-remote-bundle/src/git"

	"github.com/fatih/color"
	"github.com/illikainen/go-utils/src/ensure"
	"github.com/illikainen/go-utils/src/logging"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
)

func main() {
	color.NoColor = !isatty.IsTerminal(os.Stderr.Fd())

	log.SetOutput(os.Stderr)
	log.SetFormatter(&logging.SanitizedTextFormatter{})
	log.SetLevel(git.LogLevel())

	ensure.Unprivileged()

	// The GIT_DIR and GIT_EXEC_PATH environment variables are set when Git
	// executes the helper.  We use the environment to determine whether to
	// run as a CLI tool or as a remote helper.
	//
	// Behavior observed with Git 2.39.2.
	if os.Getenv("GIT_DIR") != "" {
		err := git.Communicate()
		if err != nil {
			log.Tracef("%+v", err)
			log.Fatalf("%s", err)
		}
	} else {
		err := cmd.Command().Execute()
		if err != nil {
			log.Tracef("%+v", err)
			log.Fatalf("%s", err)
		}
	}
}
