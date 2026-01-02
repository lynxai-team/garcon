// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, a static web builder, API server and middleware using Git, docker and podman.
// SPDX-License-Identifier: MIT

package gwt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
)

const (
	// NonceSize= 12 // AES-128 nonce is 12 bytes.
	gcmTagSize = 16 // AES-GCM tag is 16 bytes
)

// EncodingKey is used to encode each JWT secret key in the DB.
var EncodingKey []byte

// AesGcmEncryptHex : encrypt content.
func AesGcmEncryptHex(plaintext string) (string, error) {
	b, err := AesGcmEncryptBin([]byte(plaintext))
	return hex.EncodeToString(b), err
}

// AesGcmEncryptBin : encrypt content.
func AesGcmEncryptBin(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(EncodingKey)
	if err != nil {
		log.EncryptError("NewCipher AES", len(EncodingKey), "bytes", err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.EncryptError("NewGCM", err)
		return nil, err
	}

	// all will contain the nonce (first 12 bytes) + the cypher text + the GCM tag
	all := make([]byte, gcm.NonceSize(), gcm.NonceSize()+len(plaintext)+gcmTagSize)

	// write a random nonce
	_, err = rand.Read(all)
	if err != nil {
		return nil, log.EncryptError("random iv generation", err).Err()
	}

	// write the cypher text after the nonce and appends the GCM tag
	all = gcm.Seal(all, all, plaintext, nil)
	return all, nil
}

// AesGcmDecryptHex : decrypt content.
func AesGcmDecryptHex(encryptedString string) (string, error) {
	bytes, err := hex.DecodeString(encryptedString)
	if err != nil {
		return "", err
	}

	plaintext, err := AesGcmDecryptBin(bytes)
	return string(plaintext), err
}

func AesGcmDecryptBin(encryptedBytes []byte) ([]byte, error) {
	block, err := aes.NewCipher(EncodingKey)
	if err != nil {
		log.DecryptError("NewCipher AES", len(EncodingKey), "bytes", err)
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		log.DecryptError("NewGCM", err)
		return nil, err
	}

	iv := encryptedBytes[:gcm.NonceSize()]
	ciphertext := encryptedBytes[gcm.NonceSize():]
	dst := ciphertext[:0]

	// we are not subject to confused deputy attack => additionalData can be empty
	plaintext, err := gcm.Open(dst, iv, ciphertext, nil)
	if err != nil {
		log.DecryptError("Open ciphertext", len(ciphertext), "bytes", err)
		return nil, err
	}

	return plaintext, nil
}
