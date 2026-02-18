package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/domain"
)

// TestParseMemoryLimit verifies human-readable memory strings are converted to bytes.
func TestParseMemoryLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty string returns zero", input: "", want: 0},
		{name: "zero string returns zero", input: "0", want: 0},
		{name: "megabytes suffix", input: "512m", want: 512 * 1024 * 1024},
		{name: "gigabytes suffix", input: "2g", want: 2 * 1024 * 1024 * 1024},
		{name: "kilobytes suffix", input: "1024k", want: 1024 * 1024},
		{name: "uppercase is case-insensitive", input: "1G", want: 1024 * 1024 * 1024},
		{name: "bare integer treated as bytes", input: "100", want: 100},
		{name: "whitespace is trimmed", input: "  512m  ", want: 512 * 1024 * 1024},
		{name: "non-numeric returns error", input: "abc", wantErr: true},
		{name: "float value returns error", input: "12.5m", wantErr: true},
		{name: "negative value is allowed", input: "-1g", want: -1 * 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseMemoryLimit(tt.input)
			if tt.wantErr {
				require.Error(t, err, "parseMemoryLimit(%q) should return an error", tt.input)
				assert.Zero(t, got, "parseMemoryLimit(%q) should return 0 on error", tt.input)
				return
			}

			require.NoError(t, err, "parseMemoryLimit(%q) unexpected error", tt.input)
			assert.Equal(t, tt.want, got, "parseMemoryLimit(%q) value mismatch", tt.input)
		})
	}
}

// TestParseCPULimit verifies CPU limit strings are converted to Docker CPU quota.
func TestParseCPULimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{name: "empty string returns zero", input: "", want: 0},
		{name: "zero string returns zero", input: "0", want: 0},
		{name: "two cores", input: "2", want: 200_000},
		{name: "half core", input: "0.5", want: 50_000},
		{name: "one core", input: "1", want: 100_000},
		{name: "whitespace is trimmed", input: "  2  ", want: 200_000},
		{name: "non-numeric returns error", input: "abc", wantErr: true},
		{name: "four cores", input: "4", want: 400_000},
		{name: "quarter core", input: "0.25", want: 25_000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCPULimit(tt.input)
			if tt.wantErr {
				require.Error(t, err, "parseCPULimit(%q) should return an error", tt.input)
				assert.Zero(t, got, "parseCPULimit(%q) should return 0 on error", tt.input)
				return
			}

			require.NoError(t, err, "parseCPULimit(%q) unexpected error", tt.input)
			assert.Equal(t, tt.want, got, "parseCPULimit(%q) value mismatch", tt.input)
		})
	}
}

// TestBuildPrompt verifies prompt construction from a domain.Task.
func TestBuildPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task *domain.Task
		want string
	}{
		{
			name: "title only with empty description",
			task: &domain.Task{Title: "Fix bug", Description: ""},
			want: "## Task: Fix bug\n\n",
		},
		{
			name: "title and description",
			task: &domain.Task{Title: "Fix bug", Description: "Detailed description here"},
			want: "## Task: Fix bug\n\nDetailed description here\n",
		},
		{
			name: "empty title",
			task: &domain.Task{Title: "", Description: ""},
			want: "## Task: \n\n",
		},
		{
			name: "long title and long description",
			task: &domain.Task{
				Title:       "Implement the new authentication flow for the OAuth2 provider",
				Description: "We need to integrate a third-party OAuth2 provider to handle user authentication. This includes token refresh, scope validation, and proper error handling for expired sessions.",
			},
			want: "## Task: Implement the new authentication flow for the OAuth2 provider\n\n" +
				"We need to integrate a third-party OAuth2 provider to handle user authentication. " +
				"This includes token refresh, scope validation, and proper error handling for expired sessions.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := buildPrompt(tt.task)
			assert.Equal(t, tt.want, got)
		})
	}
}
