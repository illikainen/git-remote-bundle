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

var unsealOpts struct {
	input      flag.Path
	output     flag.Path
	signedOnly bool
}

var unsealCmd = &cobra.Command{
	Use:   "unseal",
	Short: "Verify and decrypt a bundle",
	Run:   cobrax.Run(unsealRun),
}

func init() {
	flags := unsealCmd.Flags()

	unsealOpts.input.State = flag.MustExist
	flags.VarP(&unsealOpts.input, "input", "i", "File to unseal")
	lo.Must0(unsealCmd.MarkFlagRequired("input"))

	unsealOpts.output.State = flag.MustNotExist
	unsealOpts.output.Mode = flag.ReadWriteMode
	flags.VarP(&unsealOpts.output, "output", "o", "Output file for the unsealed blob")
	lo.Must0(unsealCmd.MarkFlagRequired("output"))

	flags.BoolVarP(&unsealOpts.signedOnly, "signed-only", "s", false,
		"Required if the archive is signed but not encrypted")

	rootCmd.AddCommand(unsealCmd)
}

func unsealRun(_ *cobra.Command, _ []string) (err error) {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	reader, err := os.Open(unsealOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	bundle, err := blob.NewReader(reader, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: !unsealOpts.signedOnly,
	})
	if err != nil {
		return err
	}

	writer, err := os.Create(unsealOpts.output.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	_, err = io.Copy(writer, bundle)
	if err != nil {
		return err
	}

	log.Infof("signed by: %s", bundle.Signer)
	log.Infof("sha2-256: %s", bundle.Metadata.Hashes.SHA256)
	log.Infof("sha3-512: %s", bundle.Metadata.Hashes.KECCAK512)
	log.Infof("blake2b-512: %s", bundle.Metadata.Hashes.BLAKE2b512)
	log.Infof("successfully wrote unsealed blob to %s", unsealOpts.output.String())
	return nil
}
