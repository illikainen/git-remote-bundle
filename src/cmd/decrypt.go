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
	"github.com/illikainen/go-utils/src/process"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var decryptOpts struct {
	input  flag.Path
	output flag.Path
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt a bundle",
	Run:   cobrax.Run(decryptRun),
}

func init() {
	flags := decryptCmd.Flags()

	decryptOpts.input.State = flag.MustExist
	flags.VarP(&decryptOpts.input, "input", "i", "File to decrypt")
	lo.Must0(decryptCmd.MarkFlagRequired("input"))

	decryptOpts.output.State = flag.MustNotExist
	flags.VarP(&decryptOpts.output, "output", "o", "Output file for the decrypted blob")
	lo.Must0(decryptCmd.MarkFlagRequired("output"))

	rootCmd.AddCommand(decryptCmd)
}

func decryptRun(_ *cobra.Command, _ []string) (err error) {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{decryptOpts.input.String()}
		rw := []string{decryptOpts.output.String()}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		// Required to mount the file in the sandbox.
		f, err := os.Create(decryptOpts.output.String())
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

	reader, err := os.Open(decryptOpts.input.String())
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

	writer, err := os.Create(decryptOpts.output.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(writer.Close, &err)

	_, err = io.Copy(writer, bundle)
	if err != nil {
		return err
	}

	log.Infof("successfully verified and decrypted %s to %s",
		decryptOpts.input.String(), decryptOpts.output.String())
	return nil
}
