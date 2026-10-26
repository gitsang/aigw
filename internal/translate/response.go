package translate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParseResponse parses a non-streaming upstream response into the neutral IR.
func ParseResponse(apiType string, body []byte) (Response, error) {
	switch apiType {
	case APIChat:
		return parseChatResponse(body)
	case APIResponses:
		return parseResponsesResponse(body)
	case APIMessages:
		return parseMessagesResponse(body)
	default:
		return Response{}, fmt.Errorf("unsupported API type %q", apiType)
	}
}

func parseChatResponse(body []byte) (Response, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Response{}, fmt.Errorf("invalid JSON response: %w", err)
	}
	out := Response{
		ID:      stringValue(m["id"]),
		Model:   stringValue(m["model"]),
		Created: int64(intValue(m["created"])),
	}
	if out.Created == 0 {
		out.Created = time.Now().Unix()
	}
	if usage, ok := m["usage"].(map[string]any); ok {
		out.Usage = Usage{
			PromptTokens:     intValue(usage["prompt_tokens"]),
			CompletionTokens: intValue(usage["completion_tokens"]),
			TotalTokens:      intValue(usage["total_tokens"]),
		}
	}
	choices, _ := m["choices"].([]any)
	if len(choices) == 0 {
		return out, nil
	}
	choice, _ := choices[0].(map[string]any)
	out.FinishReason = stringValue(choice["finish_reason"])
	msg, _ := choice["message"].(map[string]any)
	out.Content = textContent(msg["content"])
	if calls, ok := msg["tool_calls"].([]any); ok {
		for _, raw := range calls {
			c, _ := raw.(map[string]any)
			fn, _ := c["function"].(map[string]any)
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:        stringValue(c["id"]),
				Type:      "function",
				Name:      stringValue(fn["name"]),
				Arguments: stringValue(fn["arguments"]),
			})
		}
	}
	return out, nil
}

func parseResponsesResponse(body []byte) (Response, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Response{}, fmt.Errorf("invalid JSON response: %w", err)
	}
	out := Response{
		ID:      stringValue(m["id"]),
		Model:   stringValue(m["model"]),
		Created: int64(intValue(m["created_at"])),
	}
	if out.Created == 0 {
		out.Created = time.Now().Unix()
	}
	if usage, ok := m["usage"].(map[string]any); ok {
		out.Usage = Usage{
			PromptTokens:     intValue(usage["input_tokens"]),
			CompletionTokens: intValue(usage["output_tokens"]),
			TotalTokens:      intValue(usage["total_tokens"]),
		}
	}
	if status := stringValue(m["status"]); status == "incomplete" {
		out.FinishReason = "length"
	}
	if output, ok := m["output"].([]any); ok {
		for _, raw := range output {
			item, _ := raw.(map[string]any)
			switch stringValue(item["type"]) {
			case "message":
				content, _ := item["content"].([]any)
				for _, craw := range content {
					c, _ := craw.(map[string]any)
					if stringValue(c["type"]) == "output_text" {
						out.Content += stringValue(c["text"])
					}
				}
			case "function_call":
				out.ToolCalls = append(out.ToolCalls, ToolCall{
					ID:        stringValue(item["call_id"]),
					Type:      "function",
					Name:      stringValue(item["name"]),
					Arguments: stringValue(item["arguments"]),
				})
			}
		}
	}
	if len(out.ToolCalls) > 0 {
		out.FinishReason = "tool_calls"
	} else if out.FinishReason == "" {
		out.FinishReason = "stop"
	}
	return out, nil
}

func parseMessagesResponse(body []byte) (Response, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Response{}, fmt.Errorf("invalid JSON response: %w", err)
	}
	out := Response{
		ID:           stringValue(m["id"]),
		Model:        stringValue(m["model"]),
		Created:      time.Now().Unix(),
		FinishReason: messagesStopReason(stringValue(m["stop_reason"])),
	}
	if usage, ok := m["usage"].(map[string]any); ok {
		out.Usage = Usage{
			PromptTokens:     intValue(usage["input_tokens"]),
			CompletionTokens: intValue(usage["output_tokens"]),
		}
		out.Usage.TotalTokens = out.Usage.PromptTokens + out.Usage.CompletionTokens
	}
	if content, ok := m["content"].([]any); ok {
		for _, raw := range content {
			block, _ := raw.(map[string]any)
			switch stringValue(block["type"]) {
			case "text":
				out.Content += stringValue(block["text"])
			case "tool_use":
				out.ToolCalls = append(out.ToolCalls, ToolCall{
					ID:        stringValue(block["id"]),
					Type:      "function",
					Name:      stringValue(block["name"]),
					Arguments: jsonString(block["input"]),
				})
			}
		}
	}
	return out, nil
}

func messagesStopReason(stop string) string {
	switch stop {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence", "end_turn", "":
		return "stop"
	default:
		return "stop"
	}
}

func upstreamStopReason(finish string) string {
	switch finish {
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// BuildResponse serializes the neutral IR into a downstream API response.
func BuildResponse(apiType string, r Response, requestedModel string) ([]byte, error) {
	if r.ID == "" {
		r.ID = randomID(apiType)
	}
	if r.Created == 0 {
		r.Created = time.Now().Unix()
	}
	model := requestedModel
	if model == "" {
		model = r.Model
	}
	switch apiType {
	case APIChat:
		return buildChatResponse(r, model)
	case APIResponses:
		return buildResponsesResponse(r, model)
	case APIMessages:
		return buildMessagesResponse(r, model)
	default:
		return nil, fmt.Errorf("unsupported API type %q", apiType)
	}
}

func finishReason(r Response) string {
	if len(r.ToolCalls) > 0 && r.FinishReason == "" {
		return "tool_calls"
	}
	if r.FinishReason == "" {
		return "stop"
	}
	return r.FinishReason
}

func buildChatResponse(r Response, model string) ([]byte, error) {
	message := map[string]any{"role": "assistant", "content": r.Content}
	if len(r.ToolCalls) > 0 {
		if r.Content == "" {
			message["content"] = nil
		}
		calls := make([]map[string]any, 0, len(r.ToolCalls))
		for _, c := range r.ToolCalls {
			calls = append(calls, map[string]any{
				"id": c.ID, "type": "function",
				"function": map[string]any{"name": c.Name, "arguments": c.Arguments},
			})
		}
		message["tool_calls"] = calls
	}
	out := map[string]any{
		"id":      responseID(r.ID, APIChat),
		"object":  "chat.completion",
		"created": r.Created,
		"model":   model,
		"choices": []map[string]any{{
			"index": 0, "message": message, "finish_reason": finishReason(r), "logprobs": nil,
		}},
		"usage": map[string]any{
			"prompt_tokens":     r.Usage.PromptTokens,
			"completion_tokens": r.Usage.CompletionTokens,
			"total_tokens":      totalTokens(r.Usage),
		},
	}
	return json.Marshal(out)
}

func buildResponsesResponse(r Response, model string) ([]byte, error) {
	output := []map[string]any{}
	if r.Content != "" {
		output = append(output, map[string]any{
			"id":     "msg_" + suffix(r.ID),
			"type":   "message",
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{{
				"type": "output_text", "text": r.Content, "annotations": []any{},
			}},
		})
	}
	for _, c := range r.ToolCalls {
		output = append(output, map[string]any{
			"id": "fc_" + suffix(c.ID), "type": "function_call", "status": "completed",
			"call_id": c.ID, "name": c.Name, "arguments": c.Arguments,
		})
	}
	out := map[string]any{
		"id":         responseID(r.ID, APIResponses),
		"object":     "response",
		"created_at": r.Created,
		"status":     "completed",
		"model":      model,
		"output":     output,
		"usage": map[string]any{
			"input_tokens":  r.Usage.PromptTokens,
			"output_tokens": r.Usage.CompletionTokens,
			"total_tokens":  totalTokens(r.Usage),
		},
	}
	return json.Marshal(out)
}

func buildMessagesResponse(r Response, model string) ([]byte, error) {
	content := []map[string]any{}
	if r.Content != "" || len(r.ToolCalls) == 0 {
		content = append(content, map[string]any{"type": "text", "text": r.Content})
	}
	for _, c := range r.ToolCalls {
		var input any = map[string]any{}
		if c.Arguments != "" {
			_ = json.Unmarshal([]byte(c.Arguments), &input)
		}
		content = append(content, map[string]any{
			"type": "tool_use", "id": c.ID, "name": c.Name, "input": input,
		})
	}
	out := map[string]any{
		"id":            responseID(r.ID, APIMessages),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       content,
		"stop_reason":   upstreamStopReason(finishReason(r)),
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  r.Usage.PromptTokens,
			"output_tokens": r.Usage.CompletionTokens,
		},
	}
	return json.Marshal(out)
}

func totalTokens(u Usage) int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.PromptTokens + u.CompletionTokens
}

func responseID(id, apiType string) string {
	prefix := "chatcmpl-"
	switch apiType {
	case APIResponses:
		prefix = "resp_"
	case APIMessages:
		prefix = "msg_"
	}
	if strings.HasPrefix(id, prefix) || strings.HasPrefix(id, "chatcmpl") || strings.HasPrefix(id, "resp_") || strings.HasPrefix(id, "msg_") {
		return id
	}
	return prefix + suffix(id)
}

func suffix(id string) string {
	if id == "" {
		return randomSuffix()
	}
	replacer := strings.NewReplacer("-", "", "_", "")
	if s := replacer.Replace(id); len(s) > 40 {
		return s[len(s)-40:]
	}
	return replacer.Replace(id)
}
