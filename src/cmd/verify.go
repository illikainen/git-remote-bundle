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

var verifyOpts struct {
	input  flag.Path
	output flag.Path
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

	verifyOpts.output.State = flag.MustNotExist
	flags.VarP(&verifyOpts.output, "output", "o", "Output file for the verified blob")

	rootCmd.AddCommand(verifyCmd)
}

func verifyRun(_ *cobra.Command, _ []string) (err error) {
	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{verifyOpts.input.String()}
		rw := []string{}

		gitRO, gitRW, err := git.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		if verifyOpts.output.String() != "" {
			// Required to mount the file in the sandbox.
			f, err := os.Create(verifyOpts.output.String())
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			rw = append(rw, verifyOpts.output.String())
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

	inf, err := os.Open(verifyOpts.input.String())
	if err != nil {
		return err
	}
	defer errorx.Defer(inf.Close, &err)

	bundle, err := blob.NewReader(inf, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: git.Encrypt(),
	})
	if err != nil {
		return err
	}
	if verifyOpts.output.String() == "" {
		_, err := io.Copy(io.Discard, bundle)
		if err != nil {
			return nil
		}

		log.Infof("successfully verified %s", verifyOpts.input.String())
	} else {
		outf, err := os.Create(verifyOpts.output.String())
		if err != nil {
			return err
		}
		defer errorx.Defer(outf.Close, &err)

		_, err = io.Copy(outf, bundle)
		if err != nil {
			return err
		}

		log.Infof("successfully verified %s and wrote the verified data to %s",
			verifyOpts.input.String(), verifyOpts.output.String())
	}
	return nil
}
