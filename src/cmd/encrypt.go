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

var encryptOpts struct {
	cryptor.EncryptOptions
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt and sign a bundle",
	Run:   cobrax.Run(encryptRun),
}

func init() {
	flags := cryptor.EncryptFlags(cryptor.EncryptConfig{
		Options: &encryptOpts.EncryptOptions,
	})
	encryptCmd.Flags().AddFlagSet(flags)
	lo.Must0(encryptCmd.MarkFlagRequired("input"))
	lo.Must0(encryptCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(encryptCmd)
}

func encryptRun(_ *cobra.Command, _ []string) error {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{encryptOpts.Input}
		rw := []string{encryptOpts.Output}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		// Required to mount the file in the sandbox.
		f, err := os.Create(encryptOpts.Output)
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

	bundle, err := blob.New(blob.Config{
		Type: metadata.Name(),
		Path: encryptOpts.Output,
		Keys: keys,
	})
	if err != nil {
		return err
	}

	err = bundle.Import(encryptOpts.Input, nil)
	if err != nil {
		return err
	}

	err = bundle.Encrypt()
	if err != nil {
		return err
	}

	err = bundle.Sign()
	if err != nil {
		return err
	}

	log.Infof("successfully wrote signed and encrypted blob to %s", encryptOpts.Output)
	return nil
}
