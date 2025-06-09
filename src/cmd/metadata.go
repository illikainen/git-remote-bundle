package cmd

import (
	"encoding/json"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var metadataOpts struct {
	input      string
	output     string
	signedOnly bool
}

var metadataCmd = &cobra.Command{
	Use:     "metadata",
	Short:   "Show the metadata for a signed and optionally encrypted bundle",
	PreRunE: metadataPreRun,
	RunE:    metadataRun,
	Hidden:  true,
}

func init() {
	flags := metadataCmd.Flags()

	flags.StringVarP(&metadataOpts.input, "input", "i", "", "File to verify")
	lo.Must0(metadataCmd.MarkFlagRequired("input"))

	flags.StringVarP(&metadataOpts.output, "output", "o", "", "Output file for the verified blob")

	flags.BoolVarP(&metadataOpts.signedOnly, "signed-only", "s", false,
		"Required if the archive is signed but not encrypted")

	rootCmd.AddCommand(metadataCmd)
}

func metadataPreRun(_ *cobra.Command, _ []string) error {
	err := rootOpts.Sandbox.AddReadOnlyPath(metadataOpts.input)
	if err != nil {
		return err
	}

	if metadataOpts.output != "" {
		err := rootOpts.Sandbox.AddReadWritePath(metadataOpts.output)
		if err != nil {
			return err
		}
	}

	return rootOpts.Sandbox.Confine()
}

func metadataRun(_ *cobra.Command, _ []string) (err error) {
	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	in, err := os.Open(metadataOpts.input)
	if err != nil {
		return err
	}
	defer errorx.Defer(in.Close, &err)

	blobber, err := blob.NewReader(in, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: !metadataOpts.signedOnly,
	})
	if err != nil {
		return err
	}

	meta, err := json.MarshalIndent(blobber.Metadata, "", "    ")
	if err != nil {
		return err
	}
	meta = append(meta, '\n')

	log.Infof("%s", meta)
	if metadataOpts.output != "" {
		out, err := os.Create(metadataOpts.output)
		if err != nil {
			return err
		}
		defer errorx.Defer(out.Close, &err)

		n, err := out.Write(meta)
		if err != nil {
			return err
		}
		if n != len(meta) {
			return errors.Errorf("invalid write size")
		}

		log.Infof("successfully wrote metadata to %s", metadataOpts.output)
	}

	return nil
}
