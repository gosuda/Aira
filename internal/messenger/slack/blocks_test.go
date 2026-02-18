package slack_test

import (
	"testing"

	slacklib "github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gosuda/aira/internal/messenger"
	airaslack "github.com/gosuda/aira/internal/messenger/slack"
)

func TestBuildQuestionBlocks(t *testing.T) {
	t.Parallel()

	t.Run("with options returns text section and action buttons", func(t *testing.T) {
		t.Parallel()

		opts := []messenger.QuestionOption{
			{Label: "PostgreSQL", Value: "pg"},
			{Label: "MySQL", Value: "mysql"},
			{Label: "SQLite", Value: "sqlite"},
		}
		blocks := airaslack.BuildQuestionBlocks("Which database should we use?", opts)

		require.GreaterOrEqual(t, len(blocks), 2, "should have at least text section + action block")

		// First block is a section with the question text.
		section, ok := blocks[0].(*slacklib.SectionBlock)
		require.True(t, ok, "first block should be a SectionBlock")
		assert.Equal(t, slacklib.MBTSection, section.Type)
		require.NotNil(t, section.Text)
		assert.Contains(t, section.Text.Text, "Which database should we use?")

		// Second block is an action block with buttons.
		actionBlock, ok := blocks[1].(*slacklib.ActionBlock)
		require.True(t, ok, "second block should be an ActionBlock")
		assert.Equal(t, slacklib.MBTAction, actionBlock.Type)
		require.NotNil(t, actionBlock.Elements)
		assert.Len(t, actionBlock.Elements.ElementSet, 3, "should have 3 buttons")

		// Verify first button.
		btn, ok := actionBlock.Elements.ElementSet[0].(*slacklib.ButtonBlockElement)
		require.True(t, ok, "element should be a ButtonBlockElement")
		assert.Equal(t, "pg", btn.Value)
		require.NotNil(t, btn.Text)
		assert.Equal(t, "PostgreSQL", btn.Text.Text)
	})

	t.Run("with no options returns text-only blocks", func(t *testing.T) {
		t.Parallel()

		blocks := airaslack.BuildQuestionBlocks("What color do you want?", nil)

		require.Len(t, blocks, 1, "should have only text section block")

		section, ok := blocks[0].(*slacklib.SectionBlock)
		require.True(t, ok, "block should be a SectionBlock")
		require.NotNil(t, section.Text)
		assert.Contains(t, section.Text.Text, "What color do you want?")
	})

	t.Run("with empty options returns text-only blocks", func(t *testing.T) {
		t.Parallel()

		blocks := airaslack.BuildQuestionBlocks("Free-form question?", []messenger.QuestionOption{})

		require.Len(t, blocks, 1)
	})
}

func TestBuildTaskNotificationBlocks(t *testing.T) {
	t.Parallel()

	t.Run("returns formatted notification blocks", func(t *testing.T) {
		t.Parallel()

		blocks := airaslack.BuildTaskNotificationBlocks("Implement login page", "in_progress")

		require.GreaterOrEqual(t, len(blocks), 1, "should have at least one block")

		// First block should be a section with task info.
		section, ok := blocks[0].(*slacklib.SectionBlock)
		require.True(t, ok, "first block should be a SectionBlock")
		require.NotNil(t, section.Text)
		assert.Contains(t, section.Text.Text, "Implement login page")
		assert.Contains(t, section.Text.Text, "in_progress")
	})
}
