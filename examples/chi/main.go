// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, a static web builder, API server and middleware using Git, docker and podman.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"net/http"

	"github.com/LynxAIeu/garcon/gc"
	"github.com/LynxAIeu/garcon/gg"

	"github.com/LynxAIeu/emo"
	"github.com/go-chi/chi/v5"
)

var (
	log = emo.NewZone("app")

	port = ":" + gg.EnvStr("MAIN_PORT", "8086")
)

func main() {
	endpoint := flag.String("post-endpoint", "/", "The endpoint for the POST request.")
	flag.Parse()

	g := gc.New(gc.WithServerName("ChiExample"))

	middleware := gg.NewChain(
		g.MiddlewareLogRequest("safe"),
		g.MiddlewareServerHeader(),
	)

	router := chi.NewRouter()
	router.Post(*endpoint, post)
	router.MethodNotAllowed(others) // handle other methods of the above POST endpoint
	router.NotFound(others)         // handle all other endpoints

	handler := middleware.Then(router)

	server := http.Server{
		Addr:    port,
		Handler: handler,
	}

	log.Init("-------------- Open http://localhost" + server.Addr + *endpoint + " --------------")
	log.Fatal(server.ListenAndServe())
}

func post(w http.ResponseWriter, _ *http.Request) {
	log.Info("router.Post()")
	w.Write([]byte("<html><body> router.Post() </body></html>"))
}

func others(w http.ResponseWriter, _ *http.Request) {
	log.Info("router.NotFound()")
	w.Write([]byte("<html><body> router.MethodNotAllowed() </body></html>"))
}
