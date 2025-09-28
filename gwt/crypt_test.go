// Copyright 2021-2025 The contributors of Garcon.
// This file is part of Garcon, web+API server toolkit under the MIT License.
// SPDX-License-Identifier: MIT

package gwt_test

import (
	"testing"

	"github.com/LM4eu/garcon/gwt"
)

func TestAesGcm(t *testing.T) {
	t.Parallel()

	gwt.EncodingKey = []byte("eb037d66a3d07cc90c393a9bb04c172c")

	data := "some plaintext"
	out, err := gwt.AesGcmEncryptHex(data)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	in, err := gwt.AesGcmDecryptHex(out)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if data != in {
		t.Fatalf("expect %x got %x", data, in)
	}
}
