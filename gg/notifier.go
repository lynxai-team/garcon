// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package gg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type (
	// Notifier interface for sending messages.
	Notifier interface {
		Notify(message string) error
	}

	// LogNotifier implements a Notifier interface that logs the received notifications.
	// LogNotifier can be used as a mocked Notifier or for debugging purpose
	// or as a fallback when a real Notifier cannot be created for whatever reason.
	LogNotifier struct{}

	// MattermostNotifier for sending messages to a Mattermost server.
	MattermostNotifier struct {
		endpoint string
	}
)

// NewMattermostNotifier creates a MattermostNotifier given a Mattermost server endpoint (see mattermost hooks).
func NewMattermostNotifier(endpoint string) MattermostNotifier {
	return MattermostNotifier{endpoint}
}

// NewNotifier selects the Notifier type depending on the parameter pattern.
func NewNotifier(dataSourceName string) Notifier {
	if dataSourceName == "" {
		log.Info("empty dataSourceName => use the LogNotifier")
		return NewLogNotifier()
	}

	const telegramPrefix = "https://api.telegram.org/bot"
	if strings.HasPrefix(dataSourceName, telegramPrefix) {
		log.Info("URL has the Telegram prefix: " + dataSourceName)
		p := SplitClean(dataSourceName)
		if len(p) == 2 {
			return NewTelegramNotifier(p[0], p[1])
		}

		log.Error("Cannot retrieve ChatID from %v", p)
		return NewLogNotifier()
	}

	// default
	return NewMattermostNotifier(dataSourceName)
}

// NewLogNotifier creates a LogNotifier.
func NewLogNotifier() LogNotifier {
	return LogNotifier{}
}

// Notify prints the messages to the logs.
func (n LogNotifier) Notify(msg string) error {
	log.State("LogNotifier:", sanitize(msg))
	return nil
}

// Notify sends a message to a Mattermost server.
func (n MattermostNotifier) Notify(msg string) error {
	buf := strconv.AppendQuoteToGraphic([]byte(`{"text":`), msg)
	buf = append(buf, byte('}'))
	body := bytes.NewBuffer(buf)

	resp, err := http.Post(n.endpoint, "application/json", body)
	if err != nil {
		return fmt.Errorf("MattermostNotifier: %w from host=%s", err, n.host())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MattermostNotifier: %s from host=%s", resp.Status, n.host())
	}
	return nil
}

func (n MattermostNotifier) host() string {
	u, err := url.Parse(n.endpoint)
	if err == nil {
		return u.Hostname()
	}
	return ""
}

// TelegramNotifier is a Notifier for a specific Telegram chat room.
type TelegramNotifier struct {
	endpoint string
	chatID   string
}

// NewTelegramNotifier creates a TelegramNotifier.
func NewTelegramNotifier(endpoint, chatID string) TelegramNotifier {
	return TelegramNotifier{
		endpoint: endpoint,
		chatID:   chatID,
	}
}

// Notify sends a message to the Telegram server.
func (n TelegramNotifier) Notify(msg string) error {
	response, err := http.PostForm(
		n.endpoint,
		url.Values{
			"chat_id": {n.chatID},
			"text":    {msg},
		})
	if err != nil {
		return fmt.Errorf("TelegramNotifier chat_id=%s: %w", n.chatID, err)
	}

	defer response.Body.Close()

	var resp telegramResponse
	err = json.NewDecoder(response.Body).Decode(&resp)
	if err != nil {
		return fmt.Errorf("TelegramNotifier chat_id=%s: %w", n.chatID, err)
	}

	if !resp.Ok {
		return fmt.Errorf("TelegramNotifier chat_id=%s: sending failed", n.chatID)
	}

	return nil
}

type telegramResponse struct {
	Result struct {
		Text string `json:"text"`
		Chat struct {
			Title string `json:"title"`
			Type  string `json:"type"`
			ID    int64  `json:"id"`
		} `json:"chat"`
		From struct {
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
			ID        int    `json:"id"`
			IsBot     bool   `json:"is_bot"`
		} `json:"from"`
		MessageID int `json:"message_id"`
		Date      int `json:"date"`
	} `json:"result"`
	Ok bool `json:"ok"`
}
