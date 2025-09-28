// Copyright 2021-2025 The contributors of Garcon.
// This file is part of Garcon, web+API server toolkit under the MIT License.
// SPDX-License-Identifier: MIT

package main

import (
	"flag"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/teal-finance/emo"
	"github.com/teal-finance/garcon"
	"github.com/teal-finance/garcon/gg"
)

var log = emo.NewZone("app")

var port = ":" + gg.EnvStr("MAIN_PORT", "8087")

func main() {
	endpoint := flag.String("post-endpoint", "/", "The endpoint for the POST request.")
	flag.Parse()

	g := garcon.New(garcon.WithServerName("HttpRouterExample"))

	middleware := gg.NewChain(
		g.MiddlewareLogRequest("safe"),
		g.MiddlewareServerHeader(),
	)

	router := httprouter.New()
	router.POST(*endpoint, post)
	router.NotFound = others{}
	router.HandleMethodNotAllowed = false

	handler := middleware.Then(router)

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

type others struct{}

func (others) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	log.Info("router.NotFound")
	w.Write([]byte("<html><body> router.NotFound </body></html>"))
}
