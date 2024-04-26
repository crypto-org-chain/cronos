package keyring

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/99designs/keyring"
	"golang.org/x/crypto/bcrypt"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/client/input"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
)

const (
	keyringFileDirName         = "e2ee-keyring-file"
	keyringTestDirName         = "e2ee-keyring-test"
	passKeyringPrefix          = "e2ee-keyring-%s" //nolint: gosec
	maxPassphraseEntryAttempts = 3
)

type Keyring interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

func New(
	appName, backend, rootDir string, userInput io.Reader,
) (Keyring, error) {
	var (
		db  keyring.Keyring
		err error
	)
	serviceName := appName + "-e2ee"
	switch backend {
	case sdkkeyring.BackendMemory:
		return newKeystore(keyring.NewArrayKeyring(nil), sdkkeyring.BackendMemory), nil
	case sdkkeyring.BackendTest:
		db, err = keyring.Open(keyring.Config{
			AllowedBackends: []keyring.BackendType{keyring.FileBackend},
			ServiceName:     serviceName,
			FileDir:         filepath.Join(rootDir, keyringTestDirName),
			FilePasswordFunc: func(_ string) (string, error) {
				return "test", nil
			},
		})
	case sdkkeyring.BackendFile:
		fileDir := filepath.Join(rootDir, keyringFileDirName)
		db, err = keyring.Open(keyring.Config{
			AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
			ServiceName:      serviceName,
			FileDir:          fileDir,
			FilePasswordFunc: newRealPrompt(fileDir, userInput),
		})
	case sdkkeyring.BackendOS:
		db, err = keyring.Open(keyring.Config{
			ServiceName:              serviceName,
			FileDir:                  rootDir,
			KeychainTrustApplication: true,
			FilePasswordFunc:         newRealPrompt(rootDir, userInput),
		})
	case sdkkeyring.BackendKWallet:
		db, err = keyring.Open(keyring.Config{
			AllowedBackends: []keyring.BackendType{keyring.KWalletBackend},
			ServiceName:     "kdewallet",
			KWalletAppID:    serviceName,
			KWalletFolder:   "",
		})
	case sdkkeyring.BackendPass:
		prefix := fmt.Sprintf(passKeyringPrefix, serviceName)
		db, err = keyring.Open(keyring.Config{
			AllowedBackends: []keyring.BackendType{keyring.PassBackend},
			ServiceName:     serviceName,
			PassPrefix:      prefix,
		})
	default:
		return nil, fmt.Errorf("unknown keyring backend %v", backend)
	}

	if err != nil {
		return nil, err
	}

	return newKeystore(db, backend), nil
}

var _ Keyring = keystore{}

type keystore struct {
	db      keyring.Keyring
	backend string
}

func newKeystore(kr keyring.Keyring, backend string) keystore {
	return keystore{
		db:      kr,
		backend: backend,
	}
}

func (ks keystore) Get(name string) ([]byte, error) {
	item, err := ks.db.Get(name)
	if err != nil {
		return nil, err
	}

	return item.Data, nil
}

func (ks keystore) Set(name string, secret []byte) error {
	return ks.db.Set(keyring.Item{
		Key:   name,
		Data:  secret,
		Label: name,
	})
}

func newRealPrompt(dir string, buf io.Reader) func(string) (string, error) {
	return func(prompt string) (string, error) {
		keyhashStored := false
		keyhashFilePath := filepath.Join(dir, "keyhash")

		var keyhash []byte

		_, err := os.Stat(keyhashFilePath)

		switch {
		case err == nil:
			keyhash, err = os.ReadFile(keyhashFilePath)
			if err != nil {
				return "", errorsmod.Wrap(err, fmt.Sprintf("failed to read %s", keyhashFilePath))
			}

			keyhashStored = true

		case os.IsNotExist(err):
			keyhashStored = false

		default:
			return "", errorsmod.Wrap(err, fmt.Sprintf("failed to open %s", keyhashFilePath))
		}

		failureCounter := 0

		for {
			failureCounter++
			if failureCounter > maxPassphraseEntryAttempts {
				return "", fmt.Errorf("too many failed passphrase attempts")
			}

			buf := bufio.NewReader(buf)
			pass, err := input.GetPassword(fmt.Sprintf("Enter keyring passphrase (attempt %d/%d):", failureCounter, maxPassphraseEntryAttempts), buf)
			if err != nil {
				// NOTE: LGTM.io reports a false positive alert that states we are printing the password,
				// but we only log the error.
				//
				// lgtm [go/clear-text-logging]
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if keyhashStored {
				if err := bcrypt.CompareHashAndPassword(keyhash, []byte(pass)); err != nil {
					fmt.Fprintln(os.Stderr, "incorrect passphrase")
					continue
				}

				return pass, nil
			}

			reEnteredPass, err := input.GetPassword("Re-enter keyring passphrase:", buf)
			if err != nil {
				// NOTE: LGTM.io reports a false positive alert that states we are printing the password,
				// but we only log the error.
				//
				// lgtm [go/clear-text-logging]
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if pass != reEnteredPass {
				fmt.Fprintln(os.Stderr, "passphrase do not match")
				continue
			}

			passwordHash, err := bcrypt.GenerateFromPassword([]byte(pass), 2)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			if err := os.WriteFile(keyhashFilePath, passwordHash, 0o600); err != nil {
				return "", err
			}

			return pass, nil
		}
	}
}
