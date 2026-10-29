package translate

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gitsang/aigw/pkg/sse"
)

// StreamConverter converts an upstream SSE stream into the downstream API's
// SSE format incrementally. It never buffers the whole stream.
type StreamConverter struct {
	UpstreamAPI   string
	DownstreamAPI string
	Model         string

	id      string
	created int64
	usage   Usage
	finish  string

	text     strings.Builder
	tools    map[int]*ToolCall
	toolKeys map[string]int
	nextTool int

	started      bool
	finalized    bool
	textBlockIdx int
	textOpen     bool
	nextBlock    int
	blockForTool map[int]int

	msgItemID     string
	textPart      bool
	textItemAdded bool
}

func NewStreamConverter(upstreamAPI, downstreamAPI, model string) *StreamConverter {
	return &StreamConverter{
		UpstreamAPI:   upstreamAPI,
		DownstreamAPI: downstreamAPI,
		Model:         model,
		created:       time.Now().Unix(),
		tools:         map[int]*ToolCall{},
		toolKeys:      map[string]int{},
		blockForTool:  map[int]int{},
		textBlockIdx:  -1,
		msgItemID:     "msg_" + randomSuffix(),
	}
}

// Feed consumes one upstream event and returns zero or more downstream events.
func (c *StreamConverter) Feed(ev sse.Event) ([]sse.Event, error) {
	data := strings.TrimSpace(string(ev.Data))
	if data == "" {
		return nil, nil
	}
	if data == "[DONE]" {
		return c.finalize()
	}
	switch c.UpstreamAPI {
	case APIChat:
		return c.feedChat(data)
	case APIResponses:
		return c.feedResponses(ev.Event, data)
	case APIMessages:
		return c.feedMessages(ev.Event, data)
	default:
		return nil, fmt.Errorf("unsupported upstream API type %q", c.UpstreamAPI)
	}
}

func (c *StreamConverter) feedChat(data string) ([]sse.Event, error) {
	var chunk map[string]any
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("decode chat stream chunk: %w", err)
	}
	if id := stringValue(chunk["id"]); id != "" {
		c.id = id
	}
	if created := intValue(chunk["created"]); created > 0 {
		c.created = int64(created)
	}
	if usage, ok := chunk["usage"].(map[string]any); ok {
		c.usage = Usage{
			PromptTokens:     intValue(usage["prompt_tokens"]),
			CompletionTokens: intValue(usage["completion_tokens"]),
			TotalTokens:      intValue(usage["total_tokens"]),
		}
	}
	choices, _ := chunk["choices"].([]any)
	if len(choices) == 0 {
		return nil, nil
	}
	choice, _ := choices[0].(map[string]any)
	if fr := stringValue(choice["finish_reason"]); fr != "" {
		c.finish = fr
	}
	delta, _ := choice["delta"].(map[string]any)
	var out []sse.Event
	if content, ok := delta["content"].(string); ok && content != "" {
		evs, err := c.textDelta(content)
		if err != nil {
			return nil, err
		}
		out = append(out, evs...)
	}
	if calls, ok := delta["tool_calls"].([]any); ok {
		for _, raw := range calls {
			call, _ := raw.(map[string]any)
			idx := intValue(call["index"])
			fn, _ := call["function"].(map[string]any)
			id := stringValue(call["id"])
			name := stringValue(fn["name"])
			args := stringValue(fn["arguments"])
			if _, seen := c.tools[idx]; !seen {
				evs, err := c.toolStart(idx, id, name)
				if err != nil {
					return nil, err
				}
				out = append(out, evs...)
			}
			if args != "" {
				evs, err := c.toolArgs(idx, args)
				if err != nil {
					return nil, err
				}
				out = append(out, evs...)
			}
		}
	}
	return out, nil
}

func (c *StreamConverter) feedResponses(eventName, data string) ([]sse.Event, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return nil, fmt.Errorf("decode responses stream event: %w", err)
	}
	typ := eventName
	if typ == "" {
		typ = stringValue(m["type"])
	}
	switch typ {
	case "response.created", "response.in_progress":
		if resp, ok := m["response"].(map[string]any); ok {
			c.absorbResponseObject(resp)
		}
	case "response.output_text.delta":
		if delta := stringValue(m["delta"]); delta != "" {
			return c.textDelta(delta)
		}
	case "response.output_item.added":
		item, _ := m["item"].(map[string]any)
		if stringValue(item["type"]) == "function_call" {
			key := stringValue(item["id"])
			if key == "" {
				key = stringValue(item["call_id"])
			}
			idx, ok := c.toolKeys[key]
			if !ok {
				idx = c.nextTool
				c.nextTool++
				c.toolKeys[key] = idx
			}
			return c.toolStart(idx, stringValue(item["call_id"]), stringValue(item["name"]))
		}
	case "response.function_call_arguments.delta":
		key := stringValue(m["item_id"])
		idx, ok := c.toolKeys[key]
		if !ok {
			idx = c.nextTool
			c.nextTool++
			c.toolKeys[key] = idx
			evs, err := c.toolStart(idx, "", "")
			if err != nil {
				return nil, err
			}
			out, err := c.toolArgs(idx, stringValue(m["delta"]))
			return append(evs, out...), err
		}
		return c.toolArgs(idx, stringValue(m["delta"]))
	case "response.completed", "response.incomplete", "response.done":
		if resp, ok := m["response"].(map[string]any); ok {
			c.absorbResponseObject(resp)
			c.hydrateFromResponseObject(resp)
		}
		return c.finalize()
	case "response.failed", "error":
		msg := stringValue(m["message"])
		if msg == "" {
			if resp, ok := m["response"].(map[string]any); ok {
				if e, ok := resp["error"].(map[string]any); ok {
					msg = stringValue(e["message"])
				}
			}
		}
		return nil, fmt.Errorf("upstream response stream failed: %s", msg)
	}
	return nil, nil
}

func (c *StreamConverter) feedMessages(eventName, data string) ([]sse.Event, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return nil, fmt.Errorf("decode messages stream event: %w", err)
	}
	typ := eventName
	if typ == "" {
		typ = stringValue(m["type"])
	}
	switch typ {
	case "message_start":
		if msg, ok := m["message"].(map[string]any); ok {
			if id := stringValue(msg["id"]); id != "" {
				c.id = id
			}
			if usage, ok := msg["usage"].(map[string]any); ok {
				c.usage.PromptTokens = intValue(usage["input_tokens"])
				if v := intValue(usage["output_tokens"]); v > 0 {
					c.usage.CompletionTokens = v
				}
				c.usage.CacheReadTokens = intValue(usage["cache_read_input_tokens"])
				c.usage.CacheWriteTokens = intValue(usage["cache_creation_input_tokens"])
			}
		}
	case "content_block_start":
		block, _ := m["content_block"].(map[string]any)
		if stringValue(block["type"]) == "tool_use" {
			idx := c.nextTool
			c.nextTool++
			upstreamIdx := intValue(m["index"])
			c.toolKeys[fmt.Sprintf("block-%d", upstreamIdx)] = idx
			return c.toolStart(idx, stringValue(block["id"]), stringValue(block["name"]))
		}
	case "content_block_delta":
		delta, _ := m["delta"].(map[string]any)
		switch stringValue(delta["type"]) {
		case "text_delta":
			return c.textDelta(stringValue(delta["text"]))
		case "input_json_delta":
			upstreamIdx := intValue(m["index"])
			if idx, ok := c.toolKeys[fmt.Sprintf("block-%d", upstreamIdx)]; ok {
				return c.toolArgs(idx, stringValue(delta["partial_json"]))
			}
		}
	case "message_delta":
		delta, _ := m["delta"].(map[string]any)
		if stop := stringValue(delta["stop_reason"]); stop != "" {
			c.finish = messagesStopReason(stop)
		}
		if usage, ok := m["usage"].(map[string]any); ok {
			if v := intValue(usage["output_tokens"]); v > 0 {
				c.usage.CompletionTokens = v
			}
			if v := intValue(usage["input_tokens"]); v > 0 {
				c.usage.PromptTokens = v
			}
		}
	case "message_stop":
		return c.finalize()
	case "error":
		return nil, fmt.Errorf("upstream messages stream failed: %s", stringValue(m["message"]))
	}
	return nil, nil
}

func (c *StreamConverter) absorbResponseObject(resp map[string]any) {
	if id := stringValue(resp["id"]); id != "" {
		c.id = id
	}
	if created := intValue(resp["created_at"]); created > 0 {
		c.created = int64(created)
	}
	if usage, ok := resp["usage"].(map[string]any); ok {
		c.usage = Usage{
			PromptTokens:     intValue(usage["input_tokens"]),
			CompletionTokens: intValue(usage["output_tokens"]),
			TotalTokens:      intValue(usage["total_tokens"]),
		}
	}
	if status := stringValue(resp["status"]); status == "incomplete" {
		c.finish = "length"
	}
}

func (c *StreamConverter) hydrateFromResponseObject(resp map[string]any) {
	output, _ := resp["output"].([]any)
	for _, raw := range output {
		item, _ := raw.(map[string]any)
		switch stringValue(item["type"]) {
		case "message":
			content, _ := item["content"].([]any)
			for _, craw := range content {
				part, _ := craw.(map[string]any)
				if stringValue(part["type"]) == "output_text" && c.text.Len() == 0 {
					c.text.WriteString(stringValue(part["text"]))
				}
			}
		case "function_call":
			key := stringValue(item["id"])
			if key == "" {
				key = stringValue(item["call_id"])
			}
			if _, ok := c.toolKeys[key]; !ok {
				idx := c.nextTool
				c.nextTool++
				c.toolKeys[key] = idx
				c.tools[idx] = &ToolCall{
					ID: stringValue(item["call_id"]), Type: "function",
					Name: stringValue(item["name"]), Arguments: stringValue(item["arguments"]),
				}
			}
		}
	}
}

func (c *StreamConverter) textDelta(s string) ([]sse.Event, error) {
	c.text.WriteString(s)
	switch c.DownstreamAPI {
	case APIChat:
		return c.chatChunk(map[string]any{"role": "assistant", "content": s}, nil)
	case APIMessages:
		out, err := c.ensureMessagesStart()
		if err != nil {
			return nil, err
		}
		if !c.textOpen {
			if c.toolOpenAny() {
				out = append(out, c.closeLastBlock())
			}
			c.textOpen = true
			c.textBlockIdx = c.nextBlock
			c.nextBlock++
			out = append(out, eventJSON("content_block_start", map[string]any{
				"type": "content_block_start", "index": c.textBlockIdx,
				"content_block": map[string]any{"type": "text", "text": ""},
			}))
		}
		out = append(out, eventJSON("content_block_delta", map[string]any{
			"type": "content_block_delta", "index": c.textBlockIdx,
			"delta": map[string]any{"type": "text_delta", "text": s},
		}))
		return out, nil
	case APIResponses:
		out, err := c.ensureResponsesStart()
		if err != nil {
			return nil, err
		}
		out = append(out, c.ensureResponseTextItem()...)
		if !c.textPart {
			c.textPart = true
			out = append(out, eventJSON("response.content_part.added", map[string]any{
				"type": "response.content_part.added", "item_id": c.msgItemID,
				"output_index": 0, "content_index": 0,
				"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
			}))
		}
		out = append(out, eventJSON("response.output_text.delta", map[string]any{
			"type": "response.output_text.delta", "item_id": c.msgItemID,
			"output_index": 0, "content_index": 0, "delta": s,
		}))
		return out, nil
	}
	return nil, nil
}

func (c *StreamConverter) toolStart(idx int, id, name string) ([]sse.Event, error) {
	if id == "" {
		id = "call_" + randomSuffix()
	}
	if existing, ok := c.tools[idx]; ok {
		if name != "" {
			existing.Name = name
		}
		if existing.ID == "" {
			existing.ID = id
		}
		return nil, nil
	}
	c.tools[idx] = &ToolCall{ID: id, Type: "function", Name: name}
	switch c.DownstreamAPI {
	case APIChat:
		return c.chatChunk(map[string]any{
			"role": "assistant",
			"tool_calls": []map[string]any{{
				"index": idx, "id": id, "type": "function",
				"function": map[string]any{"name": name, "arguments": ""},
			}},
		}, nil)
	case APIMessages:
		out, err := c.ensureMessagesStart()
		if err != nil {
			return nil, err
		}
		if c.textOpen {
			out = append(out, c.closeLastBlock())
			c.textOpen = false
		}
		blockIdx := c.nextBlock
		c.nextBlock++
		c.blockForTool[idx] = blockIdx
		out = append(out, eventJSON("content_block_start", map[string]any{
			"type": "content_block_start", "index": blockIdx,
			"content_block": map[string]any{
				"type": "tool_use", "id": id, "name": name, "input": map[string]any{},
			},
		}))
		return out, nil
	case APIResponses:
		out, err := c.ensureResponsesStart()
		if err != nil {
			return nil, err
		}
		out = append(out, c.closeResponseTextForTool()...)
		outputIdx := 1 + idx
		itemID := "fc_" + suffix(id)
		out = append(out, eventJSON("response.output_item.added", map[string]any{
			"type": "response.output_item.added", "output_index": outputIdx,
			"item": map[string]any{
				"id": itemID, "type": "function_call", "status": "in_progress",
				"call_id": id, "name": name, "arguments": "",
			},
		}))
		return out, nil
	}
	return nil, nil
}

func (c *StreamConverter) toolArgs(idx int, args string) ([]sse.Event, error) {
	t := c.tools[idx]
	if t == nil {
		t = &ToolCall{ID: "call_" + randomSuffix(), Type: "function"}
		c.tools[idx] = t
	}
	t.Arguments += args
	switch c.DownstreamAPI {
	case APIChat:
		return c.chatChunk(map[string]any{
			"tool_calls": []map[string]any{{
				"index": idx, "function": map[string]any{"arguments": args},
			}},
		}, nil)
	case APIMessages:
		blockIdx, ok := c.blockForTool[idx]
		if !ok {
			return nil, nil
		}
		return []sse.Event{eventJSON("content_block_delta", map[string]any{
			"type": "content_block_delta", "index": blockIdx,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": args},
		})}, nil
	case APIResponses:
		return []sse.Event{eventJSON("response.function_call_arguments.delta", map[string]any{
			"type": "response.function_call_arguments.delta", "item_id": "fc_" + suffix(t.ID),
			"output_index": 1 + idx, "delta": args,
		})}, nil
	}
	return nil, nil
}

func (c *StreamConverter) chatChunk(delta map[string]any, finish any) ([]sse.Event, error) {
	c.started = true
	payload := map[string]any{
		"id": responseID(c.id, APIChat), "object": "chat.completion.chunk",
		"created": c.created, "model": c.Model,
		"choices": []map[string]any{{
			"index": 0, "delta": delta, "finish_reason": finish, "logprobs": nil,
		}},
	}
	return []sse.Event{eventJSON("", payload)}, nil
}

func (c *StreamConverter) ensureMessagesStart() ([]sse.Event, error) {
	if c.started {
		return nil, nil
	}
	c.started = true
	return []sse.Event{eventJSON("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id": responseID(c.id, APIMessages), "type": "message", "role": "assistant",
			"model": c.Model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": c.usage.PromptTokens, "output_tokens": 0},
		},
	})}, nil
}

func (c *StreamConverter) ensureResponsesStart() ([]sse.Event, error) {
	if c.started {
		return nil, nil
	}
	c.started = true
	return []sse.Event{eventJSON("response.created", map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id": responseID(c.id, APIResponses), "object": "response", "created_at": c.created,
			"status": "in_progress", "model": c.Model, "output": []any{}, "usage": nil,
		},
	})}, nil
}

func (c *StreamConverter) ensureResponseTextItem() []sse.Event {
	if c.textItemAdded {
		return nil
	}
	c.textItemAdded = true
	return []sse.Event{eventJSON("response.output_item.added", map[string]any{
		"type": "response.output_item.added", "output_index": 0,
		"item": map[string]any{
			"id": c.msgItemID, "type": "message", "status": "in_progress",
			"role": "assistant", "content": []any{},
		},
	})}
}

func (c *StreamConverter) closeResponseTextForTool() []sse.Event {
	if !c.textPart {
		return nil
	}
	c.textPart = false
	return []sse.Event{eventJSON("response.output_text.done", map[string]any{
		"type": "response.output_text.done", "item_id": c.msgItemID,
		"output_index": 0, "content_index": 0, "text": c.text.String(),
	})}
}

func (c *StreamConverter) toolOpenAny() bool { return len(c.tools) > 0 }

func (c *StreamConverter) closeLastBlock() sse.Event {
	idx := c.nextBlock - 1
	if c.textOpen {
		idx = c.textBlockIdx
	}
	return eventJSON("content_block_stop", map[string]any{"type": "content_block_stop", "index": idx})
}

func (c *StreamConverter) finalize() ([]sse.Event, error) {
	if c.finalized {
		return nil, nil
	}
	c.finalized = true
	out := []sse.Event{}
	switch c.DownstreamAPI {
	case APIChat:
		if !c.started {
			evs, err := c.chatChunk(map[string]any{"role": "assistant", "content": ""}, nil)
			if err != nil {
				return nil, err
			}
			out = append(out, evs...)
		}
		finish := c.finish
		if finish == "" {
			if len(c.tools) > 0 {
				finish = "tool_calls"
			} else {
				finish = "stop"
			}
		}
		evs, err := c.chatChunk(map[string]any{}, finish)
		if err != nil {
			return nil, err
		}
		out = append(out, evs...)
		if c.usage.PromptTokens > 0 || c.usage.CompletionTokens > 0 {
			out = append(out, eventJSON("", map[string]any{
				"id": responseID(c.id, APIChat), "object": "chat.completion.chunk",
				"created": c.created, "model": c.Model, "choices": []any{},
				"usage": map[string]any{
					"prompt_tokens": c.usage.PromptTokens, "completion_tokens": c.usage.CompletionTokens,
					"total_tokens": totalTokens(c.usage),
				},
			}))
		}
		out = append(out, sse.Event{Data: []byte("[DONE]")})
	case APIMessages:
		if !c.started {
			evs, err := c.ensureMessagesStart()
			if err != nil {
				return nil, err
			}
			out = append(out, evs...)
			out = append(out, eventJSON("content_block_start", map[string]any{
				"type": "content_block_start", "index": c.nextBlock,
				"content_block": map[string]any{"type": "text", "text": ""},
			}))
			c.nextBlock++
		}
		for i := 0; i < c.nextBlock; i++ {
			out = append(out, eventJSON("content_block_stop", map[string]any{
				"type": "content_block_stop", "index": i,
			}))
		}
		stop := upstreamStopReason(finishReason(Response{FinishReason: c.finish, ToolCalls: c.toolCallsSlice()}))
		out = append(out, eventJSON("message_delta", map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": c.usage.CompletionTokens},
		}))
		out = append(out, eventJSON("message_stop", map[string]any{"type": "message_stop"}))
	case APIResponses:
		if !c.started {
			evs, err := c.ensureResponsesStart()
			if err != nil {
				return nil, err
			}
			out = append(out, evs...)
		}
		output := []map[string]any{}
		if c.text.Len() > 0 {
			if !c.textPart {
				out = append(out, eventJSON("response.content_part.added", map[string]any{
					"type": "response.content_part.added", "item_id": c.msgItemID,
					"output_index": 0, "content_index": 0,
					"part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}},
				}))
			}
			out = append(out, eventJSON("response.output_text.done", map[string]any{
				"type": "response.output_text.done", "item_id": c.msgItemID,
				"output_index": 0, "content_index": 0, "text": c.text.String(),
			}))
			out = append(out, eventJSON("response.content_part.done", map[string]any{
				"type": "response.content_part.done", "item_id": c.msgItemID,
				"output_index": 0, "content_index": 0,
				"part": map[string]any{"type": "output_text", "text": c.text.String(), "annotations": []any{}},
			}))
			out = append(out, eventJSON("response.output_item.done", map[string]any{
				"type": "response.output_item.done", "output_index": 0,
				"item": map[string]any{
					"id": c.msgItemID, "type": "message", "status": "completed", "role": "assistant",
					"content": []map[string]any{{"type": "output_text", "text": c.text.String(), "annotations": []any{}}},
				},
			}))
			output = append(output, map[string]any{
				"id": c.msgItemID, "type": "message", "status": "completed", "role": "assistant",
				"content": []map[string]any{{
					"type": "output_text", "text": c.text.String(), "annotations": []any{},
				}},
			})
		}
		for i, t := range c.toolCallsSlice() {
			itemID := "fc_" + suffix(t.ID)
			out = append(out, eventJSON("response.output_item.done", map[string]any{
				"type": "response.output_item.done", "output_index": 1 + i,
				"item": map[string]any{
					"id": itemID, "type": "function_call", "status": "completed",
					"call_id": t.ID, "name": t.Name, "arguments": t.Arguments,
				},
			}))
			output = append(output, map[string]any{
				"id": itemID, "type": "function_call", "status": "completed",
				"call_id": t.ID, "name": t.Name, "arguments": t.Arguments,
			})
		}
		out = append(out, eventJSON("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": responseID(c.id, APIResponses), "object": "response", "created_at": c.created,
				"status": "completed", "model": c.Model, "output": output,
				"usage": map[string]any{
					"input_tokens": c.usage.PromptTokens, "output_tokens": c.usage.CompletionTokens,
					"total_tokens": totalTokens(c.usage),
				},
			},
		}))
	}
	return out, nil
}

func (c *StreamConverter) toolCallsSlice() []ToolCall {
	out := make([]ToolCall, 0, len(c.tools))
	for i := 0; i < c.nextTool; i++ {
		if t, ok := c.tools[i]; ok {
			out = append(out, *t)
		}
	}
	return out
}

// Usage returns the accumulated token usage.
func (c *StreamConverter) Usage() Usage { return c.usage }

// Text returns the accumulated text.
func (c *StreamConverter) Text() string { return c.text.String() }

// ToolCalls returns the accumulated function calls.
func (c *StreamConverter) ToolCalls() []ToolCall { return c.toolCallsSlice() }

func eventJSON(name string, payload any) sse.Event {
	b, _ := json.Marshal(payload)
	return sse.Event{Event: name, Data: b}
}
