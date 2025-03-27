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

var signOpts struct {
	input  flag.Path
	output flag.Path
}

var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a bundle",
	Run:   cobrax.Run(signRun),
}

func init() {
	flags := signCmd.Flags()

	signOpts.input.State = flag.MustExist
	flags.VarP(&signOpts.input, "input", "i", "File to sign")
	lo.Must0(signCmd.MarkFlagRequired("input"))

	signOpts.output.State = flag.MustNotExist
	signOpts.output.Mode = flag.ReadWriteMode
	flags.VarP(&signOpts.output, "output", "o", "File to write the signed blob to")
	lo.Must0(signCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(signCmd)
}

func signRun(_ *cobra.Command, _ []string) (err error) {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	writer, err := os.Create(signOpts.output.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	bundle, err := blob.NewWriter(writer, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: false,
	})
	if err != nil {
		return err
	}
	defer errorx.Defer(bundle.Close, &err)

	reader, err := os.Open(signOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	_, err = io.Copy(bundle, reader)
	if err != nil {
		return err
	}

	err = bundle.Sign()
	if err != nil {
		return err
	}

	log.Infof("successfully wrote signed blob to %s", signOpts.output.String())
	return nil
}
