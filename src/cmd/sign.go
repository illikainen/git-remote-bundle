package cmd

import (
	"github.com/illikainen/git-remote-bundle/src/git"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
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
