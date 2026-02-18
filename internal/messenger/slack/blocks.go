package slack

import (
	"fmt"

	slacklib "github.com/slack-go/slack"

	"github.com/gosuda/aira/internal/messenger"
)

// BuildQuestionBlocks builds Slack Block Kit blocks for a HITL question.
// If options are provided, an action block with buttons is appended below the text section.
func BuildQuestionBlocks(question string, options []messenger.QuestionOption) []slacklib.Block {
	textBlock := slacklib.NewSectionBlock(
		slacklib.NewTextBlockObject(slacklib.MarkdownType, question, false, false),
		nil,
		nil,
	)

	if len(options) == 0 {
		return []slacklib.Block{textBlock}
	}

	buttons := make([]slacklib.BlockElement, 0, len(options))
	for i, opt := range options {
		actionID := fmt.Sprintf("hitl_answer_%d", i)
		btn := slacklib.NewButtonBlockElement(
			actionID,
			opt.Value,
			slacklib.NewTextBlockObject(slacklib.PlainTextType, opt.Label, false, false),
		)
		buttons = append(buttons, btn)
	}

	actionBlock := slacklib.NewActionBlock("hitl_actions", buttons...)

	return []slacklib.Block{textBlock, actionBlock}
}

// BuildTaskNotificationBlocks builds Slack Block Kit blocks for a task status notification.
func BuildTaskNotificationBlocks(title, status string) []slacklib.Block {
	text := fmt.Sprintf("*Task:* %s\n*Status:* `%s`", title, status)
	section := slacklib.NewSectionBlock(
		slacklib.NewTextBlockObject(slacklib.MarkdownType, text, false, false),
		nil,
		nil,
	)

	return []slacklib.Block{section}
}
