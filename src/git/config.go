package git

import (
	"bufio"
	"bytes"
	"fmt"
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
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func ReadKeyrings() (*blob.Keyrings, error) {
	sign, err := readKeyring(cryptor.SignPurpose)
	if err != nil {
		return nil, err
	}
	if len(sign.NaCl.Private) == 0 && len(sign.RSA.Private) == 0 {
		return nil, errors.Wrap(cryptor.ErrMissingPrivateKey, "sign")
	}
	if len(sign.NaCl.Public) == 0 && len(sign.RSA.Public) == 0 {
		return nil, errors.Wrap(cryptor.ErrMissingPublicKey, "sign")
	}

	encrypt, err := readKeyring(cryptor.EncryptPurpose)
	if err != nil {
		return nil, err
	}
	if len(encrypt.NaCl.Private) != 0 && len(encrypt.NaCl.Public) == 0 {
		return nil, errors.Wrap(cryptor.ErrMissingPublicKey, "NaCl encrypt")
	}
	if len(encrypt.RSA.Private) != 0 && len(encrypt.RSA.Public) == 0 {
		return nil, errors.Wrap(cryptor.ErrMissingPublicKey, "RSA encrypt")
	}
	if len(encrypt.NaCl.Public) > 0 && len(encrypt.RSA.Public) > 0 {
		if encrypt.NaCl.Private == nil || encrypt.RSA.Private == nil {
			return nil, errors.Wrap(cryptor.ErrMissingPrivateKey, "NaCl or RSA")
		}
		if len(encrypt.NaCl.Public) != len(encrypt.RSA.Public) {
			return nil, errors.Wrap(cryptor.ErrMissingPublicKey, "invalid encrypt config")
		}
	}

	return &blob.Keyrings{
		Sign:    sign,
		Encrypt: encrypt,
	}, nil
}

func readKeyring(purpose int) (*blob.Keyring, error) {
	naclPub, err := readPublicKeys("nacl", purpose)
	if err != nil {
		return nil, err
	}

	naclPriv, err := readPrivateKey("nacl", purpose)
	if err != nil {
		return nil, err
	}

	rsaPub, err := readPublicKeys("rsa", purpose)
	if err != nil {
		return nil, err
	}

	rsaPriv, err := readPrivateKey("rsa", purpose)
	if err != nil {
		return nil, err
	}

	return &blob.Keyring{
		NaCl: blob.Keys{
			Public:  naclPub,
			Private: naclPriv,
		},
		RSA: blob.Keys{
			Public:  rsaPub,
			Private: rsaPriv,
		},
	}, nil
}

func readPublicKeys(kind string, purpose int) ([]cryptor.PublicKey, error) {
	key := ""
	if purpose == cryptor.SignPurpose {
		key = fmt.Sprintf("bundle.sign.%sPublicKey", kind)
	} else if purpose == cryptor.EncryptPurpose {
		key = fmt.Sprintf("bundle.encrypt.%sPublicKey", kind)
	} else {
		return nil, cryptor.ErrInvalidPurpose
	}

	paths, err := ConfigSlice(key, "path")
	if err != nil {
		return nil, err
	}

	pubKeys := []cryptor.PublicKey{}
	for _, path := range paths {
		realPath, err := expand(path)
		if err != nil {
			return nil, err
		}

		pubKey, err := asymmetric.ReadPublicKey(cryptor.AsymmetricMap[kind], realPath, purpose)
		if err != nil {
			return nil, err
		}
		pubKeys = append(pubKeys, pubKey)
	}

	return pubKeys, nil
}

func readPrivateKey(kind string, purpose int) ([]cryptor.PrivateKey, error) {
	key := ""
	if purpose == cryptor.SignPurpose {
		key = fmt.Sprintf("bundle.sign.%sPrivateKey", kind)
	} else if purpose == cryptor.EncryptPurpose {
		key = fmt.Sprintf("bundle.encrypt.%sPrivateKey", kind)
	} else {
		return nil, cryptor.ErrInvalidPurpose
	}

	paths, err := ConfigSlice(key, "path")
	if err != nil {
		return nil, err
	}

	switch len(paths) {
	case 0:
		return nil, nil
	case 1:
		realPath, err := expand(paths[0])
		if err != nil {
			return nil, err
		}

		privKey, err := asymmetric.ReadPrivateKey(cryptor.AsymmetricMap[kind], realPath, purpose)
		if err != nil {
			return nil, err
		}
		return []cryptor.PrivateKey{privKey}, nil
	default:
		return nil, fmt.Errorf("%s: at most one key can be configured", key)
	}
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

	for _, purpose := range []string{"sign", "encrypt"} {
		for _, kind := range []string{"nacl", "rsa"} {
			for _, part := range []string{"PublicKey", "PrivateKey"} {
				key := fmt.Sprintf("bundle.%s.%s%s", purpose, kind, part)
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
