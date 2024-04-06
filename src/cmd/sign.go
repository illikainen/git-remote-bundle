package cmd

import (
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var signOpts struct {
	cryptor.SignOptions
}

var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a bundle",
	Run:   cobrax.Run(signRun),
}

func init() {
	flags := cryptor.SignFlags(cryptor.SignConfig{
		Options: &signOpts.SignOptions,
	})
	signCmd.Flags().AddFlagSet(flags)
	lo.Must0(signCmd.MarkFlagRequired("input"))
	lo.Must0(signCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(signCmd)
}

func signRun(_ *cobra.Command, _ []string) error {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{signOpts.Input}
		rw := []string{signOpts.Output}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		// Required to mount the file in the sandbox.
		f, err := os.Create(signOpts.Output)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}

		return sandbox.Run(sandbox.Options{
			Args: os.Args,
			RO:   ro,
			RW:   rw,
		})
	}

	keys, err := git.ReadKeyrings()
	if err != nil {
		return err
	}

	bundle, err := blob.New(blob.Config{Path: signOpts.Output, Keys: keys})
	if err != nil {
		return err
	}

	err = bundle.Import(signOpts.Input, nil)
	if err != nil {
		return err
	}

	err = bundle.Sign()
	if err != nil {
		return err
	}

	log.Infof("successfully wrote signed blob to %s", signOpts.Output)
	return nil
}
