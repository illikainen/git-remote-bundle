package git

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/illikainen/git-remote-bundle/src/metadata"

	"github.com/illikainen/go-cryptor/src/asymmetric"
	"github.com/illikainen/go-cryptor/src/blob"
	"github.com/illikainen/go-cryptor/src/cryptor"
	"github.com/illikainen/go-utils/src/iofs"
	"github.com/illikainen/go-utils/src/stringx"
	log "github.com/sirupsen/logrus"
)

func ReadKeyring() (*blob.Keyring, error) {
	pubPaths, err := ConfigSlice("bundle.pubKeys", "path")
	if err != nil {
		return nil, err
	}

	pubKeys := []cryptor.PublicKey{}
	for _, path := range pubPaths {
		realPath, err := expand(path)
		if err != nil {
			return nil, err
		}

		pubKey, err := asymmetric.ReadPublicKey(realPath)
		if err != nil {
			return nil, err
		}

		pubKeys = append(pubKeys, pubKey)
	}

	privPath, err := Config("bundle.privKey", "path")
	if err != nil {
		return nil, err
	}

	realPrivPath, err := expand(privPath)
	if err != nil {
		return nil, err
	}

	privKey, err := asymmetric.ReadPrivateKey(realPrivPath)
	if err != nil {
		return nil, err
	}

	return &blob.Keyring{
		Public:  pubKeys,
		Private: privKey,
	}, nil
}

func BaseDir() (string, error) {
	baseDirs, err := ConfigSlice("bundle.baseDir", "path")
	if err == nil && len(baseDirs) == 1 {
		return baseDirs[0], nil
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cache, metadata.Name()), nil
}

func LogLevel() log.Level {
	logLevels, err := ConfigSlice("bundle.logLevel", "path")
	if err == nil && len(logLevels) == 1 {
		level, err := log.ParseLevel(logLevels[0])
		if err == nil {
			return level
		}

		log.Warnf("bundle.logLevel: %s", err)
	}

	return log.InfoLevel
}

func Encrypt() bool {
	encrypt, err := Config("bundle.encrypt", "bool")
	if err != nil {
		log.Warnf("unable to parse `bundle.encrypt`, failing safe by interpreting it as true")
		return true
	}

	return encrypt == "true"
}

// The `merge.verifySignatures` option has nothing to do with the cryptographic
// operations performed by this remote helper.  It's a built-in option in Git
// to enable signature verification during merge operations.
//
// The option is used by this remote helper to use the built-in signature
// verification in Git as an additional defense in depth.
func VerifyMergeSignatures() (bool, error) {
	verifySignatures, err := Config("merge.verifySignatures", "bool")
	if err != nil {
		return true, err
	}

	if verifySignatures != "" {
		return strconv.ParseBool(verifySignatures)
	}

	// Unfortunate default in Git but we break too many use-cases if we
	// override it here.
	return false, nil
}

func Config(name string, vtype string) (string, error) {
	value, err := exec.Command("git", "config", "--type", vtype, "--get", name).Output()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if ok && exit.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}

	s := string(value)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")

	return s, nil
}

func ConfigSlice(name string, vtype string) ([]string, error) {
	output, err := exec.Command("git", "config", "--type", vtype, "--get-all", name).Output()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if ok && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	values := []string{}
	scan := bufio.NewScanner(bytes.NewReader(output))
	for scan.Scan() {
		values = append(values, scan.Text())
	}

	err = scan.Err()
	if err != nil {
		return nil, err
	}

	return values, nil
}

func SandboxPaths() (ro []string, rw []string, err error) {
	ro = append(ro, filepath.Join(string(os.PathSeparator), "etc", "gitconfig"))

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, err
	}
	ro = append(ro, filepath.Join(home, ".gitconfig"))

	config, err := os.UserConfigDir()
	if err != nil {
		return nil, nil, err
	}
	ro = append(ro, filepath.Join(config, "git", "config"))

	include, err := Config("include.path", "path")
	if err != nil {
		return nil, nil, err
	}
	ro = append(ro, include)

	format, err := Config("gpg.format", "path")
	if err != nil {
		return nil, nil, err
	}

	if format == "ssh" {
		signingKey, err := Config("user.signingKey", "path")
		if err != nil {
			return nil, nil, err
		}
		ro = append(ro, signingKey)

		allowedSignersFile, err := Config("gpg.ssh.allowedSignersFile", "path")
		if err != nil {
			return nil, nil, err
		}
		ro = append(ro, allowedSignersFile)
	}

	for _, key := range []string{"bundle.pubkeys", "bundle.privkey"} {
		paths, err := ConfigSlice(key, "path")
		if err != nil {
			return nil, nil, err
		}

		for _, path := range paths {
			realPath, err := expand(path)
			if err != nil {
				return nil, nil, err
			}

			ro = append(ro, realPath)
		}
	}

	return ro, nil, nil
}

func expand(path string) (string, error) {
	intPath, err := stringx.Interpolate(path)
	if err != nil {
		return "", err
	}

	realPath, err := iofs.Expand(intPath)
	if err != nil {
		return "", err
	}

	return realPath, nil
}
