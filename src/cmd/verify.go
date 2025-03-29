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

var verifyOpts struct {
	input      flag.Path
	signedOnly bool
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify a bundle",
	Run:   cobrax.Run(verifyRun),
}

func init() {
	flags := verifyCmd.Flags()

	verifyOpts.input.State = flag.MustExist
	flags.VarP(&verifyOpts.input, "input", "i", "File to verify")
	lo.Must0(verifyCmd.MarkFlagRequired("input"))

	flags.BoolVarP(&verifyOpts.signedOnly, "signed-only", "s", false,
		"Required if the archive is signed but not encrypted")

	rootCmd.AddCommand(verifyCmd)
}

func verifyRun(_ *cobra.Command, _ []string) (err error) {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	inf, err := os.Open(verifyOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(inf.Close, &err)

	bundle, err := blob.NewReader(inf, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: !verifyOpts.signedOnly,
	})
	if err != nil {
		return err
	}

	// Not strictly needed because the blob is verified in NewReader().
	_, err = io.Copy(io.Discard, bundle)
	if err != nil {
		return nil
	}

	log.Infof("successfully verified %s", verifyOpts.input.String())
	return nil
}
