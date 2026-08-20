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

package ufp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
)

const NonceSize = 32

func NewNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

func ComputeProof(context string, secret, nonce []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(context))
	mac.Write(nonce)
	return mac.Sum(nil)
}

func VerifyProof(context string, secret, nonce, proof []byte) bool {
	return hmac.Equal(proof, ComputeProof(context, secret, nonce))
}
