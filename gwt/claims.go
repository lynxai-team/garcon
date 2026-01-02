// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, a static web builder, API server and middleware using Git, docker and podman.
// SPDX-License-Identifier: MIT

package gwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// AccessClaims is the standard claims for a user access token.
	AccessClaims struct {
		jwt.RegisteredClaims

		Username string   `json:"usr,omitempty"`
		Groups   []string `json:"grp,omitempty"`
		Orgs     []string `json:"org,omitempty"`
	}

	// RefreshClaims is the standard claims for a user refresh token.
	RefreshClaims struct {
		jwt.RegisteredClaims

		Namespace string `json:"namespace,omitempty"`
		Username  string `json:"username,omitempty"`
	}
)

// newAccessClaims creates a standard claim for a user access token.
func newAccessClaims(username string, groups, orgs []string, expiry time.Time) AccessClaims {
	return AccessClaims{
		jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiry)},
		username,
		groups,
		orgs,
	}
}

// newRefreshClaims creates a standard claim for a user refresh token.
func newRefreshClaims(namespace, user string, expiry time.Time) RefreshClaims {
	return RefreshClaims{
		jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiry)},
		namespace,
		user,
	}
}
