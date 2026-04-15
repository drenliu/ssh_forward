package sshd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"

	"golang.org/x/crypto/ssh"
)

func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		s, err := ssh.ParsePrivateKey(b)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	if !osIsNotExist(err) {
		return nil, err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, err
	}
	_ = pub
	return ssh.NewSignerFromKey(priv)
}

func osIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
