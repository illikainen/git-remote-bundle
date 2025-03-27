package cmd

import (
	"io"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/flag"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var encryptOpts struct {
	input  flag.Path
	output flag.Path
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt and sign a bundle",
	Run:   cobrax.Run(encryptRun),
}

func init() {
	flags := encryptCmd.Flags()

	encryptOpts.input.State = flag.MustExist
	flags.VarP(&encryptOpts.input, "input", "i", "File to encrypt")
	lo.Must0(encryptCmd.MarkFlagRequired("input"))

	encryptOpts.output.State = flag.MustNotExist
	encryptOpts.output.Mode = flag.ReadWriteMode
	flags.VarP(&encryptOpts.output, "output", "o", "Output file for the encrypted blob")
	lo.Must0(encryptCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(encryptCmd)
}

func encryptRun(_ *cobra.Command, _ []string) error {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	writer, err := os.Create(encryptOpts.output.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	bundle, err := blob.NewWriter(writer, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: true,
	})
	if err != nil {
		return err
	}
	defer errorx.Defer(bundle.Close, &err)

	reader, err := os.Open(encryptOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	_, err = io.Copy(bundle, reader)
	if err != nil {
		return err
	}

	log.Infof("successfully wrote signed and encrypted blob to %s", encryptOpts.output.String())
	return nil
}
