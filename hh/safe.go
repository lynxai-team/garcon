// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package hh

import (
	"encoding/base64"
	"hash"

	"github.com/minio/highwayhash"
)

//nolint:gochecknoglobals // set at startup time, used as constant during runtime
var hasherKey = RandomBytes(32)

// NewHash is based on HighwayHash, a hashing algorithm enabling high speed (especially on AMD64).
// See the study on HighwayHash and some other hash functions: https://github.com/fwessels/HashCompare
func NewHash() (hash.Hash, error) {
	h, err := highwayhash.New64(hasherKey)
	return h, err
}

// Obfuscate hashes the input string to prevent logging sensitive information.
func Obfuscate(str string) (string, error) {
	h, err := NewHash()
	if err != nil {
		return "", err
	}
	digest := h.Sum([]byte(str))
	return base64.RawURLEncoding.EncodeToString(digest), nil
}
