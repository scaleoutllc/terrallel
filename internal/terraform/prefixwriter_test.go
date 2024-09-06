package terraform

import (
	"bytes"
	"errors"
	"testing"
)

// A mock writer to simulate write errors
type errorWriter struct{}

func (e *errorWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("mock write error")
}

func TestPrefixWriter(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		prefix   string
		input    string
		expected string
	}{
		{
			name:     "Single line with newline",
			prefix:   "[prefix] ",
			input:    "test line\n",
			expected: "[prefix] test line\n",
		},
		{
			name:     "Multiple lines with newlines",
			prefix:   "[prefix] ",
			input:    "line 1\nline 2\nline 3\n",
			expected: "[prefix] line 1\n[prefix] line 2\n[prefix] line 3\n",
		},
		{
			name:     "Line without newline",
			prefix:   "[prefix] ",
			input:    "partial line",
			expected: "[prefix] partial line",
		},
		{
			name:     "Mixed complete and incomplete lines",
			prefix:   "[prefix] ",
			input:    "line 1\npartial line",
			expected: "[prefix] line 1\n[prefix] partial line",
		},
		{
			name:     "Empty input",
			prefix:   "[prefix] ",
			input:    "",
			expected: "",
		},
		{
			name:     "Empty prefix",
			prefix:   "",
			input:    "test line\n",
			expected: "test line\n",
		},
		{
			name:     "Newline within prefix",
			prefix:   "[prefix]\n",
			input:    "test line\n",
			expected: "[prefix]\ntest line\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			pw := prefixWriter(&buf, tt.prefix)
			n, err := pw.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write() wrote %d bytes, expected %d", n, len(tt.input))
			}
			output := buf.String()
			if output != tt.expected {
				t.Errorf("Expected output %q, got %q", tt.expected, output)
			}
		})
	}
}

func TestPrefixWriterFlushBuffer(t *testing.T) {
	t.Run("Flush buffer on incomplete line", func(t *testing.T) {
		var buf bytes.Buffer
		pw := prefixWriter(&buf, "[prefix] ")
		_, _ = pw.Write([]byte("incomplete"))
		err := pw.flushBuffer()
		if err != nil {
			t.Fatalf("flushBuffer() error = %v", err)
		}

		expected := "[prefix] incomplete"
		output := buf.String()
		if output != expected {
			t.Errorf("Expected output %q, got %q", expected, output)
		}
	})

	t.Run("No flush needed for empty buffer", func(t *testing.T) {
		var buf bytes.Buffer
		pw := prefixWriter(&buf, "[prefix] ")
		err := pw.flushBuffer()
		if err != nil {
			t.Fatalf("flushBuffer() error = %v", err)
		}
		output := buf.String()
		if output != "" {
			t.Errorf("Expected empty output, got %q", output)
		}
	})
}

func TestPrefixWriterOutput(t *testing.T) {
	t.Run("Output after writing data", func(t *testing.T) {
		var buf bytes.Buffer
		pw := prefixWriter(&buf, "[prefix] ")
		_, _ = pw.Write([]byte("line 1\nline 2\n"))
		_, _ = pw.Write([]byte("line 3\n"))

		expectedOutput := "line 1\nline 2\nline 3\n"
		output := pw.Output()
		if output != expectedOutput {
			t.Errorf("Expected output %q, got %q", expectedOutput, output)
		}
	})

	t.Run("Output after no data written", func(t *testing.T) {
		var buf bytes.Buffer
		pw := prefixWriter(&buf, "[prefix] ")
		output := pw.Output()
		if output != "" {
			t.Errorf("Expected empty output, got %q", output)
		}
	})
}

func TestPrefixWriterErrorHandling(t *testing.T) {
	t.Run("Write error handling", func(t *testing.T) {
		pw := prefixWriter(&errorWriter{}, "[prefix] ")
		_, err := pw.Write([]byte("line 1\n"))
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})

	t.Run("Flush buffer with write error", func(t *testing.T) {
		pw := prefixWriter(&errorWriter{}, "[prefix] ")
		_, _ = pw.Write([]byte("incomplete"))
		err := pw.flushBuffer()
		if err == nil {
			t.Fatalf("Expected error, got nil")
		}
	})
}
