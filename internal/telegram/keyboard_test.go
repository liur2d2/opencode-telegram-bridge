package telegram

import (
	"testing"
)

func TestBuildQuestionKeyboard(t *testing.T) {
	tests := []struct {
		name            string
		questionInfo    QuestionInfo
		requestID       string
		expectedButtons int
		expectedRows    int
	}{
		{
			name: "simple question with 3 options",
			questionInfo: QuestionInfo{
				Question: "Choose an option",
				Options: []QuestionOption{
					{Label: "Option 1"},
					{Label: "Option 2"},
					{Label: "Option 3"},
				},
				Multiple: false,
				Custom:   false,
			},
			requestID:       "req123",
			expectedButtons: 3,
			expectedRows:    3,
		},
		{
			name: "multiple choice question with submit button",
			questionInfo: QuestionInfo{
				Question: "Select multiple",
				Options: []QuestionOption{
					{Label: "A"},
					{Label: "B"},
				},
				Multiple: true,
				Custom:   false,
			},
			requestID:       "req456",
			expectedButtons: 3, // 2 options + submit
			expectedRows:    3,
		},
		{
			name: "question with custom option",
			questionInfo: QuestionInfo{
				Question: "Pick one",
				Options: []QuestionOption{
					{Label: "Preset 1"},
				},
				Multiple: false,
				Custom:   true,
			},
			requestID:       "req789",
			expectedButtons: 2, // 1 option + custom
			expectedRows:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyboard := BuildQuestionKeyboard(tt.questionInfo, tt.requestID)

			if keyboard == nil {
				t.Fatal("BuildQuestionKeyboard returned nil")
			}

			totalButtons := 0
			for _, row := range keyboard.InlineKeyboard {
				totalButtons += len(row)
			}

			if totalButtons != tt.expectedButtons {
				t.Errorf("expected %d buttons, got %d", tt.expectedButtons, totalButtons)
			}

			if len(keyboard.InlineKeyboard) != tt.expectedRows {
				t.Errorf("expected %d rows, got %d", tt.expectedRows, len(keyboard.InlineKeyboard))
			}
		})
	}
}

func TestBuildPermissionKeyboard(t *testing.T) {
	permissionID := "perm123"
	keyboard := BuildPermissionKeyboard(permissionID)

	if keyboard == nil {
		t.Fatal("BuildPermissionKeyboard returned nil")
	}

	// Should have exactly 3 buttons
	totalButtons := 0
	for _, row := range keyboard.InlineKeyboard {
		totalButtons += len(row)
	}

	if totalButtons != 3 {
		t.Errorf("expected 3 buttons, got %d", totalButtons)
	}

	// Verify button text
	expectedLabels := []string{"✅ Allow Once", "✅ Always Allow", "❌ Reject"}
	expectedCallbackSuffixes := []string{":once", ":always", ":reject"}

	buttonIdx := 0
	for _, row := range keyboard.InlineKeyboard {
		for _, btn := range row {
			if buttonIdx >= len(expectedLabels) {
				t.Fatalf("more buttons than expected")
			}

			if btn.Text != expectedLabels[buttonIdx] {
				t.Errorf("button %d: expected text %q, got %q", buttonIdx, expectedLabels[buttonIdx], btn.Text)
			}

			if !hasPrefix(btn.CallbackData, "p:") {
				t.Errorf("button %d: callback should start with 'p:', got %q", buttonIdx, btn.CallbackData)
			}

			if !hasSuffix(btn.CallbackData, expectedCallbackSuffixes[buttonIdx]) {
				t.Errorf("button %d: callback should end with %q, got %q", buttonIdx, expectedCallbackSuffixes[buttonIdx], btn.CallbackData)
			}

			buttonIdx++
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestCallbackDataLength(t *testing.T) {
	tests := []struct {
		name         string
		questionInfo QuestionInfo
		requestID    string
	}{
		{
			name: "normal request ID",
			questionInfo: QuestionInfo{
				Question: "Test",
				Options: []QuestionOption{
					{Label: "Option 1"},
					{Label: "Option 2"},
				},
				Multiple: false,
				Custom:   false,
			},
			requestID: "req_short",
		},
		{
			name: "longer request ID",
			questionInfo: QuestionInfo{
				Question: "Test",
				Options: []QuestionOption{
					{Label: "Opt"},
				},
				Multiple: false,
				Custom:   false,
			},
			requestID: "req_" + "longid1234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyboard := BuildQuestionKeyboard(tt.questionInfo, tt.requestID)

			for rowIdx, row := range keyboard.InlineKeyboard {
				for btnIdx, btn := range row {
					if len(btn.CallbackData) > 64 {
						t.Errorf("row %d, button %d: callback_data length %d exceeds 64 bytes: %q",
							rowIdx, btnIdx, len(btn.CallbackData), btn.CallbackData)
					}
				}
			}
		})
	}

	// Test permission keyboard
	t.Run("permission keyboard", func(t *testing.T) {
		keyboard := BuildPermissionKeyboard("perm_longid1234567890")

		for rowIdx, row := range keyboard.InlineKeyboard {
			for btnIdx, btn := range row {
				if len(btn.CallbackData) > 64 {
					t.Errorf("row %d, button %d: callback_data length %d exceeds 64 bytes: %q",
						rowIdx, btnIdx, len(btn.CallbackData), btn.CallbackData)
				}
			}
		}
	})
}
