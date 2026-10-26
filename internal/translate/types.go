// Package translate converts between OpenAI Chat Completions, OpenAI Responses
// and Anthropic Messages API payloads through a small provider-neutral IR.
package translate

import "encoding/json"

const (
	APIChat      = "chat.completions"
	APIResponses = "responses"
	APIMessages  = "messages"
)

type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Message struct {
	Role       string
	Content    any
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type Request struct {
	Model              string
	Messages           []Message
	MaxTokens          int
	Temperature        *float64
	TopP               *float64
	Stop               []string
	Stream             bool
	Tools              []Tool
	ToolChoice         any
	ResponseFormat     any
	Metadata           map[string]string
	StreamIncludeUsage bool
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Response struct {
	ID           string
	Model        string
	Created      int64
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        Usage
}

type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}
