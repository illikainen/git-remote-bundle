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

var encryptOpts struct {
	cryptor.EncryptOptions
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt and sign a bundle",
	Run:   cobrax.Run(encryptRun),
}

func init() {
	flags := cryptor.EncryptFlags(cryptor.EncryptConfig{
		Options: &encryptOpts.EncryptOptions,
	})
	encryptCmd.Flags().AddFlagSet(flags)
	lo.Must0(encryptCmd.MarkFlagRequired("input"))
	lo.Must0(encryptCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(encryptCmd)
}

func encryptRun(_ *cobra.Command, _ []string) error {
	keys, err := git.ReadKeyrings()
	if err != nil {
		return err
	}

	bundle, err := blob.New(blob.Config{Path: encryptOpts.Output, Keys: keys})
	if err != nil {
		return err
	}

	err = bundle.Import(encryptOpts.Input, nil)
	if err != nil {
		return err
	}

	err = bundle.Encrypt()
	if err != nil {
		return err
	}

	err = bundle.Sign()
	if err != nil {
		return err
	}

	log.Infof("successfully wrote signed and encrypted blob to %s", encryptOpts.Output)
	return nil
}
