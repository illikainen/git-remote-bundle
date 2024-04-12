package git

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-netutils/src/sshx"
	"github.com/illikainen/go-netutils/src/transport"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/iofs"
	"github.com/illikainen/go-utils/src/sandbox"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var ErrMissingArguments = errors.New("missing arguments")
var ErrInvalidCommand = errors.New("invalid command")

func Communicate() (err error) {
	if len(os.Args) != 3 {
		return ErrMissingArguments
	}

	uri, err := url.Parse(os.Args[2])
	if err != nil {
		return err
	}

	baseDir, err := BaseDir()
	if err != nil {
		return err
	}

	err = os.MkdirAll(baseDir, 0700)
	if err != nil {
		return err
	}

	if sandbox.Compatible() && !sandbox.IsSandboxed() {
		ro := []string{}
		rw := []string{baseDir}

		gitRO, gitRW, err := SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, gitRO...)
		rw = append(rw, gitRW...)

		sshRO, sshRW, err := sshx.SandboxPaths()
		if err != nil {
			return err
		}
		ro = append(ro, sshRO...)
		rw = append(rw, sshRW...)

		// Required to mount the file in the sandbox.
		if uri.Scheme == "file" {
			path, err := expand(uri.Path)
			if err != nil {
				return err
			}

			f, err := os.Create(path)
			if err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}

			rw = append(rw, uri.Path)
		}

		return sandbox.Run(sandbox.Options{
			Args:  os.Args,
			RO:    ro,
			RW:    rw,
			Share: sandbox.ShareNet,
		})
	}

	keys, err := ReadKeyring()
	if err != nil {
		return err
	}

	_, bundleName := filepath.Split(uri.Path)
	bundlePath := filepath.Join(baseDir, bundleName)

	xfer, err := transport.New(uri)
	if err != nil {
		return err
	}
	defer errorx.Defer(xfer.Close, &err)

	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		cmd := scan.Text()
		log.Tracef("cmd: %s", cmd)

		switch cmd {
		case "capabilities":
			err := capabilities()
			if err != nil {
				return err
			}
		case "connect git-upload-pack": // retrievals (e.g., git fetch)
			err := gitUploadPack(bundlePath, uri.Path, xfer, keys)
			if err != nil {
				return err
			}
		case "connect git-receive-pack": // uploads (e.g., git push)
			err := gitReceivePack(bundlePath, uri.Path, xfer, keys)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %s", ErrInvalidCommand, cmd)
		}
	}

	return scan.Err()
}

func capabilities() error {
	_, err := os.Stdout.WriteString("connect\n\n")
	return err
}

func gitUploadPack(bundlePath string, remotePath string, xfer transport.Transport,
	keys *blob.Keyring) error {
	return withRemoteBundle(bundlePath, remotePath, xfer, keys, false,
		func(repo string, _ string) error {
			uploadPack := exec.Command("git", "upload-pack", repo)
			uploadPack.Stdin = os.Stdin
			uploadPack.Stdout = os.Stdout
			uploadPack.Stderr = os.Stderr
			return uploadPack.Run()
		})
}

func gitReceivePack(bundlePath string, remotePath string, xfer transport.Transport,
	keys *blob.Keyring) error {
	return withRemoteBundle(bundlePath, remotePath, xfer, keys, true,
		func(repo string, tmp string) error {
			oldRefs, err := exec.Command("git", "--git-dir", repo, "show-ref").Output()
			if err != nil {
				oldRefs = []byte{}
			}

			receivePack := exec.Command("git", "receive-pack", repo)
			receivePack.Stdin = os.Stdin
			receivePack.Stdout = os.Stdout
			receivePack.Stderr = os.Stderr
			err = receivePack.Run()
			if err != nil {
				return err
			}

			newRefs, err := exec.Command("git", "--git-dir", repo, "show-ref").Output()
			if err != nil {
				return err
			}

			if bytes.Equal(oldRefs, newRefs) {
				log.Debug("nothing new to upload")
				return nil
			}

			tmpBundlePath := filepath.Join(tmp, "bundle")
			tmpBundle, err := blob.New(blob.Config{
				Path:      tmpBundlePath,
				Transport: xfer,
				Keys:      keys,
			})
			if err != nil {
				return err
			}

			tmpFlatBundlePath := filepath.Join(tmp, "flat.bundle")
			err = exec.Command("git", "--git-dir", repo, "bundle", "create",
				tmpFlatBundlePath, "--branches", "--tags").Run()
			if err != nil {
				return err
			}

			err = tmpBundle.Import(tmpFlatBundlePath, nil)
			if err != nil {
				return err
			}

			if Encrypt() {
				err := tmpBundle.Encrypt()
				if err != nil {
					return err
				}
			}

			err = tmpBundle.Sign()
			if err != nil {
				return err
			}

			bundle, err := tmpBundle.Move(bundlePath)
			if err != nil {
				return err
			}

			err = bundle.Upload(remotePath)
			if err != nil {
				return err
			}

			return nil
		})
}

func withRemoteBundle(bundlePath string, remotePath string, xfer transport.Transport, keys *blob.Keyring,
	allowMissing bool, fn func(string, string) error) (err error) {
	tmpDir, tmpCleanup, err := iofs.MkdirTemp()
	if err != nil {
		return err
	}
	defer errorx.Defer(tmpCleanup, &err)

	_, bundleName := filepath.Split(bundlePath)
	bundle, err := blob.New(blob.Config{Path: bundlePath, Transport: xfer, Keys: keys})
	if err != nil {
		return err
	}
	tmpRepo := filepath.Join(tmpDir, "repo")

	exists, err := bundle.HasRemote(remotePath)
	if err != nil {
		return err
	}
	if exists {
		err = bundle.Download(remotePath)
		if err != nil {
			return err
		}

		verifiedBundlePath := filepath.Join(tmpDir, "verified.bundle")
		meta, err := bundle.Verify(verifiedBundlePath)
		if err != nil {
			return err
		}

		cloneBundlePath := verifiedBundlePath
		if Encrypt() {
			cloneBundlePath = filepath.Join(tmpDir, bundleName)
			err := bundle.Decrypt(verifiedBundlePath, cloneBundlePath, meta.Keys)
			if err != nil {
				return err
			}
		}

		err = cloneBundle(cloneBundlePath, tmpRepo)
		if err != nil {
			return err
		}
	} else if allowMissing {
		err := exec.Command("git", "init", "--bare", tmpRepo).Run()
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("%s does not exist", xfer)
	}

	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		return err
	}

	return fn(tmpRepo, tmpDir)
}
