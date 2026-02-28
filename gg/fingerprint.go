// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gg

import (
	"net/http"
)

// FingerprintExplanation provides a description of the logged HTTP headers.
const FingerprintExplanation = `
1. Accept-Language, the language preferred by the user. 
2. User-Agent, name and version of the browser and OS. 
3. R=Referer, the website from which the request originated. 
4. A=Accept, the content types the browser prefers. 
5. E=Accept-Encoding, the compression formats the browser supports. 
6. Connection, can be empty, "keep-alive" or "close". 
7. Cache-Control, how the browser is caching data. 
8. URI=Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS. 
9. Via avoids request loops and identifies protocol capabilities. 
10. Authorization or Cookie (both should not be present at the same time). 
11. DNT (Do Not Track) is being dropped by web browsers.`

// FingerprintMD provide the browser fingerprint in markdown format.
// Attention: read the .
func FingerprintMD(r *http.Request) string {
	return "\n" + "- **IP**: " + Sanitize(r.RemoteAddr) +
		headerMD(r, "Accept-Language") + // language preferred by the user
		headerMD(r, "User-Agent") + // name and version of browser and OS
		headerMD(r, "Referer") + // URL from which the request originated
		headerMD(r, "Accept") + // content types the browser prefers
		headerMD(r, "Accept-Encoding") + // compression formats the browser supports
		headerMD(r, "Connection") + // can be: empty, "keep-alive" or "close"
		headerMD(r, "Cache-Control") + // how the browser is caching data
		headerMD(r, "DNT") + // "Do Not Track" is being dropped by web standards and browsers
		headerMD(r, "Via") + // avoid request loops and identify protocol capabilities
		headerMD(r, "Authorization") + // Attention: may contain confidential data
		headerMD(r, "Cookie") // Attention: may contain confidential data
}

// FingerprintTxt logs like logIPMethodURL and also logs the browser fingerprint.
// Attention! FingerprintTxt provides personal data that may identify users.
// To comply with GDPR, the website data owner must have a legitimate reason to do so.
// Before enabling the fingerprinting, the user must understand it
// and give their freely-given informed consent such as the settings change from "no" to "yes".
func FingerprintTxt(r *http.Request) string {
	// double space after "in" is for padding with "out" logs
	line := " " +
		// 1. Accept-Language, the language preferred by the user.
		SafeHeader(r, "Accept-Language") + " " +
		// 2. User-Agent, name and version of the browser and OS.
		SafeHeader(r, "User-Agent") +
		// 3. R=Referer, the website from which the request originated.
		headerTxt(r, "Referer", "R=", "") +
		// 4. A=Accept, the content types the browser prefers.
		headerTxt(r, "Accept", "A=", "") +
		// 5. E=Accept-Encoding, the compression formats the browser supports.
		headerTxt(r, "Accept-Encoding", "E=", "") +
		// 6. Connection, can be empty, "keep-alive" or "close".
		headerTxt(r, "Connection", "", "") +
		// 7. Cache-Control, how the browser is caching data.
		headerTxt(r, "Cache-Control", "", "") +
		// 8. Upgrade-Insecure-Requests, the browser can upgrade from HTTP to HTTPS
		headerTxt(r, "Upgrade-Insecure-Requests", "UIR=", "1") +
		// 9. Via avoids request loops and identifies protocol capabilities
		headerTxt(r, "Via", "Via=", "") +
		// 10. Authorization and Cookie: both should not be present at the same time
		headerTxt(r, "Authorization", "", "") +
		headerTxt(r, "Cookie", "", "")

	return line
}

func headerTxt(r *http.Request, header, key, skip string) string {
	v := SafeHeader(r, header)
	if v == skip {
		return ""
	}
	return " " + key + v
}

func headerMD(r *http.Request, header string) string {
	v := SafeHeader(r, header)
	if v == "" {
		return ""
	}
	return "\n" + "- **" + header + "**: " + v
}
