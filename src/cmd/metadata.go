package cmd

import (
	"encoding/json"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/process"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var metadataOpts struct {
	cryptor.VerifyOptions
}

var metadataCmd = &cobra.Command{
	Use:    "metadata",
	Short:  "Show the metadata for a signed and optionally encrypted bundle",
	Run:    cobrax.Run(metadataRun),
	Hidden: true,
}

func init() {
	flags := cryptor.VerifyFlags(cryptor.VerifyConfig{
		Options: &metadataOpts.VerifyOptions,
	})
	metadataCmd.Flags().AddFlagSet(flags)
	lo.Must0(metadataCmd.MarkFlagRequired("input"))

	rootCmd.AddCommand(metadataCmd)
}

func metadataRun(_ *cobra.Command, _ []string) (err error) {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{metadataOpts.Input}
		rw := []string{}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		if metadataOpts.Output != "" {
			// Required to mount the file in the sandbox.
			f, err := os.Create(metadataOpts.Output)
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			rw = append(rw, metadataOpts.Output)
		}

		_, err = sandbox.Exec(sandbox.Options{
			Command: os.Args,
			RO:      ro,
			RW:      rw,
			Stdout:  process.LogrusOutput,
			Stderr:  process.LogrusOutput,
		})
		return err
	}

	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	f, err := os.Open(metadataOpts.Input)
	if err != nil {
		return err
	}
	defer errorx.Defer(f.Close, &err)

	blobber, err := blob.NewReader(f, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: git.Encrypt(),
	})
	if err != nil {
		return err
	}

	meta, err := json.MarshalIndent(blobber.Metadata(), "", "    ")
	if err != nil {
		return err
	}
	meta = append(meta, '\n')

	log.Infof("%s", meta)
	if metadataOpts.Output != "" {
		f, err := os.Create(metadataOpts.Output)
		if err != nil {
			return err
		}
		defer errorx.Defer(f.Close, &err)

		n, err := f.Write(meta)
		if err != nil {
			return err
		}
		if n != len(meta) {
			return errors.Errorf("invalid write size")
		}

		log.Infof("successfully wrote metadata to %s", metadataOpts.Output)
	}

	return nil
}
