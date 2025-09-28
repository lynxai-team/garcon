// Copyright 2021-2025 The contributors of Garcon.
// This file is part of Garcon, web+API server toolkit under the MIT License.
// SPDX-License-Identifier: MIT

package gwt

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/json"
	"hash"
	"math/big"
	"strings"

	turbo64 "github.com/cristalhq/base64"

	"github.com/LM4eu/garcon/gg"
)

type (
	Tokenizer interface {
		GenAccessToken(timeout, maxTTL, user string, groups, orgs []string) (string, error)
		Sign(headerPayload []byte) []byte
		Verifier
	}

	Verifier interface {
		Claims(accessToken []byte) (*AccessClaims, error)
		Verify(headerPayload, signature []byte) bool
		Reuse() bool
	}

	Base struct {
		reuse bool
	}

	//nolint:embeddedstructfieldcheck // avoid padding
	BytesKey struct {
		key []byte
		Base
	}

	//nolint:embeddedstructfieldcheck // avoid padding
	ECDSA struct {
		key *ecdsa.PublicKey
		Base
	}

	HS256 struct{ BytesKey }
	HS384 struct{ BytesKey }
	HS512 struct{ BytesKey }
	EdDSA struct{ BytesKey }
	ES256 struct{ ECDSA }
	ES384 struct{ ECDSA }
	ES512 struct{ ECDSA }

	claimError struct {
		err         error
		claimsBytes []byte
	}
)

func (b Base) Reuse() bool { return b.reuse }

// NewVerifier creates a new Verifier to speed up the verification
// of many Access Tokens with the same verification key.
//
// The parameter algoKey accepts three different schemes:
//
//  1. only a HMAC secret key:
//     algoKey = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
//
//  2. both the signing algo and its verification key:
//     algoKey = "HS256:9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
//
//  3. the Quid URL to fetch the algo/key info from a given namespace
//     algoKey = "https://lm4.eu/quid/v1?ns=foobar"
//
// In the two first forms, NewVerifier accepts the key to be in hexadecimal, or in Base64 form.
// NewVerifier converts the verification key into binary DER form
// depending on the key string length and the optional algo name.
// The algo name is case insensitive.
func NewVerifier(algoKey string, reuse bool) (Verifier, error) {
	slice := strings.SplitN(algoKey, ":", 2)
	switch len(slice) {
	case 0:
		log.Panic("NewVerifier parameter must not be empty")
	case 1:
		return NewHMAC(algoKey, reuse) // here algoKey is just the secret-key
	}

	algo := strings.ToUpper(slice[0])
	keyTxt := slice[1]

	switch algo {
	case "", "HMAC":
		return NewHMAC(keyTxt, reuse)
	case "HS256":
		return NewHS256(keyTxt, reuse)
	case "HS384":
		return NewHS384(keyTxt, reuse)
	case "HS512":
		return NewHS512(keyTxt, reuse)
	case "RS256", "RS384", "RS512":
		return nil, log.ParamError(algo + notSupportedNotice).Err()
	case "PS256", "PS384", "PS512":
		return nil, log.ParamError(algo + notSupportedNotice).Err()
	case "ES256":
		return NewES256(keyTxt, reuse)
	case "ES384":
		return NewES384(keyTxt, reuse)
	case "ES512":
		return NewES512(keyTxt, reuse)
	case "EDDSA":
		return NewEdDSA(keyTxt, reuse)
	}

	return nil, log.ParamErrorf("Unexpected AlgoKey scheme %q in algoKey=%q", slice[0], algoKey).Err()
}

/*
$ go test -v ./tokens/... | grep -w Public
    tokens_test.go:155: HS256 Public  key len= 32
    tokens_test.go:155: HS512 Public  key len= 64
    tokens_test.go:155: HS384 Public  key len= 48
    tokens_test.go:155: EdDSA Public  key len= 44
    tokens_test.go:155: ES256 Public  key len= 91
    tokens_test.go:155: ES384 Public  key len= 120
    tokens_test.go:155: ES512 Public  key len= 158
    tokens_test.go:155: RS384 Public  key len= 294
    tokens_test.go:155: RS512 Public  key len= 294
    tokens_test.go:155: RS256 Public  key len= 294
*/

// NewHMAC creates an asymmetric-key Tokenizer based on HMAC algorithms.
//
//nolint:noinlineerr // code is nicer with inline errors
func NewHMAC(keyTxt string, reuse bool) (Tokenizer, error) {
	if strings.ContainsRune(keyTxt, ':') {
		return nil, ErrColumnInKey
	}
	if tokenizer, err := NewHS256(keyTxt, reuse); err == nil {
		return tokenizer, nil
	}
	if tokenizer, err := NewHS384(keyTxt, reuse); err == nil {
		return tokenizer, nil
	}
	if tokenizer, err := NewHS512(keyTxt, reuse); err == nil {
		return tokenizer, nil
	}
	return nil, ErrHMACKey
}

func NewHS256(keyTxt string, reuse bool) (*HS256, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 32)
	if err != nil {
		return nil, err
	}
	return &HS256{BytesKey{key, Base{reuse}}}, nil
}

func NewHS384(keyTxt string, reuse bool) (*HS384, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 48)
	if err != nil {
		return nil, err
	}
	return &HS384{BytesKey{key, Base{reuse}}}, nil
}

func NewHS512(keyTxt string, reuse bool) (*HS512, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 64)
	if err != nil {
		return nil, err
	}
	return &HS512{BytesKey{key, Base{reuse}}}, nil
}

func NewEdDSA(keyTxt string, reuse bool) (*EdDSA, error) {
	der, err := gg.DecodeHexOrB64(keyTxt, 44)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	edPubKey, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, ErrECDSAPubKey
	}
	return &EdDSA{BytesKey{edPubKey, Base{reuse}}}, nil
}

func NewES256(keyTxt string, reuse bool) (*ES256, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 91)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	ecPubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrECDSAPubKey
	}
	return &ES256{ECDSA{ecPubKey, Base{reuse}}}, nil
}

func NewES384(keyTxt string, reuse bool) (*ES384, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 120)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	ecPubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrECDSAPubKey
	}
	return &ES384{ECDSA{ecPubKey, Base{reuse}}}, nil
}

func NewES512(keyTxt string, reuse bool) (*ES512, error) {
	key, err := gg.DecodeHexOrB64(keyTxt, 158)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	ecPubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, ErrECDSAPubKey
	}
	return &ES512{ECDSA{ecPubKey, Base{reuse}}}, nil
}

func (v *HS256) GenAccessToken(timeout, maxTTL, user string, groups, orgs []string) (string, error) {
	return GenAccessToken(timeout, maxTTL, user, groups, orgs, v.key)
}

func (v *HS384) GenAccessToken(timeout, maxTTL, user string, groups, orgs []string) (string, error) {
	return GenAccessTokenWithAlgo("HS384", timeout, maxTTL, user, groups, orgs, v.key)
}

func (v *HS512) GenAccessToken(timeout, maxTTL, user string, groups, orgs []string) (string, error) {
	return GenAccessTokenWithAlgo("HS512", timeout, maxTTL, user, groups, orgs, v.key)
}

func (v *HS256) Verify(hp, sig []byte) bool { return verify(v, hp, sig) }
func (v *HS384) Verify(hp, sig []byte) bool { return verify(v, hp, sig) }
func (v *HS512) Verify(hp, sig []byte) bool { return verify(v, hp, sig) }
func (v *ES256) Verify(hp, sig []byte) bool { return v.verify(crypto.SHA256.New(), hp, sig) }
func (v *ES384) Verify(hp, sig []byte) bool { return v.verify(crypto.SHA384.New(), hp, sig) }
func (v *ES512) Verify(hp, sig []byte) bool { return v.verify(crypto.SHA512.New(), hp, sig) }
func (v *EdDSA) Verify(hp, sig []byte) bool { return v.verify(hp, sig) }

// Sign return the signature of the first two parts.
// It allocates hmac.New() each time to avoid race condition.
func (v *HS256) Sign(hp []byte) []byte { return sign(hmac.New(sha256.New, v.key), hp) }
func (v *HS384) Sign(hp []byte) []byte { return sign(hmac.New(sha512.New384, v.key), hp) }
func (v *HS512) Sign(hp []byte) []byte { return sign(hmac.New(sha512.New, v.key), hp) }

// B64Decode avoid allocating memory when reuse=true
// by reusing the input buffer to return the base64-decoded result.
func B64Decode(b64 []byte, reuse bool) ([]byte, error) {
	out := b64
	if !reuse {
		size := turbo64.RawURLEncoding.DecodedLen(len(b64))
		out = make([]byte, size)
	}
	n, err := turbo64.RawURLEncoding.Decode(out, b64)
	if err != nil {
		return nil, err
	}
	return out[:n], nil
}

// SplitThreeParts returns the period position decompose the JWT in three parts.
func SplitThreeParts(jwt []byte) (p1, p2 int, _ error) {
	p1 = bytes.IndexByte(jwt, '.')
	p2 = bytes.LastIndexByte(jwt, '.')
	if p1 < 0 || p1 >= p2 {
		return 0, 0, ErrThreeParts
	}
	return p1, p2, nil
}

func AccessClaimsFromBase64(payload []byte, reuse bool) (*AccessClaims, error) {
	payload, err := B64Decode(payload, reuse)
	if err != nil {
		return nil, ErrNoBase64JWT
	}

	var claims AccessClaims
	err = json.Unmarshal(payload, &claims)
	if err != nil {
		return nil, &claimError{err, payload}
	}

	err = claims.Valid() // error can be: expired or invalid access token
	return &claims, err
}

func (e *claimError) Error() string {
	return e.err.Error() + " => cannot JSON-decode AccessClaims: " + string(e.claimsBytes)
}

func (e *claimError) Unwrap() error {
	return e.err
}

func (v *HS256) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *HS384) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *HS512) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *ES256) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *ES384) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *ES512) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }
func (v *EdDSA) Claims(jwt []byte) (*AccessClaims, error) { return claims(v, jwt) }

func claims[T Verifier](v T, accessToken []byte) (*AccessClaims, error) {
	p1, p2, err := SplitThreeParts(accessToken)
	if err != nil {
		return nil, err
	}

	headerPayload := accessToken[:p2]
	signature := accessToken[p2+1:]
	if !v.Verify(headerPayload, signature) {
		return nil, ErrJWTSignature
	}

	payload := accessToken[p1+1 : p2]
	ac, err := AccessClaimsFromBase64(payload, v.Reuse())
	if err != nil {
		return nil, err
	}

	return ac, nil
}

func sign(digest hash.Hash, headerPayload []byte) []byte {
	digest.Write(headerPayload)
	sigBin := digest.Sum(nil)
	b64Len := turbo64.RawURLEncoding.EncodedLen(len(sigBin))
	sigB64 := make([]byte, b64Len)
	turbo64.RawURLEncoding.Encode(sigB64, sigBin)
	return sigB64
}

func verify(v Tokenizer, headerPayload, jwtSignature []byte) bool {
	ourSignature := v.Sign(headerPayload)
	return bytes.Equal(ourSignature, jwtSignature)
}

func (v *ECDSA) verify(digest hash.Hash, headerPayload, sig []byte) bool {
	sig, err := B64Decode(sig, v.Reuse())
	if err != nil {
		return false
	}
	digest.Write(headerPayload)
	r := big.NewInt(0).SetBytes(sig[:len(sig)/2])
	s := big.NewInt(0).SetBytes(sig[len(sig)/2:])
	return ecdsa.Verify(v.key, digest.Sum(nil), r, s)
}

func (v *EdDSA) verify(headerPayload, sig []byte) bool {
	sig, err := B64Decode(sig, v.Reuse())
	if err != nil {
		return false
	}
	return ed25519.Verify(v.key, headerPayload, sig)
}
