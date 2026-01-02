// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gg_test

import (
	"testing"

	"github.com/LynxAIeu/garcon/gg"
)

func TestNotifier_Notify(t *testing.T) {
	t.Parallel()

	url := "https://framateam.org/hooks/your-mattermost-hook-url"
	n := gg.NewNotifier(url)
	err := n.Notify("Hello, world!")

	want := "MattermostNotifier: 404 Not Found from host=framateam.org"
	if err.Error() != want {
		t.Error("got:  " + err.Error())
		t.Error("want: " + want)
	}
}
