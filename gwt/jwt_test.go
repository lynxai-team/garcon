// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gwt_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LynxAIeu/garcon/gg"
	"github.com/LynxAIeu/garcon/gwt"
)

type next struct {
	called bool
	perm   int
}

func (next *next) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	next.called = true
	next.perm = gwt.PermFromCtx(r).Value
}

func TestNewJWTChecker(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		gw           gg.Writer
		secretHex    string
		cookieNameIn string
		cookieName   string
		plan         string
		addresses    []string
		permissions  []any
		perm         int
		shouldPanic  bool
	}{{
		name:         "0plans",
		addresses:    []string{"http://my-dns.co"},
		gw:           gg.NewWriter("http://my-dns.co/doc"),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  nil,
		cookieNameIn: "",
		cookieName:   "my-dns",
		plan:         gwt.DefaultPlan,
		perm:         gwt.DefaultPerm,
		shouldPanic:  false,
	}, {
		name:         "name-only",
		addresses:    []string{"http://my-dns.co"},
		gw:           gg.NewWriter("http://my-dns.co/doc"),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{},
		cookieNameIn: "my-cookie-name",
		cookieName:   "my-cookie-name",
		plan:         gwt.DefaultPlan,
		perm:         gwt.DefaultPerm,
		shouldPanic:  false,
	}, {
		name:         "1plans",
		addresses:    []string{"https://sub.dns.co/"},
		gw:           gg.NewWriter("http://my-dns.co/doc"),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{"Anonymous", 6},
		cookieNameIn: "",
		cookieName:   "__Host-sub-dns",
		plan:         "Anonymous",
		perm:         6,
		shouldPanic:  false,
	}, {
		name:         "bad-plans",
		addresses:    []string{"http://my-dns.co/dir"},
		gw:           gg.NewWriter("doc"),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{"Anonymous", 6, "Personal"}, // len(permissions) is not even => panic
		cookieNameIn: "",
		cookieName:   "dir",
		plan:         "error",
		perm:         666,
		shouldPanic:  true,
	}, {
		name:         "3plans",
		addresses:    []string{"http://sub.dns.co//./sss/..///-_-my.dir_-_.jpg///"},
		gw:           gg.NewWriter("/doc"),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{"Anonymous", 6, "Personal", 48, "Enterprise", 0},
		cookieNameIn: "",
		cookieName:   "my-dir",
		plan:         "Personal",
		perm:         48,
		shouldPanic:  false,
	}, {
		name:         "localhost",
		addresses:    []string{"http://localhost:8080/"},
		gw:           gg.NewWriter(""),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{"Anonymous", 6, "Personal", 48, "Enterprise", -1},
		cookieNameIn: "",
		cookieName:   gwt.DefaultCookieName,
		plan:         "Enterprise",
		perm:         -1,
		shouldPanic:  false,
	}, {
		name:         "customPlan",
		addresses:    []string{"https://my-dns.co:8080/my/sub/-_-my.dir_-_.jpg/"},
		gw:           gg.NewWriter(""),
		secretHex:    "0a02123112dfb13d58a1bc0c8ce55b154878085035ae4d2e13383a79a3e3de1b",
		permissions:  []any{"Anonymous", 6, "Personal", 48, "Enterprise", -1},
		cookieNameIn: "",
		cookieName:   "__Secure-my-dir",
		plan:         "55",
		perm:         55,
		shouldPanic:  false,
	}}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			urls := gg.ParseURLs(c.addresses)

			if c.shouldPanic {
				defer func() { _ = recover() }()
			}

			ck := gwt.NewJWTChecker(c.gw, urls, c.secretHex, c.cookieNameIn, c.permissions...)

			if c.shouldPanic {
				t.Errorf("#%d NewChecker() did not panic", i)
			}

			if ck.Cookie(0).Name != c.cookieName {
				t.Errorf("#%d Want cookieName %q but got %q", i, c.cookieName, ck.Cookie(0).Name)
			}

			tokenizer, err := gwt.NewHMAC(c.secretHex, true)
			if err != nil {
				t.Fatalf("#%d gwt.NewHMAC err: %s", i, err)
			}
			cookie := gwt.NewCookie(tokenizer, c.cookieName, c.plan, "John Doe", false, c.addresses[0][6:], "/")

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
			}
			req.AddCookie(&cookie)

			w := httptest.NewRecorder()
			next := &next{
				called: false,
				perm:   0,
			}
			handler := ck.Chk(next)
			handler.ServeHTTP(w, req)

			body := w.Body.String()
			if body != "" {
				t.Fatalf("#%d checker.Chk() %s", i, body)
			}
			if !next.called {
				t.Errorf("#%d checker.Chk() has not called next.ServeHTTP()", i)
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Chk() request ctx perm got=%d want=%d", i, next.perm, c.perm)
			}

			req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
			}
			req.AddCookie(&cookie)

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Vet(next)
			handler.ServeHTTP(w, req)

			body = w.Body.String()
			if body != "" {
				t.Fatalf("#%d checker.Vet() %s", i, body)
			}
			if !next.called {
				t.Errorf("#%d checker.Vet() has not called next.ServeHTTP()", i)
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Vet() request ctx perm got=%d want=%d", i, next.perm, c.perm)
			}

			req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, c.addresses[0], http.NoBody)
			if err != nil {
				t.Fatalf("#%d NewRequestWithContext err: %s", i, err)
			}

			w = httptest.NewRecorder()
			next.called = false
			next.perm = 0
			handler = ck.Set(next)
			handler.ServeHTTP(w, req)

			resp := w.Result()
			cookies := resp.Cookies()
			if len(cookies) != 1 {
				t.Errorf("#%d wants only one cookie, but checker.Set() provided %d", i, len(cookies))
			} else if cookies[0].Value != ck.Cookie(0).Value {
				t.Errorf("#%d checker.Set() has not used the first cookie", i)
			}
			if !next.called {
				t.Errorf("#%d checker.Vet() has not called next.ServeHTTP()", i)
			}
			if len(c.permissions) >= 2 {
				var ok bool
				c.perm, ok = c.permissions[1].(int)
				if !ok {
					t.Errorf("#%d c.permissions[1] is not int", i)
				}
			}
			if next.perm != c.perm {
				t.Errorf("#%d checker.Set() request ctx perm got=%d want=%d "+
					"len(permissions)=%d", i, next.perm, c.perm, len(c.permissions))
			}
			err = resp.Body.Close()
			if err != nil {
				t.Fatalf("#%d resp.Body.Close err: %s", i, err)
			}
		})
	}
}
