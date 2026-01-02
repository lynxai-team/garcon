// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gwt_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	turbo64 "github.com/cristalhq/base64"
	"github.com/golang-jwt/jwt/v5"

	"github.com/LynxAIeu/garcon/gwt"

	"github.com/LynxAIeu/emo"
)

const jwtSample = `{"usr":"jane","grp":["group1","group2"],"org":["organization1","organization2"],"exp":1595950745}`

func TestNewAccessToken(t *testing.T) {
	t.Parallel()

	emo.GlobalColoring(false)

	cases := []struct {
		name       string
		timeout    string
		maxTTL     string
		user       string
		want       string
		groups     []string
		orgs       []string
		wantGenErr bool
		wantNewErr bool
	}{{
		name:       "HS256=HMAC-SHA256",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "HS384=HMAC-SHA384",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "HS512=HMAC-SHA512",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "RS256=RSASSA-PKCSv15-SHA256",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "",
		groups:     nil,
		orgs:       nil,
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "RS384=RSASSA-PKCSv15-SHA384",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "",
		groups:     nil,
		orgs:       nil,
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "RS512=RSASSA-PKCSv15-SHA512",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "",
		groups:     nil,
		orgs:       nil,
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "ES256=ECDSA-P256-SHA256",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "ES384=ECDSA-P384-SHA384",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "ES512=ECDSA-P521-SHA512",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}, {
		name:       "EdDSA=Ed25519",
		timeout:    "11m",
		maxTTL:     "12m",
		user:       "me",
		groups:     []string{"dev"},
		orgs:       []string{"wikipedia"},
		want:       "",
		wantGenErr: false,
		wantNewErr: false,
	}}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			algo := strings.Split(c.name, "=")[0]

			privateKey, err := gwt.GenerateSigningKey(algo)
			if (err != nil) != c.wantGenErr {
				t.Errorf("GenerateSigningKey() error = %v, wantGenErr %v", err, c.wantGenErr)
				return
			}
			t.Log(algo+" Private key len=", len(privateKey))

			publicDER, err := gwt.PrivateDER2PublicDER(algo, privateKey)
			if err != nil {
				t.Error("Public(privateKey) error:", err)
				return
			}
			t.Log(algo+" Public  key len=", len(publicDER))

			publicKey, err := gwt.PrivateDER2Public(algo, privateKey)
			if err != nil {
				t.Error("PrivateToPublic("+algo+",privateKey) error:", err)
				return
			}

			publicKey2, err := gwt.ParsePublicDER(algo, publicDER)
			if err != nil {
				t.Error("ParsePublicDER("+algo+",der) error:", err)
				return
			}

			if !reflect.DeepEqual(publicKey2, publicKey) {
				t.Error("public keys are not equal")
			}

			tokenStr, err := gwt.GenAccessTokenWithAlgo(algo, c.timeout, c.maxTTL, c.user, c.groups, c.orgs, privateKey)
			if (err != nil) != c.wantNewErr {
				t.Errorf("NewAccessToken() error = %v, wantNewErr %v", err, c.wantNewErr)
				return
			}
			t.Log(algo+" AccessToken len=", len(tokenStr), tokenStr)

			parser := jwt.NewParser(jwt.WithValidMethods([]string{algo}))

			var claims gwt.AccessClaims
			f := func(*jwt.Token) (any, error) { return publicKey, nil }
			_, err = parser.ParseWithClaims(tokenStr, &claims, f)
			if err != nil {
				t.Error("ParseWithClaims error:", err)
				return
			}

			err = gwt.ValidAccessToken(tokenStr, algo, publicDER)
			if err != nil {
				t.Error("ValidAccessToken:", err)
				return
			}

			var publicDERStr string
			if i%2 == 0 {
				publicDERStr = hex.EncodeToString(publicDER)
			} else {
				publicDERStr = turbo64.RawURLEncoding.EncodeToString(publicDER)
			}

			algoKey := algo + ":" + publicDERStr
			v, err := gwt.NewVerifier(algoKey, true)
			if err != nil {
				if algo[:2] == "RS" {
					return
				}
				t.Error("tokens.NewVerifier err:", err)
				t.Error("tokens.NewVerifier algoKey:", algoKey)
				t.Error("tokens.NewVerifier key len:", len(publicDERStr))
				return
			}

			if v == nil {
				return
			}

			ac, err := v.Claims([]byte(tokenStr))
			if err != nil {
				t.Error("Verifier.Claims error:", err)
				return
			}

			if !reflect.DeepEqual(ac, &claims) {
				t.Error("Different Claims")
				t.Error("claims=", claims)
				t.Error("ac    =", ac)
			}
		})
	}
}

/*
To verify the JWT signature, what is the most pertinent?
1. sign header+payload, then base64.Encode our signature, and finally compare the two base64 signatures
2. sign header+payload, then base64.Decode their signature, and finally compare the two binary signatures
Are the strings much less performant than []byte?

Let's bench it:

Go 1.18 (not sure)

go test -bench=. -benchmem ./...
goos: linux
goarch: amd64
pkg: github.com/LynxAIeu/quid/tokens
cpu: AMD Ryzen 9 3900X 12-Core Processor
BenchmarkOldDecodeStringB64-24    3331362   376.8 ns/op   256 B/op   2 allocs/op
BenchmarkOldDecodeB64-24          4375579   277.0 ns/op   112 B/op   1 allocs/op
BenchmarkOldDecodeT64-24          5554828   206.1 ns/op   112 B/op   1 allocs/op
BenchmarkOldEncodeB64-24          5913382   209.2 ns/op   144 B/op   1 allocs/op
BenchmarkOldEncodeT64-24          8053846   144.6 ns/op   144 B/op   1 allocs/op
BenchmarkOldEncodeToStringB64-24  3928515   297.4 ns/op   256 B/op   2 allocs/op
PASS
ok      github.com/LynxAIeu/quid/tokens     6.254s

Go 1.25.1

goarch: amd64
pkg: github.com/LynxAIeu/garcon/gwt
cpu: AMD Ryzen 9 3900X 12-Core Processor
BenchmarkOldDecodeB64-24          4593544   263.8 ns/op   112 B/op   1 allocs/op
BenchmarkOldDecodeT64-24          7166172   167.1 ns/op   112 B/op   1 allocs/op
BenchmarkOldEncodeB64-24          5650417   222.5 ns/op   144 B/op   1 allocs/op
BenchmarkOldEncodeT64-24          7994337   149.9 ns/op   144 B/op   1 allocs/op
BenchmarkDecodeStringB64-24       4731064   253.6 ns/op   112 B/op   1 allocs/op
BenchmarkDecodeStringT64-24       6623906   179.7 ns/op   112 B/op   1 allocs/op
BenchmarkEncodeB64-24            17638114    67.5 ns/op     0 B/op   0 allocs/op
BenchmarkEncodeT64-24            34232894    34.5 ns/op     0 B/op   0 allocs/op
BenchmarkEncodeLenB64-24          5973007   198.3 ns/op   144 B/op   1 allocs/op
BenchmarkEncodeLenT64-24          8425144   142.3 ns/op   144 B/op   1 allocs/op
BenchmarkEncodeToStringB64-24     4300639   281.0 ns/op   288 B/op   2 allocs/op
BenchmarkEncodeToStringT64-24     7856421   154.4 ns/op   144 B/op   1 allocs/op
PASS
ok      github.com/LynxAIeu/garcon/gwt     14.527s
*/

func TestEqualBase64Turbo64(t *testing.T) {
	t.Parallel()

	for i := range 200 {
		src := make([]byte, i)
		_, err := rand.Read(src)
		if err != nil {
			t.Fatal(err)
		}

		b64Len := base64.RawURLEncoding.EncodedLen(len(src))
		b64Enc := make([]byte, b64Len)
		base64.RawURLEncoding.Encode(b64Enc, src)

		t64Len := turbo64.RawURLEncoding.EncodedLen(len(src))
		t64Enc := make([]byte, t64Len)
		turbo64.RawURLEncoding.Encode(t64Enc, src)

		ok := bytes.Equal(b64Enc, t64Enc)
		if !ok {
			t.Errorf("Encode #%d b64=%v t64=%v", i, b64Enc, t64Enc)
		}

		b64Str := base64.RawURLEncoding.EncodeToString(src)
		t64Str := turbo64.RawURLEncoding.EncodeToString(src)

		if b64Str != t64Str {
			t.Errorf("EncodeToString #%d b64=%v t64=%v", i, b64Str, t64Str)
		}

		b64Data, err := base64.RawURLEncoding.DecodeString(b64Str)
		if err != nil {
			t.Error(err)
		}
		t64Data, err := turbo64.RawURLEncoding.DecodeString(b64Str)
		if err != nil {
			t.Error(err)
		}

		ok = bytes.Equal(b64Data, t64Data)
		if !ok {
			t.Errorf("DecodeString #%d b64=%v t64=%v", i, b64Data, t64Data)
		}
	}
}

func BenchmarkOldEncodeB64(b *testing.B) {
	src := []byte(jwtSample)

	size := base64.RawURLEncoding.EncodedLen(len(src))
	dataSame := make([]byte, size)
	base64.RawURLEncoding.Encode(dataSame, src)

	dataDiff := make([]byte, size)
	_, err := rand.Read(dataDiff)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; b.Loop(); i++ {
		dst := make([]byte, size)
		base64.RawURLEncoding.Encode(dst, src)
		_ = dst

		same := i%2 == 0
		var ok bool
		if same {
			ok = bytes.Equal(dst, dataSame)
		} else {
			ok = bytes.Equal(dst, dataDiff)
		}

		if ok != same {
			b.Errorf("#%d same=%v ok=%v", i, same, ok)
		}
	}
}

func BenchmarkOldEncodeT64(b *testing.B) {
	src := []byte(jwtSample)

	size := turbo64.RawURLEncoding.EncodedLen(len(src))
	dataSame := make([]byte, size)
	turbo64.RawURLEncoding.Encode(dataSame, src)

	dataDiff := make([]byte, size)
	_, err := rand.Read(dataDiff)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; b.Loop(); i++ {
		dst := make([]byte, size)
		turbo64.RawURLEncoding.Encode(dst, src)
		_ = dst

		same := i%2 == 0
		var ok bool
		if same {
			ok = bytes.Equal(dst, dataSame)
		} else {
			ok = bytes.Equal(dst, dataDiff)
		}

		if ok != same {
			b.Errorf("#%d same=%v ok=%v", i, same, ok)
		}
	}
}

func BenchmarkEncodeLenB64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	for i := 0; b.Loop(); i++ {
		size := base64.RawURLEncoding.EncodedLen(len(srcTxt))
		dst := make([]byte, size)
		base64.RawURLEncoding.Encode(dst, srcTxt)
	}
}

func BenchmarkEncodeB64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	size := base64.RawURLEncoding.EncodedLen(len(srcTxt))
	dst := make([]byte, size)
	for i := 0; b.Loop(); i++ {
		base64.RawURLEncoding.Encode(dst, srcTxt)
	}
}

func BenchmarkEncodeToStringB64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	for i := 0; b.Loop(); i++ {
		base64.RawURLEncoding.EncodeToString(srcTxt)
	}
}

func BenchmarkEncodeLenT64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	for i := 0; b.Loop(); i++ {
		size := turbo64.RawURLEncoding.EncodedLen(len(srcTxt))
		dst := make([]byte, size)
		turbo64.RawURLEncoding.Encode(dst, srcTxt)
	}
}

func BenchmarkEncodeT64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	size := turbo64.RawURLEncoding.EncodedLen(len(srcTxt))
	dst := make([]byte, size)
	for i := 0; b.Loop(); i++ {
		turbo64.RawURLEncoding.Encode(dst, srcTxt)
	}
}

func BenchmarkEncodeToStringT64(b *testing.B) {
	srcTxt := []byte(jwtSample)
	for i := 0; b.Loop(); i++ {
		turbo64.RawURLEncoding.EncodeToString(srcTxt)
	}
}

func BenchmarkOldDecodeB64(b *testing.B) {
	txtSame := []byte(jwtSample)
	txtDiff := make([]byte, len(txtSame))
	_, err := rand.Read(txtDiff)
	if err != nil {
		b.Fatal(err)
	}

	size := base64.RawURLEncoding.EncodedLen(len(txtSame))
	src := make([]byte, size)
	base64.RawURLEncoding.Encode(src, txtSame)

	for i := 0; b.Loop(); i++ {
		dst := make([]byte, len(txtSame))
		n, er := base64.RawURLEncoding.Decode(dst, src)
		if er != nil {
			b.Error(er)
		}
		dst = dst[:n]

		same := i%2 == 0
		var ok bool
		if same {
			ok = bytes.Equal(dst, txtSame)
		} else {
			ok = bytes.Equal(dst, txtDiff)
		}

		if ok != same {
			b.Errorf("#%d same=%v ok=%v", i, same, ok)
		}
	}
}

func BenchmarkOldDecodeT64(b *testing.B) {
	txtSame := []byte(jwtSample)
	txtDiff := make([]byte, len(txtSame))
	_, err := rand.Read(txtDiff)
	if err != nil {
		b.Fatal(err)
	}

	size := turbo64.RawURLEncoding.EncodedLen(len(txtSame))
	src := make([]byte, size)
	turbo64.RawURLEncoding.Encode(src, txtSame)

	for i := 0; b.Loop(); i++ {
		dst := make([]byte, len(txtSame))
		n, er := turbo64.RawURLEncoding.Decode(dst, src)
		if er != nil {
			b.Error(er)
		}
		dst = dst[:n]

		same := i%2 == 0
		var ok bool
		if same {
			ok = bytes.Equal(dst, txtSame)
		} else {
			ok = bytes.Equal(dst, txtDiff)
		}

		if ok != same {
			b.Errorf("#%d same=%v ok=%v", i, same, ok)
		}
	}
}

func BenchmarkDecodeStringB64(b *testing.B) {
	src := base64.RawURLEncoding.EncodeToString([]byte(jwtSample))
	for i := 0; b.Loop(); i++ {
		_, err := base64.RawURLEncoding.DecodeString(src)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeStringT64(b *testing.B) {
	src := turbo64.RawURLEncoding.EncodeToString([]byte(jwtSample))
	for i := 0; b.Loop(); i++ {
		_, err := turbo64.RawURLEncoding.DecodeString(src)
		if err != nil {
			b.Error(err)
		}
	}
}
