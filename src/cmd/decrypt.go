package cmd

import (
	"os"
	"path/filepath"

	"github.com/illikainen/git-remote-bundle/src/git"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/iofs"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var decryptOpts struct {
	cryptor.DecryptOptions
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt a bundle",
	Run:   cobrax.Run(decryptRun),
}

func init() {
	flags := cryptor.DecryptFlags(cryptor.DecryptConfig{
		Options: &decryptOpts.DecryptOptions,
	})
	lo.Must0(flags.MarkHidden("extract"))

	decryptCmd.Flags().AddFlagSet(flags)
	lo.Must0(decryptCmd.MarkFlagRequired("input"))
	lo.Must0(decryptCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(decryptCmd)
}

func decryptRun(_ *cobra.Command, _ []string) (err error) {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{decryptOpts.Input}
		rw := []string{decryptOpts.Output}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		// Required to mount the file in the sandbox.
		f, err := os.Create(decryptOpts.Output)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
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

	bundle, err := blob.New(blob.Config{Path: decryptOpts.Input, Keys: keys})
	if err != nil {
		return err
	}

	tmpDir, tmpClean, err := iofs.MkdirTemp()
	if err != nil {
		return err
	}
	defer errorx.Defer(tmpClean, &err)

	tmpCiphertext := filepath.Join(tmpDir, "ciphertext")
	meta, err := bundle.Verify(tmpCiphertext)
	if err != nil {
		return err
	}

	if !meta.Encrypted {
		return errors.Errorf("%s is not encrypted", decryptOpts.Input)
	}

	err = bundle.Decrypt(tmpCiphertext, decryptOpts.Output, meta.Keys)
	if err != nil {
		return err
	}

	log.Infof("successfully verified and decrypted %s to %s", decryptOpts.Input, decryptOpts.Output)
	return nil
}
