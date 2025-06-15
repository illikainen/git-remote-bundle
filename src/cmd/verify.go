package cmd

import (
	"io"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/fn"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var verifyOpts struct {
	input      string
	signedOnly bool
}

var verifyCmd = &cobra.Command{
	Use:     "verify",
	Short:   "Verify a bundle",
	PreRunE: verifyPreRun,
	RunE:    verifyRun,
}

func init() {
	flags := verifyCmd.Flags()

	flags.StringVarP(&verifyOpts.input, "input", "i", "", "File to verify")
	fn.Must(verifyCmd.MarkFlagRequired("input"))

	flags.BoolVarP(&verifyOpts.signedOnly, "signed-only", "s", false,
		"Required if the archive is signed but not encrypted")

	rootCmd.AddCommand(verifyCmd)
}

func verifyPreRun(_ *cobra.Command, _ []string) error {
	err := rootOpts.Sandbox.AddReadOnlyPath(verifyOpts.input)
	if err != nil {
		return err
	}

	return rootOpts.Sandbox.Confine()
}

func verifyRun(_ *cobra.Command, _ []string) (err error) {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	inf, err := os.Open(verifyOpts.input)
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

	log.Infof("signed by: %s", bundle.Signer)
	log.Infof("sha2-256: %s", bundle.Metadata.Hashes.SHA256)
	log.Infof("sha3-512: %s", bundle.Metadata.Hashes.KECCAK512)
	log.Infof("blake2b-512: %s", bundle.Metadata.Hashes.BLAKE2b512)
	log.Infof("successfully verified %s", verifyOpts.input)
	return nil
}
