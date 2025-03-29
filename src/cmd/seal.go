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

var sealOpts struct {
	input  flag.Path
	output flag.Path
}

var sealCmd = &cobra.Command{
	Use:   "seal",
	Short: "Encrypt and sign a bundle",
	Run:   cobrax.Run(sealRun),
}

func init() {
	flags := sealCmd.Flags()

	sealOpts.input.State = flag.MustExist
	flags.VarP(&sealOpts.input, "input", "i", "Input file to seal")
	lo.Must0(sealCmd.MarkFlagRequired("input"))

	sealOpts.output.State = flag.MustNotExist
	sealOpts.output.Mode = flag.ReadWriteMode
	flags.VarP(&sealOpts.output, "output", "o", "Output file for the sealed blob")
	lo.Must0(sealCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(sealCmd)
}

func sealRun(_ *cobra.Command, _ []string) error {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	writer, err := os.Create(sealOpts.output.String())
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

	reader, err := os.Open(sealOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	_, err = io.Copy(bundle, reader)
	if err != nil {
		return err
	}

	log.Infof("successfully wrote sealed blob to %s", sealOpts.output.String())
	return nil
}
