package keyring

import (
	"io"

	"github.com/99designs/keyring"

	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
)

type Keyring interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

func New(
	appName, backend, rootDir string, userInput io.Reader,
) (Keyring, error) {
	serviceName := appName + "-e2ee"
	var db keyring.Keyring
	if backend == sdkkeyring.BackendMemory {
		db = keyring.NewArrayKeyring(nil)
	} else {
		kr, err := sdkkeyring.New(serviceName, backend, rootDir, userInput, nil)
		if err != nil {
			return nil, err
		}
		db = kr.DB()
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
