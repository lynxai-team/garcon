// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package wf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lynxai-team/emo"
	"github.com/lynxai-team/garcon/gg"
)

// Notifier interface for sending messages.
type Notifier interface {
	Notify(message []byte) error
}

// NewNotifier selects the Notifier type depending on the parameter pattern.
func NewNotifier(notifierURL string) Notifier {
	if len(notifierURL) == 0 {
		emo.Info("empty dataSourceName => only log the received messages (LogNotifier)")
		return NewLogNotifier()
	}

	const telegramPrefix = "https://api.telegram.org/bot"
	if strings.HasPrefix(notifierURL, telegramPrefix) {
		emo.Info("URL has the Telegram prefix: " + notifierURL)
		p := gg.SplitClean(notifierURL)
		if len(p) == 2 {
			return NewTelegramNotifier(string(p[0]), string(p[1]))
		}

		emo.Error("Cannot retrieve ChatID from %v", p)
		return NewLogNotifier()
	}

	// default
	return NewMattermostNotifier(notifierURL)
}

// LogNotifier implements a Notifier interface that logs the received notifications.
// LogNotifier can be used as a mocked Notifier or for debugging purpose
// or as a fallback when a real Notifier cannot be created for whatever reason.
type LogNotifier struct{}

// NewLogNotifier creates a LogNotifier.
func NewLogNotifier() LogNotifier {
	return LogNotifier{}
}

// Notify prints the messages to the logs.
func (n LogNotifier) Notify(msg []byte) error {
	emo.State("LogNotifier:", gg.SanitizeBytes(msg))
	return nil
}

// MattermostNotifier for sending messages to a Mattermost server.
type MattermostNotifier struct {
	endpoint string
}

// NewMattermostNotifier creates a MattermostNotifier given a Mattermost server endpoint (see mattermost hooks).
func NewMattermostNotifier(endpoint string) MattermostNotifier {
	return MattermostNotifier{endpoint}
}

// Notify sends a message to a Mattermost server.
// It constructs the JSON payload directly with a curated and escaped string.
func (n MattermostNotifier) Notify(msg []byte) error {
	// Allocate a buffer to avoid reallocations (some special characters are escaped such as newlines and tabs).
	buf := make([]byte, 0, len(msg)+len(`{"text":"`)+2+len(msg)/16)

	buf = append(buf, `{"text":"`...)

	// Append sanitized and escaped content in one pass.
	buf = AppendCurateEscape(buf, msg)

	// Close the JSON structure.
	buf = append(buf, '"', '}')

	// Send the request using bytes.NewReader for zero-allocation reading.
	resp, err := http.Post(n.endpoint, "application/json", bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("MattermostNotifier: %w from host=%s", err, n.host())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MattermostNotifier: %s from host=%s", resp.Status, n.host())
	}
	return nil
}

// AppendCurateEscape processes the input slice in a single pass:
// - Sanitizes the input by skipping invalid UTF-8 sequences and unnecessary code-points.
// - Retains only important UTF-8 characters (Graphic and essential Markdown characters).
// - JSON compatibility: Escape some characters: newline, tab, double-quote.
// AppendCurateEscape does not quote: it does not surround with double-quotes.
func AppendCurateEscape(buf []byte, s []byte) []byte {
	for len(s) > 0 {
		// Decode the next rune. utf8.DecodeRune handles multi-byte correctly.
		r, width := utf8.DecodeRune(s)

		// Security & Curation: Skip invalid UTF-8 sequences.
		// utf8.DecodeRune returns width=1 and r==RuneError for invalid bytes.
		if width == 1 && r == utf8.RuneError {
			s = s[1:]
			continue
		}

		// Switch for essential JSON and Markdown escaping.
		switch r {
		case '"': // Escape double quote for JSON.
			buf = append(buf, '\\', '"')
		case '\\': // Escape backslash for JSON.
			buf = append(buf, '\\', '\\')
		case '\n': // Escape newline for JSON (Markdown needs newlines).
			buf = append(buf, '\\', 'n')
		case '\t': // Escape tab for JSON.
			buf = append(buf, '\\', 't')
		default:
			// Retain only graphic = letters, numbers, punctuation, symbols, and spaces.
			// This ensures we only transmit useful characters for the contact form.
			if strconv.IsGraphic(r) {
				buf = append(buf, s[:width]...)
			} else {
				// Skip non-graphic, non-essential runes (security/performance curation).
				// This drops control characters and zero-width chars.
			}
		}

		// Advance the slice by the width of the consumed rune.
		s = s[width:]
	}
	return buf
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
func (n TelegramNotifier) Notify(msg []byte) error {
	response, err := http.PostForm(
		n.endpoint,
		url.Values{
			"chat_id": {n.chatID},
			"text":    {string(msg)},
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
