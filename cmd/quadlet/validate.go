package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path"
)

var pubkeysForDir = make(map[string][]ed25519.PublicKey)

func loadPublicKeysFromDir(dir string) ([]ed25519.PublicKey, error) {
	if existing, ok := pubkeysForDir[dir]; ok {
		return existing, nil
	}

	keys := make([]ed25519.PublicKey, 0)

	files, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return keys, nil
		}
		return nil, err
	}

	for _, file := range files {
		name := file.Name()
		path := path.Join(dir, name)
		fileInfo, err := os.Stat(path)
		if err != nil {
			continue
		}

		if fileInfo.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}

			pub, err := x509.ParsePKIXPublicKey(data)
			if err != nil {
				return nil, err
			}

			if key, ok := pub.(ed25519.PublicKey); ok {
				keys = append(keys, key)
			} else {
				Logf("Warning: Public key %s has the wrong type", path)
			}
		}
	}

	pubkeysForDir[dir] = keys

	return keys, nil
}

func loadPublicKeysFromDirs(dirs []string) ([]ed25519.PublicKey, error) {
	keys := make([]ed25519.PublicKey, 0)

	for _, dir := range dirs {
		dirKeys, err := loadPublicKeysFromDir(dir)
		if err != nil {
			return nil, err
		}
		keys = append(keys, dirKeys...)
	}
	return keys, nil
}

func validateSignatureFor(path string, data []byte, keys []ed25519.PublicKey) error {
	sigPath := path + ".sig"
	sigData, err := os.ReadFile(sigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("No signature for %s, but signatures are required", path)
		}
		return fmt.Errorf("error loading signature %q, %w", sigPath, err)
	}

	if len(sigData) != ed25519.SignatureSize {
		return fmt.Errorf("Invalid signature size for %s", sigPath)
	}

	for _, key := range keys {
		if ed25519.Verify(key, data, sigData) {
			return nil
		}
	}

	return fmt.Errorf("No valid signature for %s", path)
}
