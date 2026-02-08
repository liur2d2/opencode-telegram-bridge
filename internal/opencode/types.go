package opencode

import "time"

// Config holds OpenCode client configuration
type Config struct {
	BaseURL   string
	Directory string
}

// QuestionOption represents a choice in a question
type QuestionOption struct {
	Label       string `json:"label"`       // Display text (1-5 words, concise)
	Description string `json:"description"` // Explanation of choice
}

// QuestionInfo represents a single question
type QuestionInfo struct {
	Question string           `json:"question"`           // Complete question
	Header   string           `json:"header"`             // Very short label (max 30 chars)
	Options  []QuestionOption `json:"options"`            // Available choices
	Multiple *bool            `json:"multiple,omitempty"` // Allow selecting multiple choices
	Custom   *bool            `json:"custom,omitempty"`   // Allow typing a custom answer (default: true)
}

// QuestionRequest represents a question request from the AI
type QuestionRequest struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	Questions []QuestionInfo `json:"questions"`
	Tool      *struct {
		MessageID string `json:"messageID"`
		CallID    string `json:"callID"`
	} `json:"tool,omitempty"`
}

// QuestionAnswer is an array of selected option labels (strings, NOT indices)
type QuestionAnswer []string

// QuestionReplyRequest is the request body for replying to a question
type QuestionReplyRequest struct {
	Answers []QuestionAnswer `json:"answers,omitempty"`
}

// PermissionRequest represents a permission request from the AI
type PermissionRequest struct {
	ID         string                 `json:"id"`
	SessionID  string                 `json:"sessionID"`
	Permission string                 `json:"permission"`
	Patterns   []string               `json:"patterns"`
	Metadata   map[string]interface{} `json:"metadata"`
	Always     []string               `json:"always"`
	Tool       *struct {
		MessageID string `json:"messageID"`
		CallID    string `json:"callID"`
	} `json:"tool,omitempty"`
}

// PermissionResponse is the response type for permission requests
type PermissionResponse string

const (
	PermissionOnce   PermissionResponse = "once"
	PermissionAlways PermissionResponse = "always"
	PermissionReject PermissionResponse = "reject"
)

// PermissionReplyRequest is the request body for replying to a permission
type PermissionReplyRequest struct {
	Reply PermissionResponse `json:"reply"`
}

// Session represents an OpenCode session
type Session struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectID"`
	Directory string  `json:"directory"`
	ParentID  *string `json:"parentID,omitempty"`
	Slug      string  `json:"slug"`
	Title     string  `json:"title"`
	Version   string  `json:"version"`
	Time      struct {
		Created    int64  `json:"created"`
		Updated    int64  `json:"updated"`
		Compacting *int64 `json:"compacting,omitempty"`
		Archived   *int64 `json:"archived,omitempty"`
	} `json:"time"`
}

// SessionCreateRequest is the request body for creating a session
type SessionCreateRequest struct {
	ParentID *string `json:"parentID,omitempty"`
	Title    *string `json:"title,omitempty"`
}

// TextPartInput represents a text part in a message
type TextPartInput struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ImagePartInput represents an image part in a message
type ImagePartInput struct {
	Type     string `json:"type"`     // "image"
	Image    string `json:"image"`    // base64 encoded image data
	MimeType string `json:"mimeType"` // "image/jpeg" or "image/png"
}

// SendPromptRequest is the request body for sending a prompt
type SendPromptRequest struct {
	Agent  *string       `json:"agent,omitempty"`  // Agent type (per-message, not per-session)
	Parts  []interface{} `json:"parts"`            // Message parts (TextPartInput or ImagePartInput)
	System *string       `json:"system,omitempty"` // System message
}

// AssistantMessage represents the response from sending a prompt
type AssistantMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	ParentID  string `json:"parentID"`
	Time      struct {
		Created   int64  `json:"created"`
		Completed *int64 `json:"completed,omitempty"`
	} `json:"time"`
}

// SendPromptResponse is the response from sending a prompt
type SendPromptResponse struct {
	Info  AssistantMessage `json:"info"`
	Parts []interface{}    `json:"parts"`
}

// Event represents an SSE event from OpenCode
type Event struct {
	Type       string
	Properties interface{}
	Timestamp  time.Time
}

// EventQuestionAsked represents a question.asked event
type EventQuestionAsked struct {
	Type       string          `json:"type"`
	Properties QuestionRequest `json:"properties"`
}

// EventQuestionReplied represents a question.replied event
type EventQuestionReplied struct {
	Type       string `json:"type"`
	Properties struct {
		SessionID string           `json:"sessionID"`
		RequestID string           `json:"requestID"`
		Answers   []QuestionAnswer `json:"answers"`
	} `json:"properties"`
}

// EventQuestionRejected represents a question.rejected event
type EventQuestionRejected struct {
	Type       string `json:"type"`
	Properties struct {
		SessionID string `json:"sessionID"`
		RequestID string `json:"requestID"`
	} `json:"properties"`
}

// EventPermissionAsked represents a permission.asked event
type EventPermissionAsked struct {
	Type       string            `json:"type"`
	Properties PermissionRequest `json:"properties"`
}

// EventPermissionReplied represents a permission.replied event
type EventPermissionReplied struct {
	Type       string `json:"type"`
	Properties struct {
		SessionID string             `json:"sessionID"`
		RequestID string             `json:"requestID"`
		Reply     PermissionResponse `json:"reply"`
	} `json:"properties"`
}

// EventMessageUpdated represents a message.updated event
type EventMessageUpdated struct {
	Type       string `json:"type"`
	Properties struct {
		Info *struct {
			ID        string `json:"id"`
			SessionID string `json:"sessionID"`
			Role      string `json:"role"`
			ParentID  string `json:"parentID,omitempty"`
			Time      struct {
				Created   int64  `json:"created"`
				Completed *int64 `json:"completed,omitempty"`
			} `json:"time"`
			ModelID    string `json:"modelID,omitempty"`
			ProviderID string `json:"providerID,omitempty"`
			Mode       string `json:"mode,omitempty"`
			Agent      string `json:"agent,omitempty"`
		} `json:"info,omitempty"`
		Parts []interface{} `json:"parts,omitempty"`
	} `json:"properties"`
}

// EventMessagePartUpdated represents a message.part.updated event
type EventMessagePartUpdated struct {
	Type       string `json:"type"`
	Properties struct {
		Part  interface{} `json:"part"`
		Delta *string     `json:"delta,omitempty"`
	} `json:"properties"`
}

// EventSessionIdle represents a session.idle event
type EventSessionIdle struct {
	Type       string `json:"type"`
	Properties struct {
		SessionID string `json:"sessionID"`
	} `json:"properties"`
}

// EventSessionError represents a session.error event
type EventSessionError struct {
	Type       string `json:"type"`
	Properties struct {
		SessionID *string     `json:"sessionID,omitempty"`
		Error     interface{} `json:"error,omitempty"`
	} `json:"properties"`
}
