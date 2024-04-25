package cmd

import (
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var verifyOpts struct {
	cryptor.VerifyOptions
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify a bundle",
	Run:   cobrax.Run(verifyRun),
}

func init() {
	flags := cryptor.VerifyFlags(cryptor.VerifyConfig{
		Options: &verifyOpts.VerifyOptions,
	})
	verifyCmd.Flags().AddFlagSet(flags)
	lo.Must0(verifyCmd.MarkFlagRequired("input"))

	rootCmd.AddCommand(verifyCmd)
}

func verifyRun(_ *cobra.Command, _ []string) error {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{verifyOpts.Input}
		rw := []string{}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		if verifyOpts.Output != "" {
			// Required to mount the file in the sandbox.
			f, err := os.Create(verifyOpts.Output)
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			rw = append(rw, verifyOpts.Output)
		}

		return sandbox.Run(sandbox.Options{
			Args: os.Args,
			RO:   ro,
			RW:   rw,
		})
	}

	keys, err := git.ReadKeyring()
	if err != nil {
		return err
	}

	bundle, err := blob.New(blob.Config{
		Type: metadata.Name(),
		Path: verifyOpts.Input,
		Keys: keys,
	})
	if err != nil {
		return err
	}

	meta, err := bundle.Verify(verifyOpts.Output)
	if err != nil {
		return err
	}

	if !meta.Encrypted {
		log.Warnf("be aware that %s is unencrypted", verifyOpts.Input)
	}

	if verifyOpts.Output == "" {
		log.Infof("successfully verified %s", verifyOpts.Input)
	} else {
		log.Infof("successfully verified %s and wrote the verified data to %s",
			verifyOpts.Input, verifyOpts.Output)
	}
	return nil
}
