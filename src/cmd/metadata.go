package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/git"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/illikainen/go-utils/src/stringx"
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

		return sandbox.Run(sandbox.Options{
			Args: os.Args,
			RO:   ro,
			RW:   rw,
		})
	}

	keys, err := git.ReadKeyrings()
	if err != nil {
		return err
	}

	data, err := blob.New(blob.Config{Path: metadataOpts.Input, Keys: keys})
	if err != nil {
		return err
	}

	meta, err := data.Verify(metadataOpts.Output)
	if err != nil {
		return err
	}

	metaData, err := meta.Marshal()
	if err != nil {
		return err
	}
	metaStr := stringx.Sanitize(strings.TrimRight(string(metaData), "\x00"))
	_, err = fmt.Printf("%s\n", metaStr)
	if err != nil {
		return err
	}

	if metadataOpts.Output != "" {
		f, err := os.Create(metadataOpts.Output)
		if err != nil {
			return err
		}
		defer errorx.Defer(f.Close, &err)

		n, err := f.Write(metaStr)
		if err != nil {
			return err
		}
		if n != len(metaStr) {
			return errors.Errorf("invalid write size")
		}

		log.Infof("successfully wrote metadata to %s", metadataOpts.Output)
	}

	return nil
}
