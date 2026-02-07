package bridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/user/opencode-telegram/internal/opencode"
	"github.com/user/opencode-telegram/internal/telegram"
)

func (b *Bridge) handleQuestionAsked(event opencode.EventQuestionAsked) error {
	ctx := context.Background()
	props := event.Properties

	var msgBuilder strings.Builder
	msgBuilder.WriteString("🤔 OpenCode has questions:\n\n")

	for i, q := range props.Questions {
		msgBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, q.Question))
		if len(q.Options) > 0 {
			for _, opt := range q.Options {
				msgBuilder.WriteString(fmt.Sprintf("  • %s\n", opt.Label))
			}
		}
		custom := q.Custom != nil && *q.Custom
		if custom {
			msgBuilder.WriteString("  • ✏️ Custom answer allowed\n")
		}
		msgBuilder.WriteString("\n")
	}

	msgBuilder.WriteString("Please answer the first question:")

	firstQ := props.Questions[0]
	shortKey := b.registry.Register(props.ID, "q", fmt.Sprintf("%d", 0))

	multiple := firstQ.Multiple != nil && *firstQ.Multiple
	custom := firstQ.Custom != nil && *firstQ.Custom
	tgQuestion := telegram.QuestionInfo{
		Question: firstQ.Question,
		Header:   firstQ.Header,
		Options:  make([]telegram.QuestionOption, len(firstQ.Options)),
		Multiple: multiple,
		Custom:   custom,
	}
	for i, opt := range firstQ.Options {
		tgQuestion.Options[i] = telegram.QuestionOption{
			Label:       opt.Label,
			Description: opt.Description,
		}
	}

	keyboard := telegram.BuildQuestionKeyboard(tgQuestion, shortKey)

	messageID, err := b.tgBot.SendMessageWithKeyboard(ctx, msgBuilder.String(), keyboard)
	if err != nil {
		return fmt.Errorf("failed to send question: %w", err)
	}

	state := &QuestionState{
		RequestID:       props.ID,
		SessionID:       props.SessionID,
		MessageID:       messageID,
		QuestionIndex:   0,
		QuestionInfo:    firstQ,
		SelectedOptions: make(map[int]bool),
		WaitingCustom:   false,
	}
	b.questions.Store(shortKey, state)

	return nil
}

func (b *Bridge) HandleQuestionCallback(ctx context.Context, shortKey, action string) error {
	val, ok := b.questions.Load(shortKey)
	if !ok {
		return fmt.Errorf("question state not found")
	}
	state := val.(*QuestionState)

	if action == "custom" {
		state.WaitingCustom = true
		b.questions.Store(shortKey, state)
		return b.tgBot.EditMessage(ctx, state.MessageID,
			fmt.Sprintf("%s\n\n✏️ Please type your custom answer:", state.QuestionInfo.Question))
	}

	if action == "submit" {
		return b.submitQuestionAnswer(ctx, shortKey, state)
	}

	optionIdx, err := strconv.Atoi(action)
	if err != nil {
		return fmt.Errorf("invalid option index: %s", action)
	}

	if optionIdx < 0 || optionIdx >= len(state.QuestionInfo.Options) {
		return fmt.Errorf("option index out of range: %d", optionIdx)
	}

	multiple := state.QuestionInfo.Multiple != nil && *state.QuestionInfo.Multiple
	if !multiple {
		state.SelectedOptions = map[int]bool{optionIdx: true}
		b.questions.Store(shortKey, state)
		return b.submitQuestionAnswer(ctx, shortKey, state)
	}

	state.SelectedOptions[optionIdx] = !state.SelectedOptions[optionIdx]
	b.questions.Store(shortKey, state)

	return nil
}

func (b *Bridge) HandleQuestionCustomInput(ctx context.Context, text string) bool {
	var foundShortKey string
	var foundState *QuestionState

	b.questions.Range(func(key, value interface{}) bool {
		state := value.(*QuestionState)
		if state.WaitingCustom {
			foundShortKey = key.(string)
			foundState = state
			return false
		}
		return true
	})

	if foundState == nil {
		return false
	}

	foundState.SelectedOptions = map[int]bool{-1: true}
	b.questions.Store(foundShortKey, foundState)

	answers := []opencode.QuestionAnswer{{text}}

	if err := b.ocClient.ReplyQuestion(foundState.RequestID, answers); err != nil {
		b.tgBot.SendMessage(ctx, fmt.Sprintf("❌ Failed to submit answer: %v", err))
		return true
	}

	b.tgBot.EditMessage(ctx, foundState.MessageID,
		fmt.Sprintf("%s\n\n✅ Answer submitted: %s", foundState.QuestionInfo.Question, text))

	b.questions.Delete(foundShortKey)

	return true
}

func (b *Bridge) submitQuestionAnswer(ctx context.Context, shortKey string, state *QuestionState) error {
	var values []string
	for idx, selected := range state.SelectedOptions {
		if selected && idx >= 0 && idx < len(state.QuestionInfo.Options) {
			values = append(values, state.QuestionInfo.Options[idx].Label)
		}
	}

	if len(values) == 0 {
		return fmt.Errorf("no options selected")
	}

	answers := []opencode.QuestionAnswer{values}

	if err := b.ocClient.ReplyQuestion(state.RequestID, answers); err != nil {
		return fmt.Errorf("failed to submit answer: %w", err)
	}

	answerText := strings.Join(values, ", ")
	b.tgBot.EditMessage(ctx, state.MessageID,
		fmt.Sprintf("%s\n\n✅ Answer submitted: %s", state.QuestionInfo.Question, answerText))

	b.questions.Delete(shortKey)

	return nil
}
