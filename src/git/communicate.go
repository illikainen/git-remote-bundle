package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-netutils/src/transport"
	"github.com/illikainen/go-utils/src/errorx"
	"github.com/illikainen/go-utils/src/iofs"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var ErrMissingArguments = errors.New("missing arguments")
var ErrInvalidCommand = errors.New("invalid command")

func Communicate(uri *url.URL, cacheDir string) (err error) {
	keys, err := ReadKeyring()
	if err != nil {
		return err
	}

	bundleFile, err := os.OpenFile(filepath.Join(cacheDir, filepath.Base(uri.Path)),
		os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer errorx.Defer(bundleFile.Close, &err)

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
			err := gitUploadPack(bundleFile, uri, keys)
			if err != nil {
				return err
			}
		case "connect git-receive-pack": // uploads (e.g., git push)
			err := gitReceivePack(bundleFile, uri, keys)
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

func gitUploadPack(bundleFile *os.File, uri *url.URL, keys *blob.Keyring) error {
	return withRemoteBundle(bundleFile, uri, keys, false, func(repo string, _ string) error {
		uploadPack := exec.Command("git", "upload-pack", repo)
		uploadPack.Stdin = os.Stdin
		uploadPack.Stdout = os.Stdout
		uploadPack.Stderr = os.Stderr
		return uploadPack.Run()
	})
}

func gitReceivePack(bundleFile *os.File, uri *url.URL, keys *blob.Keyring) error {
	return withRemoteBundle(bundleFile, uri, keys, true, func(repo string, tmp string) (err error) {
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

		tmpPath := filepath.Join(tmp, "plaintext")
		err = exec.Command("git", "--git-dir", repo, "bundle", "create",
			tmpPath, "--branches", "--tags").Run()
		if err != nil {
			return err
		}

		tmpFile, err := os.Open(tmpPath) // #nosec G304
		if err != nil {
			return err
		}
		defer errorx.Defer(tmpFile.Close, &err)

		writer, err := blob.NewWriter(bundleFile, &blob.Options{
			Type:      metadata.Name(),
			Keyring:   keys,
			Encrypted: Encrypt(),
		})
		if err != nil {
			return err
		}
		defer errorx.Defer(writer.Close, &err)

		err = iofs.Copy(writer, tmpFile)
		if err != nil {
			return err
		}

		err = writer.Sign()
		if err != nil {
			return err
		}

		err = tmpFile.Sync()
		if err != nil {
			return err
		}

		err = blob.Upload(uri, bundleFile, &blob.Options{
			Type:      metadata.Name(),
			Keyring:   keys,
			Encrypted: Encrypt(),
		})
		if err != nil {
			return err
		}

		return nil
	})
}

func withRemoteBundle(bundleFile *os.File, uri *url.URL, keys *blob.Keyring, allowMissing bool,
	fn func(string, string) error) (err error) {
	tmpDir, tmpCleanup, err := iofs.MkdirTemp()
	if err != nil {
		return err
	}
	defer errorx.Defer(tmpCleanup, &err)

	tmpRepo := filepath.Join(tmpDir, "repo")
	bundle, err := blob.Download(uri, bundleFile, &blob.Options{
		Type:      metadata.Name(),
		Keyring:   keys,
		Encrypted: Encrypt(),
	})
	if err != nil {
		if !allowMissing || !errors.Is(err, transport.ErrNotExist) {
			return err
		}

		log.Tracef("initializing bare repo in %s", tmpRepo)
		err := exec.Command("git", "init", "--bare", tmpRepo).Run()
		if err != nil {
			return err
		}
	} else {
		log.Infof("%s: signed by %s", bundleFile.Name(), bundle.Signer)
		log.Infof("%s: sha2-256: %s", bundleFile.Name(), bundle.Metadata.Hashes.SHA256)
		log.Infof("%s: sha3-512: %s", bundleFile.Name(), bundle.Metadata.Hashes.KECCAK512)
		log.Infof("%s: blake2b-512: %s", bundleFile.Name(), bundle.Metadata.Hashes.BLAKE2b512)

		tmpBundle := filepath.Join(tmpDir, "bundle")
		f, err := os.Create(tmpBundle)
		if err != nil {
			return err
		}
		defer errorx.Defer(f.Close, &err)

		_, err = iofs.Seek(bundle, 0, io.SeekStart)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, bundle)
		if err != nil {
			return err
		}

		err = f.Sync()
		if err != nil {
			return err
		}

		err = cloneBundle(tmpBundle, tmpRepo)
		if err != nil {
			return err
		}
	}

	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		return err
	}

	return fn(tmpRepo, tmpDir)
}
