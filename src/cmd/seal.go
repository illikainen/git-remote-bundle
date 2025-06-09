package cmd

import (
	"io"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var sealOpts struct {
	input      string
	output     string
	signedOnly bool
}

var sealCmd = &cobra.Command{
	Use:     "seal",
	Short:   "Encrypt and sign a bundle",
	PreRunE: sealPreRun,
	RunE:    sealRun,
}

func init() {
	flags := sealCmd.Flags()

	flags.StringVarP(&sealOpts.input, "input", "i", "", "Input file to seal")
	lo.Must0(sealCmd.MarkFlagRequired("input"))

	flags.StringVarP(&sealOpts.output, "output", "o", "", "Output file for the sealed blob")
	lo.Must0(sealCmd.MarkFlagRequired("output"))

	flags.BoolVarP(&sealOpts.signedOnly, "signed-only", "s", false,
		"Only sign the archive, don't encrypt it")

	rootCmd.AddCommand(sealCmd)
}

func sealPreRun(_ *cobra.Command, _ []string) error {
	err := rootOpts.Sandbox.AddReadOnlyPath(sealOpts.input)
	if err != nil {
		return err
	}

	err = rootOpts.Sandbox.AddReadWritePath(sealOpts.output)
	if err != nil {
		return err
	}

	return rootOpts.Sandbox.Confine()
}

func sealRun(_ *cobra.Command, _ []string) error {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	writer, err := os.Create(sealOpts.output)
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	bundle, err := blob.NewWriter(writer, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: !sealOpts.signedOnly,
	})
	if err != nil {
		return err
	}
	defer errorx.Defer(bundle.Close, &err)

	reader, err := os.Open(sealOpts.input)
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	_, err = io.Copy(bundle, reader)
	if err != nil {
		return err
	}

	log.Infof("successfully wrote sealed blob to %s", sealOpts.output)
	return nil
}
