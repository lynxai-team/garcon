// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, a static web builder, API server and middleware using Git, docker and podman.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"net/http"
	"strconv"

	"github.com/LynxAIeu/garcon/gc"
	"github.com/LynxAIeu/garcon/gg"
	"github.com/LynxAIeu/garcon/vv"

	"github.com/LynxAIeu/emo"
	"github.com/go-chi/chi/v5"
)

// Garcon settings.
const (
	// The HMAC-SHA256 key to decode JWT (do not put your secret keys in your code).
	hmacSHA256 = "9d2e0a02121179a3c3de1b035ae1355b1548781c8ce8538a1dc0853a12dfb13d"
	aes128bits = "00112233445566778899aabbccddeeff"
)

var (
	log = emo.NewZone("app")

	mainPort  = gg.EnvInt("MAIN_PORT", 8084)
	pprofPort = gg.EnvInt("PPROF_PORT", 8094)
	expPort   = gg.EnvInt("EXP_PORT", 9094)
)

func main() {
	defer gc.ProbeCPU().Stop() // collects the CPU-profile and writes it in the file "cpu.pprof"

	vv.LogVersion()
	vv.SetVersionFlag()
	// TODO disable --- auth := flag.Bool("auth", false, "Enable OPA authorization specified in file "+opaFile)
	prod := flag.Bool("prod", false, "Use settings for production")
	jwt := flag.Bool("jwt", false, "Use JWT in lieu of the Incorruptible token")
	flag.Parse()

	var addr string
	if *prod {
		addr = "https://my-dns.co/myapp"
	} else {
		addr = "http://localhost:" + strconv.Itoa(mainPort) + "/myapp"
	}

	g := gc.New(
		gc.WithURLs(addr),
		gc.WithDocURL("/doc"),
		gc.WithPProf(pprofPort),
		gc.WithDev(!*prod),
		nil, // just to test "none" option
	)

	var ck gc.TokenChecker
	if *jwt {
		ck = g.JWTChecker(hmacSHA256, "FreePlan", 10, "PremiumPlan", 100)
	} else {
		ck = g.IncorruptibleChecker(aes128bits, 60, true)
	}

	middleware, connState := g.StartExporter(expPort,
		gc.WithLivenessProbes(func() []byte { return nil }),
		gc.WithLivenessProbes(func() []byte { return nil }),
		gc.WithLivenessProbes(func() []byte { return nil }),
		gc.WithReadinessProbes(func() []byte { return nil }),
		gc.WithReadinessProbes(func() []byte { return []byte("fail") }))
	middleware = middleware.Append(
		g.MiddlewareRejectUnprintableURI(),
		g.MiddlewareLogRequest("fingerprint"),
		g.MiddlewareRateLimiter(),
		g.MiddlewareServerHeader("MyApp"),
		g.MiddlewareCORS(),
		g.MiddlewareLogDuration(true))

	// TODO disable --- if *auth {
	// TODO disable --- 	middleware = middleware.Append(g.MiddlewareOPA(opaFile))
	// TODO disable --- }

	// handles both REST API and static web files
	r := handler(g, addr, ck)
	h := middleware.Then(r)

	server := gc.Server(h, mainPort, connState)

	log.Init("-------------- Open http://localhost" + server.Addr + "/myapp --------------")
	err := gc.ListenAndServe(&server)
	log.Error(err)
}

// handler creates the mapping between the endpoints and the handler functions.
func handler(g *gc.Garcon, addr string, ck gc.TokenChecker) http.Handler {
	r := chi.NewRouter()

	// Static website files
	ws := g.NewStaticWebServer("examples/www")
	r.Get("/favicon.ico", ws.ServeFile("favicon.ico", "image/x-icon"))
	r.With(ck.Set).Get("/myapp", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(ck.Set).Get("/myapp/", ws.ServeFile("myapp/index.html", "text/html; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/js/*", ws.ServeDir("text/javascript; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/css/*", ws.ServeDir("text/css; charset=utf-8"))
	r.With(ck.Chk).Get("/myapp/images/*", ws.ServeImages())
	r.With(ck.Chk).Get("/myapp/version", vv.ServeVersion())

	// Contact-form
	wf := g.NewContactForm(addr)
	r.With(ck.Set).Post("/myapp", wf.Notify(""))

	// API
	r.With(ck.Vet).Get("/path/not/in/cookie", items)
	r.With(ck.Vet).Get("/myapp/api/v1/items", items)
	r.With(ck.Vet).Get("/myapp/api/v1/ducks", g.Writer.NotImplemented)

	// Other endpoints
	r.NotFound(g.Writer.InvalidPath)

	return r
}

func items(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`["item1","item2","item3"]`))
}
