package translate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func textContent(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, part := range x {
			if m, ok := part.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// ParseRequest parses an inbound API body into the neutral request IR.
func ParseRequest(apiType string, body []byte) (Request, error) {
	switch apiType {
	case APIChat:
		return parseChatRequest(body)
	case APIResponses:
		return parseResponsesRequest(body)
	case APIMessages:
		return parseMessagesRequest(body)
	default:
		return Request{}, fmt.Errorf("unsupported API type %q", apiType)
	}
}

func parseChatRequest(body []byte) (Request, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Request{}, fmt.Errorf("invalid JSON: %w", err)
	}
	req := Request{Model: stringValue(m["model"]), Stream: boolValue(m["stream"])}
	if n := intValue(m["max_completion_tokens"]); n > 0 {
		req.MaxTokens = n
	} else {
		req.MaxTokens = intValue(m["max_tokens"])
	}
	req.Temperature = floatPtr(m["temperature"])
	req.TopP = floatPtr(m["top_p"])
	req.Stop = stringSlice(m["stop"])
	req.ToolChoice = m["tool_choice"]
	req.ResponseFormat = m["response_format"]
	if md, ok := m["metadata"].(map[string]any); ok {
		req.Metadata = map[string]string{}
		for k, v := range md {
			req.Metadata[k] = stringValue(v)
		}
	}
	if tools, ok := m["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			fn, _ := tm["function"].(map[string]any)
			req.Tools = append(req.Tools, Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        stringValue(fn["name"]),
					Description: stringValue(fn["description"]),
					Parameters:  rawJSON(fn["parameters"]),
				},
			})
		}
	}
	msgs, _ := m["messages"].([]any)
	for _, raw := range msgs {
		cm, _ := raw.(map[string]any)
		msg := Message{
			Role:       stringValue(cm["role"]),
			Content:    cm["content"],
			ToolCallID: stringValue(cm["tool_call_id"]),
			Name:       stringValue(cm["name"]),
		}
		if calls, ok := cm["tool_calls"].([]any); ok {
			for _, c := range calls {
				call, _ := c.(map[string]any)
				fn, _ := call["function"].(map[string]any)
				msg.ToolCalls = append(msg.ToolCalls, ToolCall{
					ID:        stringValue(call["id"]),
					Type:      "function",
					Name:      stringValue(fn["name"]),
					Arguments: stringValue(fn["arguments"]),
				})
			}
		}
		req.Messages = append(req.Messages, msg)
	}
	return req, nil
}

func parseResponsesRequest(body []byte) (Request, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Request{}, fmt.Errorf("invalid JSON: %w", err)
	}
	req := Request{
		Model:              stringValue(m["model"]),
		Stream:             boolValue(m["stream"]),
		MaxTokens:          intValue(m["max_output_tokens"]),
		Temperature:        floatPtr(m["temperature"]),
		TopP:               floatPtr(m["top_p"]),
		ToolChoice:         m["tool_choice"],
		StreamIncludeUsage: true,
	}
	if instructions := stringValue(m["instructions"]); instructions != "" {
		req.Messages = append(req.Messages, Message{Role: "system", Content: instructions})
	}
	if md, ok := m["metadata"].(map[string]any); ok {
		req.Metadata = map[string]string{}
		for k, v := range md {
			req.Metadata[k] = stringValue(v)
		}
	}
	if tools, ok := m["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			if stringValue(tm["type"]) != "function" {
				continue
			}
			req.Tools = append(req.Tools, Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        stringValue(tm["name"]),
					Description: stringValue(tm["description"]),
					Parameters:  rawJSON(tm["parameters"]),
				},
			})
		}
	}
	switch input := m["input"].(type) {
	case string:
		if input != "" {
			req.Messages = append(req.Messages, Message{Role: "user", Content: input})
		}
	case []any:
		for _, raw := range input {
			item, _ := raw.(map[string]any)
			typ := stringValue(item["type"])
			switch typ {
			case "function_call_output":
				req.Messages = append(req.Messages, Message{
					Role:       "tool",
					ToolCallID: stringValue(item["call_id"]),
					Content:    item["output"],
				})
			case "function_call":
				req.Messages = append(req.Messages, Message{
					Role: "assistant",
					ToolCalls: []ToolCall{{
						ID:        stringValue(item["call_id"]),
						Type:      "function",
						Name:      stringValue(item["name"]),
						Arguments: stringValue(item["arguments"]),
					}},
				})
			default:
				content := item["content"]
				if typ == "message" {
					req.Messages = append(req.Messages, Message{
						Role:    stringValue(item["role"]),
						Content: responsesInputContent(content),
					})
				} else if role := stringValue(item["role"]); role != "" {
					req.Messages = append(req.Messages, Message{Role: role, Content: responsesInputContent(content)})
				}
			}
		}
	}
	return req, nil
}

func responsesInputContent(content any) any {
	parts, ok := content.([]any)
	if !ok {
		return content
	}
	var out []map[string]any
	for _, raw := range parts {
		p, _ := raw.(map[string]any)
		switch stringValue(p["type"]) {
		case "input_text", "output_text", "text":
			out = append(out, map[string]any{"type": "text", "text": stringValue(p["text"])})
		default:
			out = append(out, p)
		}
	}
	return out
}

func parseMessagesRequest(body []byte) (Request, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Request{}, fmt.Errorf("invalid JSON: %w", err)
	}
	req := Request{
		Model:              stringValue(m["model"]),
		Stream:             boolValue(m["stream"]),
		MaxTokens:          intValue(m["max_tokens"]),
		Temperature:        floatPtr(m["temperature"]),
		TopP:               floatPtr(m["top_p"]),
		Stop:               stringSlice(m["stop_sequences"]),
		ToolChoice:         m["tool_choice"],
		StreamIncludeUsage: true,
	}
	if system := m["system"]; system != nil {
		req.Messages = append(req.Messages, Message{Role: "system", Content: textContent(system)})
	}
	if md, ok := m["metadata"].(map[string]any); ok {
		req.Metadata = map[string]string{}
		for k, v := range md {
			req.Metadata[k] = stringValue(v)
		}
	}
	if tools, ok := m["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			req.Tools = append(req.Tools, Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        stringValue(tm["name"]),
					Description: stringValue(tm["description"]),
					Parameters:  rawJSON(tm["input_schema"]),
				},
			})
		}
	}
	msgs, _ := m["messages"].([]any)
	for _, raw := range msgs {
		mm, _ := raw.(map[string]any)
		msg := Message{Role: stringValue(mm["role"])}
		blocks, isBlocks := mm["content"].([]any)
		if !isBlocks {
			msg.Content = mm["content"]
		} else {
			var text []string
			for _, braw := range blocks {
				b, _ := braw.(map[string]any)
				switch stringValue(b["type"]) {
				case "text":
					text = append(text, stringValue(b["text"]))
				case "tool_result":
					msg.Role = "tool"
					msg.ToolCallID = stringValue(b["tool_use_id"])
					msg.Content = textContent(b["content"])
				case "tool_use":
					msg.ToolCalls = append(msg.ToolCalls, ToolCall{
						ID:        stringValue(b["id"]),
						Type:      "function",
						Name:      stringValue(b["name"]),
						Arguments: jsonString(b["input"]),
					})
				}
			}
			if len(text) > 0 {
				msg.Content = strings.Join(text, "\n")
			}
		}
		req.Messages = append(req.Messages, msg)
	}
	return req, nil
}

// BuildUpstreamRequest converts the IR to the configured upstream API format.
func BuildUpstreamRequest(apiType string, req Request, upstreamModel string) ([]byte, error) {
	var payload map[string]any
	switch apiType {
	case APIChat:
		payload = buildChatRequest(req, upstreamModel)
	case APIResponses:
		payload = buildResponsesRequest(req, upstreamModel)
	case APIMessages:
		payload = buildMessagesRequest(req, upstreamModel)
	default:
		return nil, fmt.Errorf("unsupported upstream API type %q", apiType)
	}
	return json.Marshal(payload)
}

func buildChatRequest(req Request, upstreamModel string) map[string]any {
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		cm := map[string]any{"role": m.Role}
		if m.Content != nil {
			cm["content"] = m.Content
		}
		if m.Role == "tool" {
			cm["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(m.ToolCalls))
			for _, c := range m.ToolCalls {
				calls = append(calls, map[string]any{
					"id": c.ID, "type": "function",
					"function": map[string]any{"name": c.Name, "arguments": c.Arguments},
				})
			}
			cm["tool_calls"] = calls
		}
		msgs = append(msgs, cm)
	}
	p := map[string]any{"model": upstreamModel, "messages": msgs, "stream": req.Stream}
	if req.MaxTokens > 0 {
		p["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		p["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		p["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		p["stop"] = req.Stop
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			fn := map[string]any{"name": t.Function.Name}
			if t.Function.Description != "" {
				fn["description"] = t.Function.Description
			}
			if len(t.Function.Parameters) > 0 {
				fn["parameters"] = json.RawMessage(t.Function.Parameters)
			}
			tools = append(tools, map[string]any{"type": "function", "function": fn})
		}
		p["tools"] = tools
	}
	if req.ToolChoice != nil {
		p["tool_choice"] = req.ToolChoice
	}
	if req.ResponseFormat != nil {
		p["response_format"] = req.ResponseFormat
	}
	if req.Stream {
		p["stream_options"] = map[string]any{"include_usage": true}
	}
	return p
}

func buildResponsesRequest(req Request, upstreamModel string) map[string]any {
	p := map[string]any{"model": upstreamModel, "stream": req.Stream, "store": false}
	var instructions []string
	input := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			instructions = append(instructions, textContent(m.Content))
		case "tool":
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": m.ToolCallID,
				"output": textContent(m.Content),
			})
		case "assistant":
			if txt := textContent(m.Content); txt != "" {
				input = append(input, map[string]any{
					"type": "message", "role": "assistant",
					"content": []map[string]any{{"type": "output_text", "text": txt}},
				})
			}
			for _, c := range m.ToolCalls {
				input = append(input, map[string]any{
					"type": "function_call", "call_id": c.ID, "name": c.Name, "arguments": c.Arguments,
				})
			}
		default:
			input = append(input, map[string]any{
				"type": "message", "role": m.Role,
				"content": []map[string]any{{"type": "input_text", "text": textContent(m.Content)}},
			})
		}
	}
	if len(instructions) > 0 {
		p["instructions"] = strings.Join(instructions, "\n")
	}
	p["input"] = input
	if req.MaxTokens > 0 {
		p["max_output_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		p["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		p["top_p"] = *req.TopP
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tm := map[string]any{"type": "function", "name": t.Function.Name}
			if t.Function.Description != "" {
				tm["description"] = t.Function.Description
			}
			if len(t.Function.Parameters) > 0 {
				tm["parameters"] = json.RawMessage(t.Function.Parameters)
			}
			tools = append(tools, tm)
		}
		p["tools"] = tools
	}
	if req.ToolChoice != nil {
		p["tool_choice"] = normalizeResponsesToolChoice(req.ToolChoice)
	}
	return p
}

func normalizeResponsesToolChoice(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if stringValue(m["type"]) == "function" {
		if name := stringValue(m["name"]); name != "" {
			return map[string]any{"type": "function", "name": name}
		}
		if fn, ok := m["function"].(map[string]any); ok {
			return map[string]any{"type": "function", "name": stringValue(fn["name"])}
		}
	}
	return v
}

func buildMessagesRequest(req Request, upstreamModel string) map[string]any {
	p := map[string]any{"model": upstreamModel, "stream": req.Stream}
	var system []string
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			system = append(system, textContent(m.Content))
		case "tool":
			msgs = append(msgs, map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type": "tool_result", "tool_use_id": m.ToolCallID,
					"content": textContent(m.Content),
				}},
			})
		case "assistant":
			blocks := []map[string]any{}
			if txt := textContent(m.Content); txt != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": txt})
			}
			for _, c := range m.ToolCalls {
				var input any = map[string]any{}
				if c.Arguments != "" {
					_ = json.Unmarshal([]byte(c.Arguments), &input)
				}
				blocks = append(blocks, map[string]any{
					"type": "tool_use", "id": c.ID, "name": c.Name, "input": input,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]any{"type": "text", "text": ""})
			}
			msgs = append(msgs, map[string]any{"role": "assistant", "content": blocks})
		default:
			msgs = append(msgs, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": textContent(m.Content)}},
			})
		}
	}
	if len(system) > 0 {
		p["system"] = strings.Join(system, "\n")
	}
	p["messages"] = msgs
	if req.MaxTokens > 0 {
		p["max_tokens"] = req.MaxTokens
	} else {
		p["max_tokens"] = 4096
	}
	if req.Temperature != nil {
		p["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		p["top_p"] = *req.TopP
	}
	if len(req.Stop) > 0 {
		p["stop_sequences"] = req.Stop
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tm := map[string]any{"name": t.Function.Name}
			if t.Function.Description != "" {
				tm["description"] = t.Function.Description
			}
			if len(t.Function.Parameters) > 0 {
				tm["input_schema"] = json.RawMessage(t.Function.Parameters)
			} else {
				tm["input_schema"] = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, tm)
		}
		p["tools"] = tools
	}
	if req.ToolChoice != nil {
		p["tool_choice"] = normalizeMessagesToolChoice(req.ToolChoice)
	}
	return p
}

func normalizeMessagesToolChoice(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if fn, ok := m["function"].(map[string]any); ok {
		return map[string]any{"type": "tool", "name": stringValue(fn["name"])}
	}
	return v
}

func rawJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

func intValue(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func floatPtr(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func stringSlice(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

var ErrEmptyModel = errors.New("model is required")
