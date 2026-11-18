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
	case APIInteractions:
		return parseInteractionsRequest(body)
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

// parseInteractionsRequest parses a Google Gemini Interactions API request into
// the neutral IR. Sampling controls (temperature/top_p) do not exist in that
// API, so they are simply absent from the IR.
func parseInteractionsRequest(body []byte) (Request, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return Request{}, fmt.Errorf("invalid JSON: %w", err)
	}
	req := Request{
		Model:                 stringValue(m["model"]),
		Stream:                boolValue(m["stream"]),
		Store:                 boolValue(m["store"]),
		ResponseFormat:        interactionsResponseFormatToIR(m["response_format"]),
		PreviousInteractionID: stringValue(m["previous_interaction_id"]),
	}
	req.ToolChoice = interactionsToolChoiceToIR(m["tool_choice"])
	if system := m["system_instruction"]; system != nil {
		if txt := textContent(system); txt != "" {
			req.Messages = append(req.Messages, Message{Role: "system", Content: txt})
		}
	}
	if gc, ok := m["generation_config"].(map[string]any); ok {
		req.MaxTokens = intValue(gc["max_output_tokens"])
		req.Stop = stringSlice(gc["stop_sequences"])
		if req.ToolChoice == nil {
			req.ToolChoice = interactionsToolChoiceToIR(gc["tool_choice"])
		}
	}
	if tools, ok := m["tools"].([]any); ok {
		for _, raw := range tools {
			tm, _ := raw.(map[string]any)
			// Interactions tools are flat; only client-side function tools map
			// onto the IR. Server-side tools (code_execution, ...) are skipped.
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
	req.Messages = append(req.Messages, parseInteractionsInput(m["input"])...)
	return req, nil
}

// parseInteractionsInput converts the polymorphic `input` field (string, one
// Content object, Content array or Step array) into IR messages.
func parseInteractionsInput(input any) []Message {
	switch v := input.(type) {
	case string:
		if text := strings.TrimSpace(v); text != "" {
			return []Message{{Role: "user", Content: text}}
		}
	case map[string]any:
		return parseInteractionsItems([]any{v})
	case []any:
		return parseInteractionsItems(v)
	}
	return nil
}

// parseInteractionsItems walks input entries, grouping consecutive plain text
// content blocks into one user message and mapping steps to their IR message
// shape. Unknown step and content types are skipped, never fatal.
func parseInteractionsItems(items []any) []Message {
	var out []Message
	var pendingText strings.Builder
	flushText := func() {
		if pendingText.Len() == 0 {
			return
		}
		out = append(out, Message{Role: "user", Content: pendingText.String()})
		pendingText.Reset()
	}
	for _, raw := range items {
		if s, ok := raw.(string); ok {
			pendingText.WriteString(s)
			continue
		}
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch stringValue(item["type"]) {
		case "user_input":
			flushText()
			if txt := interactionsContentText(item["content"]); txt != "" {
				out = append(out, Message{Role: "user", Content: txt})
			}
		case "model_output":
			flushText()
			if txt := interactionsContentText(item["content"]); txt != "" {
				out = append(out, Message{Role: "assistant", Content: txt})
			}
		case "function_call":
			flushText()
			call := ToolCall{
				ID:        stringValue(item["id"]),
				Type:      "function",
				Name:      stringValue(item["name"]),
				Arguments: interactionsArgumentsString(item["arguments"]),
			}
			// Consecutive function calls belong to one assistant turn.
			if n := len(out); n > 0 && out[n-1].Role == "assistant" && out[n-1].Content == nil {
				out[n-1].ToolCalls = append(out[n-1].ToolCalls, call)
			} else {
				out = append(out, Message{Role: "assistant", ToolCalls: []ToolCall{call}})
			}
		case "function_result":
			flushText()
			out = append(out, Message{
				Role:       "tool",
				ToolCallID: stringValue(item["call_id"]),
				Name:       stringValue(item["name"]),
				Content:    interactionsResultText(item["result"]),
			})
		case "text":
			pendingText.WriteString(stringValue(item["text"]))
		default:
			// thought, thought_signature, server-side tool steps and unknown
			// content types carry nothing the IR can express.
		}
	}
	flushText()
	return out
}

// interactionsContentText extracts the text of a Content array or one Content
// block.
func interactionsContentText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if stringValue(x["type"]) == "text" {
			return stringValue(x["text"])
		}
	case []any:
		var b strings.Builder
		for _, raw := range x {
			block, _ := raw.(map[string]any)
			if stringValue(block["type"]) == "text" {
				b.WriteString(stringValue(block["text"]))
			}
		}
		return b.String()
	}
	return ""
}

// interactionsResultText flattens a function_result `result` value, which may
// be a string, a JSON object or a Content array.
func interactionsResultText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, raw := range x {
			block, _ := raw.(map[string]any)
			if stringValue(block["type"]) == "text" {
				b.WriteString(stringValue(block["text"]))
			}
		}
		return b.String()
	}
	return jsonString(v)
}

// interactionsArgumentsString renders function call arguments, which are a JSON
// object in the Interactions API and a JSON string in the IR.
func interactionsArgumentsString(v any) string {
	if v == nil {
		return "{}"
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return "{}"
		}
		return s
	}
	return jsonString(v)
}

// interactionsToolChoiceToIR normalizes an Interactions tool choice into the
// chat-style IR representation used by the other parsers.
func interactionsToolChoiceToIR(v any) any {
	switch x := v.(type) {
	case string:
		switch x {
		case "auto":
			return "auto"
		case "none":
			return "none"
		case "any", "validated":
			return "required"
		}
	case map[string]any:
		allowed, _ := x["allowed_tools"].(map[string]any)
		if allowed == nil {
			return nil
		}
		names := stringSlice(allowed["tools"])
		mode := stringValue(allowed["mode"])
		if len(names) == 1 && (mode == "any" || mode == "validated") {
			return map[string]any{
				"type":     "function",
				"function": map[string]any{"name": names[0]},
			}
		}
		switch mode {
		case "none":
			return "none"
		case "auto":
			return "auto"
		case "any", "validated":
			return "required"
		}
	}
	return nil
}

// interactionsResponseFormatToIR normalizes an Interactions response format into
// the chat-style JSON-schema form stored in the IR.
func interactionsResponseFormatToIR(v any) any {
	fm, ok := v.(map[string]any)
	if !ok || stringValue(fm["type"]) != "text" {
		return nil
	}
	switch stringValue(fm["mime_type"]) {
	case "application/json":
		schema := fm["schema"]
		if schema == nil {
			return map[string]any{"type": "json_object"}
		}
		return map[string]any{
			"type":        "json_schema",
			"json_schema": map[string]any{"name": "response", "schema": schema},
		}
	default:
		return nil
	}
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
	case APIInteractions:
		payload = buildInteractionsRequest(req, upstreamModel)
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

// buildInteractionsRequest renders the IR as a Google Gemini Interactions API
// request body. The Interactions API has no temperature/top_p, so those IR
// fields are intentionally dropped.
func buildInteractionsRequest(req Request, upstreamModel string) map[string]any {
	p := map[string]any{
		"model":  upstreamModel,
		"stream": req.Stream,
		// The gateway is stateless and replays history itself, so storage is
		// off unless the caller explicitly asked for it.
		"store": req.Store,
	}
	if req.PreviousInteractionID != "" {
		p["previous_interaction_id"] = req.PreviousInteractionID
	}

	var system []string
	steps := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if txt := textContent(m.Content); txt != "" {
				system = append(system, txt)
			}
		case "tool":
			result := textContent(m.Content)
			step := map[string]any{
				"type":    "function_result",
				"call_id": m.ToolCallID,
				"result":  []map[string]any{{"type": "text", "text": result}},
			}
			if m.Name != "" {
				step["name"] = m.Name
			}
			steps = append(steps, step)
		case "assistant":
			if txt := textContent(m.Content); txt != "" {
				steps = append(steps, map[string]any{
					"type":    "model_output",
					"content": []map[string]any{{"type": "text", "text": txt}},
				})
			}
			for _, c := range m.ToolCalls {
				id := c.ID
				if id == "" {
					id = "call_" + randomSuffix()
				}
				steps = append(steps, map[string]any{
					"type": "function_call", "id": id, "name": c.Name,
					"arguments": interactionsArgumentsObject(c.Arguments),
				})
			}
		default: // user
			if txt := textContent(m.Content); txt != "" {
				steps = append(steps, map[string]any{
					"type":    "user_input",
					"content": []map[string]any{{"type": "text", "text": txt}},
				})
			}
		}
	}
	if len(system) > 0 {
		p["system_instruction"] = strings.Join(system, "\n")
	}
	p["input"] = steps

	generation := map[string]any{}
	if req.MaxTokens > 0 {
		generation["max_output_tokens"] = req.MaxTokens
	}
	if len(req.Stop) > 0 {
		generation["stop_sequences"] = req.Stop
	}
	if choice := interactionsToolChoiceFromIR(req.ToolChoice); choice != nil {
		generation["tool_choice"] = choice
	}
	if len(generation) > 0 {
		p["generation_config"] = generation
	}
	if format := interactionsResponseFormatFromIR(req.ResponseFormat); format != nil {
		p["response_format"] = format
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
	return p
}

// interactionsArgumentsObject converts the IR's JSON-string arguments into the
// JSON object the Interactions API expects.
func interactionsArgumentsObject(args string) map[string]any {
	out := map[string]any{}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &out); err != nil {
			out = map[string]any{}
		}
	}
	return out
}

// interactionsToolChoiceFromIR converts the chat-style IR tool choice into the
// Interactions API shape.
func interactionsToolChoiceFromIR(v any) any {
	switch x := v.(type) {
	case string:
		switch x {
		case "auto":
			return "auto"
		case "none":
			return "none"
		case "required":
			return "any"
		}
	case map[string]any:
		name := stringValue(x["name"])
		if fn, ok := x["function"].(map[string]any); ok {
			name = stringValue(fn["name"])
		}
		if name != "" {
			return map[string]any{
				"allowed_tools": map[string]any{"mode": "any", "tools": []string{name}},
			}
		}
		switch stringValue(x["type"]) {
		case "auto":
			return "auto"
		case "none":
			return "none"
		case "required":
			return "any"
		}
	}
	return nil
}

// interactionsResponseFormatFromIR converts the IR response format into the
// Interactions API ResponseFormat shape.
func interactionsResponseFormatFromIR(v any) any {
	fm, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	switch stringValue(fm["type"]) {
	case "text":
		// Already an Interactions response format.
		return fm
	case "json_object":
		return map[string]any{"type": "text", "mime_type": "application/json"}
	case "json_schema":
		var schema any
		if js, ok := fm["json_schema"].(map[string]any); ok {
			schema = js["schema"]
		}
		if schema == nil {
			schema = fm["schema"]
		}
		format := map[string]any{"type": "text", "mime_type": "application/json"}
		if schema != nil {
			format["schema"] = schema
		}
		return format
	}
	return nil
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
