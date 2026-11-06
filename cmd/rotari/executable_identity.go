package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

type executableIdentity struct {
	SHA256   string
	Modified time.Time
}

func currentExecutableIdentity() (executableIdentity, error) {
	path, err := os.Executable()
	if err != nil {
		return executableIdentity{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return executableIdentity{}, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		return executableIdentity{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return executableIdentity{}, err
	}
	if err := file.Close(); err != nil {
		return executableIdentity{}, err
	}
	return executableIdentity{SHA256: hex.EncodeToString(hash.Sum(nil)), Modified: info.ModTime()}, nil
}
