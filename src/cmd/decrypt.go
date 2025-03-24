package cmd

import (
	"io"
	"os"

	"github.com/illikainen/git-remote-bundle/src/git"
	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/cobrax"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/process"
	"github.com/illikainen/go-utils/src/sandbox"
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

	reader, err := os.Open(decryptOpts.Input)
	if err != nil {
		return err
	}
	defer errorx.Defer(reader.Close, &err)

	bundle, err := blob.NewReader(reader, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: true,
	})
	if err != nil {
		return err
	}

	writer, err := os.Create(decryptOpts.Output)
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	_, err = io.Copy(writer, bundle)
	if err != nil {
		return err
	}

	log.Infof("successfully verified and decrypted %s to %s", decryptOpts.Input, decryptOpts.Output)
	return nil
}
