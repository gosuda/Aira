package slack_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	airaslack "github.com/gosuda/aira/internal/messenger/slack"
)

func TestParseCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantAction airaslack.CommandAction
		wantTitle  string
	}{
		{
			name:       "task from @aira mention",
			input:      "@aira fix the auth bug",
			wantAction: airaslack.CommandActionTask,
			wantTitle:  "fix the auth bug",
		},
		{
			name:       "create task with colon prefix",
			input:      "@aira create task: implement login page",
			wantAction: airaslack.CommandActionTask,
			wantTitle:  "implement login page",
		},
		{
			name:       "status command",
			input:      "@aira status",
			wantAction: airaslack.CommandActionStatus,
		},
		{
			name:       "help command",
			input:      "@aira help",
			wantAction: airaslack.CommandActionHelp,
		},
		{
			name:       "link command",
			input:      "@aira link",
			wantAction: airaslack.CommandActionLink,
		},
		{
			name:       "empty input",
			input:      "",
			wantAction: airaslack.CommandActionUnknown,
		},
		{
			name:       "random text without mention",
			input:      "random text without mention",
			wantAction: airaslack.CommandActionUnknown,
		},
		{
			name:       "slack user ID format mention",
			input:      "<@U12345> fix the auth bug",
			wantAction: airaslack.CommandActionTask,
			wantTitle:  "fix the auth bug",
		},
		{
			name:       "slack user ID with status",
			input:      "<@UABC123> status",
			wantAction: airaslack.CommandActionStatus,
		},
		{
			name:       "slack user ID with help",
			input:      "<@U999> help",
			wantAction: airaslack.CommandActionHelp,
		},
		{
			name:       "slack user ID with link",
			input:      "<@U42> link",
			wantAction: airaslack.CommandActionLink,
		},
		{
			name:       "slack user ID with create task colon",
			input:      "<@U12345> create task: implement login page",
			wantAction: airaslack.CommandActionTask,
			wantTitle:  "implement login page",
		},
		{
			name:       "whitespace only",
			input:      "   ",
			wantAction: airaslack.CommandActionUnknown,
		},
		{
			name:       "raw field preserved",
			input:      "@aira status",
			wantAction: airaslack.CommandActionStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := airaslack.ParseCommand(tt.input)

			assert.Equal(t, tt.wantAction, cmd.Action)
			assert.Equal(t, tt.wantTitle, cmd.Title)
			assert.Equal(t, tt.input, cmd.Raw)
		})
	}
}
