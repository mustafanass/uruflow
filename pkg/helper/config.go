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

package helper

import (
	"crypto/rand"
	"encoding/hex"
	"os"
)

func GenerateSecret() string {
	return randomHex(16)
}

func GenerateToken() string {
	return randomHex(32)
}

func GenerateID() string {
	return randomHex(8)
}

func randomHex(size int) string {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
