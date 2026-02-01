//go:build windows

package security

import (
	"github.com/danieljoos/wincred"
)

const CredentialTarget = "xe_pypi_token"

func SaveToken(token string) error {
	cred := wincred.NewGenericCredential(CredentialTarget)
	cred.CredentialBlob = []byte(token)
	cred.Persist = wincred.PersistSession
	return cred.Write()
}

func GetToken() (string, error) {
	cred, err := wincred.GetGenericCredential(CredentialTarget)
	if err != nil {
		return "", err
	}
	return string(cred.CredentialBlob), nil
}

func RevokeToken() error {
	cred, err := wincred.GetGenericCredential(CredentialTarget)
	if err != nil {
		return err
	}
	return cred.Delete()
}
