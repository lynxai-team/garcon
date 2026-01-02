// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"net/http"

	"github.com/LynxAIeu/garcon/gc"
	"github.com/LynxAIeu/garcon/gg"

	"github.com/LynxAIeu/emo"
	"github.com/julienschmidt/httprouter"
)

type others struct{}

var log = emo.NewZone("app")

func (others) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	log.Info("router.NotFound")
	w.Write([]byte("<html><body> router.NotFound </body></html>"))
}

func main() {
	endpoint := flag.String("post-endpoint", "/", "The endpoint for the POST request.")
	flag.Parse()

	g := gc.New(gc.WithServerName("HttpRouterExample"))

	middleware := gg.NewChain(
		g.MiddlewareLogRequest("safe"),
		g.MiddlewareServerHeader(),
	)

	router := httprouter.New()
	router.POST(*endpoint, post)
	router.NotFound = others{}
	router.HandleMethodNotAllowed = false

	handler := middleware.Then(router)

	port := ":" + gg.EnvStr("MAIN_PORT", "8087")

	server := http.Server{
		Addr:    port,
		Handler: handler,
	}

	log.Init("-------------- Open http://localhost" + server.Addr + *endpoint + " --------------")
	log.Fatal(server.ListenAndServe())
}

func post(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	log.Info("router.Post")
	w.Write([]byte("<html><body> router.Post </body></html>"))
}
