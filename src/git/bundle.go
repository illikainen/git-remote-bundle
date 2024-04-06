package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/iofs"
	"github.com/illikainen/go-utils/src/stringx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Clone a bundle.
//
// If `merge.verifySignatures` is true in .gitconfig, the references in `dir`
// are verified with the built-in signature functionality in Git.
//
// While the bundle is signed and verified with NaCl and/or RSA by this remote
// helper, the built-in signature functionality in Git may also be used as an
// additional defense in depth.
func cloneBundle(bundle string, dir string) (err error) {
	log.Tracef("cloning %s to %s", bundle, dir)

	if !strings.HasPrefix(bundle, "/") {
		panic("bug")
	}

	tmpDir, tmpClean, err := iofs.MkdirTemp()
	if err != nil {
		return err
	}
	defer errorx.Defer(tmpClean, &err)

	tmpRepo := filepath.Join(tmpDir, "repo")
	cloneCmd := exec.Command("git", "clone", "--bare", "--mirror", bundle, tmpRepo)
	err = cloneCmd.Run()
	if err != nil {
		return err
	}

	verifySignatures, err := VerifyMergeSignatures()
	if err != nil {
		return err
	}

	if verifySignatures {
		showRefCmd := exec.Command("git", "--git-dir", tmpRepo, "show-ref")
		showRef, err := showRefCmd.Output()
		if err != nil {
			return err
		}

		showRefLines := stringx.SplitLines(string(showRef))
		if len(showRefLines) == 0 {
			return errors.Errorf("%s has no refs", bundle)
		}

		for _, line := range stringx.SplitLines(string(showRef)) {
			elts := strings.Split(line, " ")
			if len(elts) != 2 || len(elts[0]) != 40 || !strings.HasPrefix(elts[1], "refs/") {
				return errors.Errorf("invalid show-ref line: %s", line)
			}

			verifyCmd := &exec.Cmd{}
			if strings.HasPrefix(elts[1], "refs/tags/") {
				log.Debugf("verify tag %s (%s)", elts[0], elts[1])
				verifyCmd = exec.Command("git", "--git-dir", tmpRepo, "verify-tag", elts[0])
			} else {
				log.Debugf("verify commit %s (%s)", elts[0], elts[1])
				verifyCmd = exec.Command("git", "--git-dir", tmpRepo, "verify-commit", elts[0])
			}

			verify, err := verifyCmd.CombinedOutput()
			if err != nil {
				return errors.Errorf("%s: %v", strings.TrimRight(string(verify), "\r\n"), err)
			}

			log.Infof("%s", verify)
		}
	}

	return os.Rename(tmpRepo, dir)
}
