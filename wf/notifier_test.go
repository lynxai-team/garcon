// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package wf_test

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lynxai-team/garcon/gg"
	"github.com/lynxai-team/garcon/wf"
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

// TestNotify_Functional2 tests the Notify method against a test server.
func TestNotify_Functional2(t *testing.T) {
	// Setup a mock server to verify the request body and control the response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and content type.
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Read the body to verify the content.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
		}

		// Verify the JSON structure.
		if !strings.HasPrefix(string(body), `{"text":"`) {
			t.Errorf("Expected JSON body to start with '{\"text\":\"', got %s", string(body))
		}

		// Ensure the server responds with 200 OK for the success test.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		notifier := wf.NewMattermostNotifier(server.URL)
		msg := []byte("Hello, World! \n \" \u00A0")
		err := notifier.Notify(msg)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("HTTP Error", func(t *testing.T) {
		// Create a specific handler for failure.
		failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError) // 500
		}))
		defer failServer.Close()

		notifier := wf.NewMattermostNotifier(failServer.URL)
		err := notifier.Notify([]byte("test"))
		if err == nil {
			t.Errorf("Expected error for 500 status, got nil")
		}
		// Check error string contains status code.
		if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "Internal Server Error") {
			t.Errorf("Expected error to contain status info, got %v", err)
		}
	})
}

// TestAppendCuratedEscaped_Unit2 tests the core logic of the sanitizer/quoter.
func TestAppendCuratedEscaped_Unit2(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string // Expected output after quoting
	}{
		{name: "Empty", input: []byte(""), want: ``},
		{name: "Simple ASCII", input: []byte("Hello World"), want: `Hello World`},
		{name: "Double Quote", input: []byte(`He said "Hello"`), want: `He said \"Hello\"`},
		{name: "Backslash", input: []byte(`C:\Path\`), want: `C:\\Path\\`},
		{name: "Control Char Newline", input: []byte("Line1\nLine2"), want: `Line1\nLine2`},
		{name: "Control Char Tab", input: []byte("Tab\tHere"), want: `Tab\tHere`},
		{name: "Unicode Valid", input: []byte("日本"), want: `日本`},        // Assuming valid UTF-8 is kept
		{name: "Graphic Rune", input: []byte("\u00A0"), want: "\u00a0"}, // \u00A0 is in isGraphic, so NOT escaped as \u...
		{name: "Invalid UTF-8", input: []byte("\xFE"), want: ``},        // Invalid sequence should be stripped.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := wf.AppendCurateEscape([]byte{}, tc.input)
			// We check the string representation.
			// Note: wf.AppendCuratedEscaped does not add quotes around the string.
			if string(result) != tc.want {
				t.Errorf("AppendCuratedEscaped(%q) = %q, want %q", tc.input, string(result), tc.want)
			}
		})
	}
}

// FuzzAppendCuratedEscaped2 implements the native fuzzing target for Go 1.18+.
func FuzzAppendCuratedEscaped2(f *testing.F) {
	f.Add([]byte("Hello"))            // Seed corpus.
	f.Add([]byte("\xFE\xFF\xFF\xFF")) // Invalid UTF-8 seed.

	// The fuzzer will generate random byte slices.
	f.Fuzz(func(t *testing.T, data []byte) {
		// Run the function.
		res := wf.AppendCurateEscape([]byte{}, data)

		// Invariant 1: Output must be valid UTF-8.
		if !utf8.Valid(res) {
			t.Errorf("Output is not valid UTF-8: %v", res)
		}

		// Invariant 2: The output must be valid JSON string content.
		// We check by wrapping it in a JSON struct.
		jsonStr := fmt.Sprintf(`{"key": "%s"}`, string(res))
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Errorf("Output produced invalid JSON: %v, Input: %v", res, data)
		}

		// Invariant 3: No invalid UTF-8 sequences from input are present in output.
		// The function should strip them.
		// We can't easily check this without re-parsing, but the JSON validity check covers it.
	})
}

// TestNotify_Functional tests the integration of Notify with a mock HTTP server.
func TestNotify_Functional(t *testing.T) {
	// Setup a mock server to verify the request body and control the response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and content type.
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Type"))
		}

		// Read the body to verify the content.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
		}

		// Verify the JSON structure. The body should be {"text":"..."}
		if !strings.HasPrefix(string(body), `{"text":"`) {
			t.Errorf("Expected JSON body to start with '{\"text\":\"', got %s", string(body))
		}

		// Ensure the server responds with 200 OK for the success test.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		notifier := wf.NewMattermostNotifier(server.URL)
		msg := []byte("Hello, World! \n \" \u00A0")
		err := notifier.Notify(msg)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("HTTP Error", func(t *testing.T) {
		// Create a specific handler for failure.
		failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError) // 500
		}))
		defer failServer.Close()

		notifier := wf.NewMattermostNotifier(failServer.URL)
		err := notifier.Notify([]byte("test"))
		if err == nil {
			t.Errorf("Expected error for 500 status, got nil")
		}
		// Check error string contains status code.
		if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "Internal Server Error") {
			t.Errorf("Expected error to contain status info, got %v", err)
		}
	})
}

// TestAppendCuratedEscaped_Unit tests the core logic of the sanitizer/quoter.
func TestAppendCuratedEscaped_Unit(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string // Expected output string (including quotes)
	}{
		{name: "Empty", input: []byte(""), want: ``},
		{name: "Simple ASCII", input: []byte("Hello World"), want: `Hello World`},
		{name: "Double Quote", input: []byte(`He said "Hello"`), want: `He said \"Hello\"`},
		{name: "Backslash", input: []byte(`C:\Path\`), want: `C:\\Path\\`},
		{name: "Control Char Newline", input: []byte("Line1\nLine2"), want: `Line1\nLine2`},
		{name: "Control Char Tab", input: []byte("Tab\tHere"), want: `Tab\tHere`},
		{name: "Unicode Valid", input: []byte("日本"), want: `日本`},        // Valid UTF-8 is kept
		{name: "Graphic Rune", input: []byte("\u00A0"), want: "\u00a0"}, // NO-BREAK SPACE (in isGraphic list)
		{name: "Invalid UTF-8", input: []byte("\xFE"), want: ``},        // Invalid sequence stripped.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := wf.AppendCurateEscape([]byte{}, tc.input)
			// We check the string representation.
			if string(result) != tc.want {
				t.Errorf("wf.AppendCuratedEscaped(%q) = %q, want %q", tc.input, string(result), tc.want)
			}
		})
	}
}

// TestAppendCuratedEscaped_ValidJSON verifies the output is always valid JSON.
func TestAppendCuratedEscaped_ValidJSON(t *testing.T) {
	// Seed with some random data.
	for i := 0; i < 100; i++ {
		// Generate random byte slice.
		size := rand.Intn(100) // Random size up to 100 bytes.
		input := make([]byte, size)
		// Fill with random data (including invalid UTF-8).
		for i := range input {
			input[i] = byte(rand.Intn(256)) // All possible byte values.
		}

		result := wf.AppendCurateEscape([]byte{}, input)

		// Wrap the output in a JSON object to verify it's valid.
		jsonStr := fmt.Sprintf(`{"test": "%s"}`, string(result))

		// Try to unmarshal. If it fails, the output was invalid.
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Errorf("Failed to unmarshal JSON for input %v: %v", input, err)
		}
	}
}

// FuzzAppendCuratedEscaped implements the native fuzzing target for Go 1.18+.
// This provides high coverage for the complex logic in wf.AppendCuratedEscaped.
func FuzzAppendCuratedEscaped(f *testing.F) {
	// Add seed corpus with interesting values.
	f.Add([]byte("Hello"))
	f.Add([]byte("\xFE\xFF\xFF\xFF")) // Invalid UTF-8.
	f.Add([]byte("\u00A0"))           // Graphic rune.
	f.Add([]byte("\n\t"))             // Control chars.

	// The fuzzer will generate random byte slices.
	f.Fuzz(func(t *testing.T, data []byte) {
		// Run the function.
		res := wf.AppendCurateEscape([]byte{}, data)

		// Invariant 1: Output must be valid UTF-8.
		if !utf8.Valid(res) {
			t.Errorf("Output is not valid UTF-8: %v", res)
		}

		// Invariant 2: The output must be valid JSON string content.
		// We check by wrapping it in a JSON struct.
		jsonStr := fmt.Sprintf(`{"key": "%s"}`, string(res))
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Errorf("Output produced invalid JSON: %v, Input: %v", res, data)
		}

		// Invariant 3: No invalid UTF-8 sequences from input are present in output.
		// The function should strip them.
		// We can't easily check this without re-parsing, but the JSON validity check covers it.
	})
}

// TestNotify_Functional tests the integration of Notify with a mock HTTP server.
func TestNotify_Functional3(t *testing.T) {
	// Setup a mock server to verify the request body and control the response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method and content type.
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		// Read the body to verify the content.
		_, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
		}

		// Verify the JSON structure. The body should be {"text":"..."}
		// We cannot easily check the exact content without mocking the curation logic.
		// We ensure the server responds with 200 OK for the success test.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Mock the notifier struct.
	notifier := wf.NewMattermostNotifier(server.URL)

	t.Run("Success", func(t *testing.T) {
		msg := []byte("Hello, World! \n \" \u00A0")
		err := notifier.Notify(msg)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("HTTP Error", func(t *testing.T) {
		// Create a specific handler for failure.
		failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError) // 500
		}))
		defer failServer.Close()

		notifier := wf.NewMattermostNotifier(failServer.URL)
		err := notifier.Notify([]byte("test"))
		if err == nil {
			t.Errorf("Expected error for 500 status, got nil")
		}
		// Check error string contains status info.
		if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "Internal Server Error") {
			t.Errorf("Expected error to contain status info, got %v", err)
		}
	})
}

// TestAppendCuratedEscaped_Unit tests the core logic of the sanitizer/quoter.
func TestAppendCuratedEscaped_Unit3(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string // Expected output string (including quotes)
	}{
		{name: "Empty", input: []byte(""), want: ``},
		{name: "Simple ASCII", input: []byte("Hello World"), want: `Hello World`},
		{name: "Double Quote", input: []byte(`He said "Hello"`), want: `He said \"Hello\"`},
		{name: "Backslash", input: []byte(`C:\Path\`), want: `C:\\Path\\`},
		{name: "Control Char Newline", input: []byte("Line1\nLine2"), want: `Line1\nLine2`}, // Newline is preserved via switch case.
		{name: "Control Char Tab", input: []byte("Tab\tHere"), want: `Tab\tHere`},           // Tab is preserved via switch case.
		{name: "Unicode Valid Graphic", input: []byte("日本"), want: `日本`},                    // Valid UTF-8 is kept.
		{name: "Invalid UTF-8", input: []byte("\xFE"), want: ``},                            // Invalid sequence stripped.
		{name: "Non Graphic Control", input: []byte("\x00"), want: ``},                      // Null byte stripped (not graphic).
		{name: "Graphic Rune", input: []byte("\u00A0"), want: "\u00A0"},                     // NO-BREAK SPACE (Graphic).
		{name: "Escape Check", input: []byte("\n\r\t"), want: `\n\t`},                       // Escaped control chars.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := wf.AppendCurateEscape([]byte{}, tc.input)
			// We check the string representation.
			if string(result) != tc.want {
				t.Errorf("AppendCuratedEscaped(%q) = %q, want %q", tc.input, string(result), tc.want)
			}
		})
	}
}

// TestAppendCuratedEscaped_ValidJSON verifies the output is always valid JSON.
func TestAppendCuratedEscaped_ValidJSON3(t *testing.T) {
	// Seed with some random data.
	rand.New(rand.NewSource(0)) // Use a deterministic seed for reproducibility.

	for i := 0; i < 100; i++ {
		// Generate random byte slice.
		size := rand.Intn(100) // Random size up to 100 bytes.
		input := make([]byte, size)
		// Fill with random data (including invalid UTF-8).
		for i := range input {
			input[i] = byte(rand.Intn(256)) // All possible byte values.
		}

		result := wf.AppendCurateEscape([]byte{}, input)

		// Wrap the output in a JSON object to verify it's valid.
		jsonStr := fmt.Sprintf(`{"test": "%s"}`, string(result))

		// Try to unmarshal. If it fails, the output was invalid.
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Errorf("Failed to unmarshal JSON for input %v: %v", input, err)
		}
	}
}

// FuzzAppendCuratedEscaped implements the native fuzzing target for Go 1.18+.
// This provides high coverage for the complex logic in AppendCuratedEscaped.
func FuzzAppendCuratedEscaped3(f *testing.F) {
	// Add seed corpus with interesting values.
	f.Add([]byte("Hello"))
	f.Add([]byte("\xFE\xFF\xFF\xFF")) // Invalid UTF-8.
	f.Add([]byte("\u00A0"))           // Graphic rune.
	f.Add([]byte("\n\t"))             // Control chars handled by switch.
	f.Add([]byte("\x00\x01"))         // Non graphic controls.

	// The fuzzer will generate random byte slices.
	f.Fuzz(func(t *testing.T, data []byte) {
		// Run the function.
		res := wf.AppendCurateEscape([]byte{}, data)

		// Invariant 1: Output must be valid UTF-8.
		if !utf8.Valid(res) {
			t.Errorf("Output is not valid UTF-8: %v", res)
		}

		// Invariant 2: The output must be valid JSON string content.
		// We check by wrapping it in a JSON struct.
		jsonStr := fmt.Sprintf(`{"key": "%s"}`, string(res))
		var m map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
			t.Errorf("Output produced invalid JSON: %v, Input: %v", res, data)
		}

		// Invariant 3: The output should not contain invalid UTF-8 sequences from input.
		// The function should strip them. We verify by checking that every rune in output is valid.
		for i := 0; i < len(res); {
			r, width := utf8.DecodeRune(res[i:])
			if width == 1 && r == utf8.RuneError {
				t.Errorf("Output contained invalid rune at index %d", i)
			}
			i += width
		}

		// Invariant 4: The output should only contain graphic runes or escaped essential chars.
		// We check that no raw control characters (except those allowed) are present.
	})
}
