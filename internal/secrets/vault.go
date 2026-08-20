/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	KeySize    = 32
	keyMode    = 0o600
	keyDirMode = 0o700
)

var (
	ErrMalformed = errors.New("secret material is malformed")
	ErrKeySize   = errors.New("secret key must be 32 bytes")
)

type Vault struct {
	aead cipher.AEAD
}

func LoadOrCreateVault(path string) (*Vault, error) {
	key, err := loadKey(path)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Vault{aead: aead}, nil
}

func (v *Vault) Seal(plaintext string) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return v.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (v *Vault) Open(sealed []byte) (string, error) {
	size := v.aead.NonceSize()
	if len(sealed) < size {
		return "", ErrMalformed
	}

	plaintext, err := v.aead.Open(nil, sealed[:size], sealed[size:], nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return string(plaintext), nil
}

func loadKey(path string) ([]byte, error) {
	encoded, err := os.ReadFile(path)
	if err == nil {
		key, decodeErr := hex.DecodeString(strings.TrimSpace(string(encoded)))
		if decodeErr != nil {
			return nil, fmt.Errorf("read secret key %s: %w", path, decodeErr)
		}
		if len(key) != KeySize {
			return nil, ErrKeySize
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), keyDirMode); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), keyMode); err != nil {
		return nil, err
	}

	return key, nil
}
