package translate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gitsang/aigw/pkg/sse"
)

func TestChatToMessagesRoundTrip(t *testing.T) {
	reqBody := []byte(`{
"model":"deepseek-flash","stream":false,"max_tokens":100,
"messages":[
{"role":"system","content":"be brief"},
{"role":"user","content":"hello"},
{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},
{"role":"tool","tool_call_id":"call_1","content":"result"}
],
"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]
}`)
	req, err := ParseRequest(APIChat, reqBody)
	if err != nil {
		t.Fatal(err)
	}
	out, err := BuildUpstreamRequest(APIMessages, req, "deepseek/deepseek-flash")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model"] != "deepseek/deepseek-flash" || m["system"] != "be brief" {
		t.Fatalf("unexpected upstream request: %s", out)
	}
	msgs := m["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages got %d: %s", len(msgs), out)
	}
}

func TestParseResponseAndBuildChat(t *testing.T) {
	body := []byte(`{"id":"msg_1","model":"x","content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"tool_1","name":"f","input":{"a":1}}],"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":7}}`)
	resp, err := ParseResponse(APIMessages, body)
	if err != nil {
		t.Fatal(err)
	}
	out, err := BuildResponse(APIChat, resp, "deepseek-flash")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model"] != "deepseek-flash" {
		t.Fatalf("model not rewritten: %s", out)
	}
	usage := m["usage"].(map[string]any)
	if usage["prompt_tokens"].(float64) != 5 || usage["completion_tokens"].(float64) != 7 {
		t.Fatalf("bad usage: %s", out)
	}
	choice := m["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("bad finish reason: %s", out)
	}
	msg := choice["message"].(map[string]any)
	if msg["content"] != "hi" || len(msg["tool_calls"].([]any)) != 1 {
		t.Fatalf("bad message: %s", out)
	}
}

func TestResponsesRequestToChat(t *testing.T) {
	body := []byte(`{
"model":"deepseek-flash","instructions":"be exact","input":"hello",
"max_output_tokens":50,
"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]
}`)
	req, err := ParseRequest(APIResponses, body)
	if err != nil {
		t.Fatal(err)
	}
	out, err := BuildUpstreamRequest(APIChat, req, "upstream-model")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model"] != "upstream-model" || m["max_tokens"].(float64) != 50 {
		t.Fatalf("bad output: %s", out)
	}
	if !strings.Contains(string(out), "be exact") {
		t.Fatalf("system not translated: %s", out)
	}
}

func TestStreamChatToMessages(t *testing.T) {
	conv := NewStreamConverter(APIChat, APIMessages, "deepseek-flash")
	chunks := []string{
		`{"id":"chatcmpl-1","created":123,"choices":[{"index":0,"delta":{"role":"assistant","content":"hel"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
	}
	var events []sse.Event
	for _, chunk := range chunks {
		evs, err := conv.Feed(sse.Event{Data: []byte(chunk)})
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, evs...)
	}
	evs, err := conv.Feed(sse.Event{Data: []byte("[DONE]")})
	if err != nil {
		t.Fatal(err)
	}
	events = append(events, evs...)
	var names []string
	for _, ev := range events {
		names = append(names, ev.Event)
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"message_start", "content_block_start", "content_block_delta", "message_delta", "message_stop"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
	if conv.Usage().TotalTokens != 5 || conv.Text() != "hello" {
		t.Fatalf("bad accumulated state: %+v %q", conv.Usage(), conv.Text())
	}
}

func TestInteractionsRequestToChat(t *testing.T) {
	body := []byte(`{
"model":"gemini-4-hydrogen","stream":false,
"system_instruction":"be brief",
"generation_config":{"max_output_tokens":64,"stop_sequences":["END"],"tool_choice":"any"},
"tools":[{"type":"function","name":"lookup","description":"look it up","parameters":{"type":"object"}},
         {"type":"code_execution"}],
"input":[
 {"type":"user_input","content":[{"type":"text","text":"hello"}]},
 {"type":"model_output","content":[{"type":"text","text":"let me check"},{"type":"thought","text":"ignored"}]},
 {"type":"function_call","id":"call_9","name":"lookup","arguments":{"q":"x"}},
 {"type":"google_search_call","id":"srv_1"},
 {"type":"function_result","call_id":"call_9","name":"lookup","result":[{"type":"text","text":"found"}]}
]}`)
	req, err := ParseRequest(APIInteractions, body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gemini-4-hydrogen" || req.MaxTokens != 64 {
		t.Fatalf("bad parse: %+v", req)
	}
	if len(req.Stop) != 1 || req.Stop[0] != "END" {
		t.Fatalf("bad stop sequences: %+v", req.Stop)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "lookup" {
		t.Fatalf("server-side tools must be skipped: %+v", req.Tools)
	}
	if req.ToolChoice != "required" {
		t.Fatalf("tool_choice any must map to required, got %v", req.ToolChoice)
	}
	// system, user, assistant text, assistant tool call, tool result
	if len(req.Messages) != 5 {
		t.Fatalf("expected 5 messages got %d: %+v", len(req.Messages), req.Messages)
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "be brief" {
		t.Fatalf("bad system message: %+v", req.Messages[0])
	}
	call := req.Messages[3]
	if len(call.ToolCalls) != 1 || call.ToolCalls[0].ID != "call_9" || call.ToolCalls[0].Arguments != `{"q":"x"}` {
		t.Fatalf("arguments must become an IR JSON string: %+v", call.ToolCalls)
	}
	if req.Messages[4].Role != "tool" || req.Messages[4].ToolCallID != "call_9" || req.Messages[4].Content != "found" {
		t.Fatalf("bad function result message: %+v", req.Messages[4])
	}

	out, err := BuildUpstreamRequest(APIChat, req, "upstream-model")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model"] != "upstream-model" || m["max_tokens"].(float64) != 64 {
		t.Fatalf("bad chat payload: %s", out)
	}
	if !strings.Contains(string(out), `"arguments":"{\"q\":\"x\"}"`) {
		t.Fatalf("tool argument string not preserved: %s", out)
	}
}

func TestChatToInteractionsRequestKeepsToolsAndDropsSampling(t *testing.T) {
	body := []byte(`{
"model":"deepseek-flash","stream":true,"max_tokens":100,"temperature":0.7,"top_p":0.9,
"messages":[
 {"role":"system","content":"be brief"},
 {"role":"user","content":"weather?"},
 {"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Boston\"}"}}]},
 {"role":"tool","tool_call_id":"call_1","content":"52F"}
],
"tools":[{"type":"function","function":{"name":"get_weather","description":"w","parameters":{"type":"object"}}}]}`)
	req, err := ParseRequest(APIChat, body)
	if err != nil {
		t.Fatal(err)
	}
	out, err := BuildUpstreamRequest(APIInteractions, req, "gemini-4-hydrogen")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["model"] != "gemini-4-hydrogen" || m["stream"] != true || m["store"] != false {
		t.Fatalf("bad interactions payload: %s", out)
	}
	if m["system_instruction"] != "be brief" {
		t.Fatalf("system_instruction must be top-level: %s", out)
	}
	if _, ok := m["temperature"]; ok {
		t.Fatalf("temperature does not exist in the interactions API: %s", out)
	}
	if _, ok := m["top_p"]; ok {
		t.Fatalf("top_p does not exist in the interactions API: %s", out)
	}
	gen := m["generation_config"].(map[string]any)
	if gen["max_output_tokens"].(float64) != 100 {
		t.Fatalf("max_output_tokens must live in generation_config: %s", out)
	}
	steps := m["input"].([]any)
	types := make([]string, 0, len(steps))
	for _, raw := range steps {
		types = append(types, raw.(map[string]any)["type"].(string))
	}
	// The assistant turn carries no text, so no empty model_output step is
	// emitted; the tool call and its result are steps of their own.
	want := []string{"user_input", "function_call", "function_result"}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected step sequence %v: %s", types, out)
	}
	call := steps[1].(map[string]any)
	args, ok := call["arguments"].(map[string]any)
	if !ok || args["location"] != "Boston" {
		t.Fatalf("function_call arguments must be a JSON object: %s", out)
	}
	tools := m["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "get_weather" {
		t.Fatalf("tools must be flat declarations: %s", out)
	}
	if _, nested := tool["function"]; nested {
		t.Fatalf("tools must not nest a function object: %s", out)
	}
	result := steps[2].(map[string]any)
	if result["call_id"] != "call_1" {
		t.Fatalf("function_result must carry call_id: %s", out)
	}
}

func TestParseInteractionsResponse(t *testing.T) {
	body := []byte(`{
"id":"v1_abc","object":"interaction","status":"requires_action","model":"gemini-4-hydrogen",
"created":"2026-01-02T03:04:05Z","updated":"2026-01-02T03:04:05Z",
"steps":[
 {"type":"thought","content":[{"type":"text","text":"thinking"}]},
 {"type":"model_output","content":[{"type":"text","text":"Hel"},{"type":"image","data":"x"},{"type":"text","text":"lo"}]},
 {"type":"function_call","id":"call_7","name":"get_weather","arguments":{"location":"Boston"}},
 {"type":"google_search_call","id":"srv_1"}
],
"usage":{"total_input_tokens":11,"total_output_tokens":90,"total_tokens":346,"total_cached_tokens":4}}`)
	resp, err := ParseResponse(APIInteractions, body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "v1_abc" || resp.Model != "gemini-4-hydrogen" {
		t.Fatalf("bad response: %+v", resp)
	}
	if resp.Content != "Hello" {
		t.Fatalf("text must come from model_output text blocks only, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call_7" || resp.ToolCalls[0].Arguments != `{"location":"Boston"}` {
		t.Fatalf("bad tool calls: %+v", resp.ToolCalls)
	}
	if resp.FinishReason != "tool_calls" {
		t.Fatalf("requires_action must map to tool_calls, got %q", resp.FinishReason)
	}
	if resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 90 ||
		resp.Usage.TotalTokens != 346 || resp.Usage.CacheReadTokens != 4 {
		t.Fatalf("bad usage: %+v", resp.Usage)
	}
	if resp.Created != 1767323045 {
		t.Fatalf("RFC3339 created not parsed: %d", resp.Created)
	}

	out, err := BuildResponse(APIChat, resp, "virtual")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	choice := m["choices"].([]any)[0].(map[string]any)
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("finish reason lost: %s", out)
	}
	msg := choice["message"].(map[string]any)
	if msg["content"] != "Hello" {
		t.Fatalf("content lost: %s", out)
	}
	call := msg["tool_calls"].([]any)[0].(map[string]any)
	if call["function"].(map[string]any)["arguments"] != `{"location":"Boston"}` {
		t.Fatalf("arguments must stay a JSON string downstream: %s", out)
	}
}

func TestBuildInteractionsResponse(t *testing.T) {
	resp := Response{
		ID: "chatcmpl-1", Model: "up", Created: 1767323045,
		Content:   "hello",
		ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Name: "lookup", Arguments: `{"q":"x"}`}},
		Usage:     Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5},
	}
	out, err := BuildResponse(APIInteractions, resp, "gemini-4-hydrogen")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["object"] != "interaction" || m["status"] != "requires_action" {
		t.Fatalf("bad interaction envelope: %s", out)
	}
	if m["model"] != "gemini-4-hydrogen" {
		t.Fatalf("model not rewritten: %s", out)
	}
	if m["created"] != "2026-01-02T03:04:05Z" || m["updated"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("created/updated must be RFC3339: %s", out)
	}
	steps := m["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("expected a model_output and a function_call step: %s", out)
	}
	text := steps[0].(map[string]any)
	content := text["content"].([]any)[0].(map[string]any)
	if text["type"] != "model_output" || content["type"] != "text" || content["text"] != "hello" {
		t.Fatalf("bad model_output step: %s", out)
	}
	call := steps[1].(map[string]any)
	args, ok := call["arguments"].(map[string]any)
	if !ok || args["q"] != "x" {
		t.Fatalf("function_call arguments must be a JSON object: %s", out)
	}
	usage := m["usage"].(map[string]any)
	if usage["total_input_tokens"].(float64) != 3 || usage["total_output_tokens"].(float64) != 2 ||
		usage["total_tokens"].(float64) != 5 {
		t.Fatalf("bad usage: %s", out)
	}

	// The emitted interaction must be parseable back into the same IR.
	back, err := ParseResponse(APIInteractions, out)
	if err != nil {
		t.Fatal(err)
	}
	if back.Content != "hello" || len(back.ToolCalls) != 1 || back.ToolCalls[0].Arguments != `{"q":"x"}` {
		t.Fatalf("round-trip failed: %+v", back)
	}
}

func TestInteractionsUnknownStepsAndDeltasAreSkipped(t *testing.T) {
	body := []byte(`{"id":"v1_1","status":"completed","steps":[
{"type":"thought","content":[{"type":"text","text":"hidden"}]},
{"type":"code_execution_result","id":"srv"},
{"type":"model_output","content":[{"type":"audio","data":"zz"},{"type":"text","text":"visible"}]},
{"type":"totally_unknown_step"}]}`)
	resp, err := ParseResponse(APIInteractions, body)
	if err != nil {
		t.Fatalf("unknown step types must not be fatal: %v", err)
	}
	if resp.Content != "visible" {
		t.Fatalf("unexpected content %q", resp.Content)
	}

	conv := NewStreamConverter(APIInteractions, APIMessages, "m")
	unknown := []sse.Event{
		{Event: "interaction.status_update", Data: []byte(`{"interaction_id":"v1_1","status":"in_progress","event_type":"interaction.status_update"}`)},
		{Event: "step.delta", Data: []byte(`{"index":0,"delta":{"type":"image","data":"zz"},"event_type":"step.delta"}`)},
		{Event: "step.delta", Data: []byte(`{"index":0,"delta":{"type":"future_delta","x":1},"event_type":"step.delta"}`)},
		{Event: "totally.unknown", Data: []byte(`{"event_type":"totally.unknown"}`)},
	}
	for _, ev := range unknown {
		if _, err := conv.Feed(ev); err != nil {
			t.Fatalf("unknown event %q must not be fatal: %v", ev.Event, err)
		}
	}
	// A well-formed event afterwards must still work.
	evs, err := conv.Feed(sse.Event{Event: "step.delta", Data: []byte(`{"index":1,"delta":{"type":"text","text":"ok"},"event_type":"step.delta"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) == 0 || conv.Text() != "ok" {
		t.Fatalf("stream did not recover after unknown events: %q", conv.Text())
	}
}

func TestStreamInteractionsToolCallDownstream(t *testing.T) {
	conv := NewStreamConverter(APIInteractions, APIChat, "gemini-4-hydrogen")
	events := []sse.Event{
		{Event: "interaction.created", Data: []byte(`{"interaction":{"id":"v1_1","status":"in_progress","model":"gemini-4-hydrogen"},"event_type":"interaction.created"}`)},
		{Event: "step.start", Data: []byte(`{"index":0,"step":{"type":"function_call","id":"call_5","name":"get_weather","arguments":{}},"event_type":"step.start"}`)},
		{Event: "step.delta", Data: []byte(`{"index":0,"delta":{"type":"arguments_delta","arguments":"{\"location\":"},"event_type":"step.delta"}`)},
		{Event: "step.delta", Data: []byte(`{"index":0,"delta":{"type":"arguments_delta","arguments":"\"Boston\"}"},"event_type":"step.delta"}`)},
		{Event: "step.stop", Data: []byte(`{"index":0,"event_type":"step.stop"}`)},
		{Event: "interaction.completed", Data: []byte(`{"interaction":{"id":"v1_1","status":"requires_action","usage":{"total_input_tokens":4,"total_output_tokens":6,"total_tokens":10}},"event_type":"interaction.completed"}`)},
		{Event: "done", Data: []byte(`[DONE]`)},
	}
	var joined []string
	for _, ev := range events {
		out, err := conv.Feed(ev)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range out {
			joined = append(joined, string(o.Data))
		}
	}
	calls := conv.ToolCalls()
	if len(calls) != 1 || calls[0].Name != "get_weather" || calls[0].ID != "call_5" {
		t.Fatalf("bad accumulated tool calls: %+v", calls)
	}
	if calls[0].Arguments != `{"location":"Boston"}` {
		t.Fatalf("arguments deltas not accumulated: %q", calls[0].Arguments)
	}
	if !strings.Contains(strings.Join(joined, "\n"), `"tool_calls"`) {
		t.Fatalf("chat downstream did not receive the tool call: %s", strings.Join(joined, "\n"))
	}
}

func TestStreamInteractionsToInteractions(t *testing.T) {
	conv := NewStreamConverter(APIChat, APIInteractions, "gemini-4-hydrogen")
	chunks := []sse.Event{
		eventData(`{"id":"chatcmpl-1","created":1767323045,"choices":[{"index":0,"delta":{"role":"assistant","content":"hel"}}]}`),
		eventData(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]}}]}`),
		eventData(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]},"finish_reason":"tool_calls"}]}`),
		eventData(`{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`),
	}
	var out []sse.Event
	for _, ev := range chunks {
		evs, err := conv.Feed(ev)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, evs...)
	}
	final, err := conv.Feed(eventData(`[DONE]`))
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, final...)
	assertInteractionsStreamShape(t, "chat", out)

	var order []string
	var starts int
	seenText := false
	seenArgs := false
	for _, ev := range out {
		order = append(order, ev.Event)
		if ev.Event == "step.start" {
			starts++
		}
		if ev.Event == "step.delta" {
			var m map[string]any
			if err := json.Unmarshal(ev.Data, &m); err != nil {
				t.Fatal(err)
			}
			switch m["delta"].(map[string]any)["type"] {
			case "text":
				seenText = true
			case "arguments_delta":
				seenArgs = true
			}
		}
	}
	if !seenText || !seenArgs {
		t.Fatalf("missing text/arguments deltas: %v", order)
	}
	if starts < 2 {
		t.Fatalf("expected a model_output and a function_call step, got %d: %v", starts, order)
	}
	// Each step.start must be matched by a step.stop before interaction.completed.
	open := 0
	for _, name := range order {
		switch name {
		case "step.start":
			open++
		case "step.stop":
			open--
		case "interaction.completed", "done":
			if open != 0 {
				t.Fatalf("unbalanced steps before %s: %v", name, order)
			}
		}
	}
	if order[len(order)-1] != "done" {
		t.Fatalf("last event must be done: %v", order)
	}
	if order[len(order)-2] != "interaction.completed" {
		t.Fatalf("done must follow interaction.completed: %v", order)
	}
}

func TestInteractionsResponseFormatAndToolChoiceRoundTrip(t *testing.T) {
	body := []byte(`{
"model":"m","input":"hi",
"generation_config":{"tool_choice":{"allowed_tools":{"mode":"any","tools":["lookup"]}},"max_output_tokens":8},
"response_format":{"type":"text","mime_type":"application/json","schema":{"type":"object"}},
"previous_interaction_id":"v1_prev","store":true}`)
	req, err := ParseRequest(APIInteractions, body)
	if err != nil {
		t.Fatal(err)
	}
	if req.PreviousInteractionID != "v1_prev" || !req.Store {
		t.Fatalf("previous_interaction_id/store not parsed: %+v", req)
	}
	out, err := BuildUpstreamRequest(APIInteractions, req, "up")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["previous_interaction_id"] != "v1_prev" || m["store"] != true {
		t.Fatalf("previous_interaction_id/store not propagated: %s", out)
	}
	gen := m["generation_config"].(map[string]any)
	choice, ok := gen["tool_choice"].(map[string]any)
	if !ok {
		t.Fatalf("tool_choice not translated: %s", out)
	}
	allowed := choice["allowed_tools"].(map[string]any)
	if allowed["mode"] != "any" || allowed["tools"].([]any)[0] != "lookup" {
		t.Fatalf("tool_choice not translated: %s", out)
	}
	format := m["response_format"].(map[string]any)
	if format["type"] != "text" || format["mime_type"] != "application/json" {
		t.Fatalf("response_format not translated: %s", out)
	}
	if _, ok := format["schema"]; !ok {
		t.Fatalf("response_format schema lost: %s", out)
	}
}

func TestStreamInteractionsCompletedOnlyHydrates(t *testing.T) {
	// Some upstreams only send the terminal Interaction object with assembled
	// steps. The converter must still forward text and tool calls downstream.
	conv := NewStreamConverter(APIInteractions, APIChat, "gemini-4-hydrogen")
	events := []sse.Event{
		{Event: "interaction.created", Data: []byte(`{"interaction":{"id":"v1_1","status":"in_progress","object":"interaction","model":"gemini-4-hydrogen"},"event_type":"interaction.created"}`)},
		{Event: "interaction.completed", Data: []byte(`{"interaction":{"id":"v1_1","status":"completed","object":"interaction","model":"gemini-4-hydrogen","usage":{"total_input_tokens":4,"total_output_tokens":6,"total_tokens":10},"steps":[{"type":"model_output","content":[{"type":"text","text":"hi there"}]},{"type":"function_call","id":"call_2","name":"lookup","arguments":{"q":"x"}}]},"event_type":"interaction.completed"}`)},
		{Event: "done", Data: []byte(`[DONE]`)},
	}
	var joined []string
	for _, ev := range events {
		out, err := conv.Feed(ev)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range out {
			joined = append(joined, string(o.Data))
		}
	}
	all := strings.Join(joined, "\n")
	if conv.Text() != "hi there" {
		t.Fatalf("text not hydrated: %q", conv.Text())
	}
	calls := conv.ToolCalls()
	if len(calls) != 1 || calls[0].Name != "lookup" || calls[0].Arguments != `{"q":"x"}` {
		t.Fatalf("tool call not hydrated: %+v", calls)
	}
	if !strings.Contains(all, "hi there") || !strings.Contains(all, `"tool_calls"`) {
		t.Fatalf("hydrated content not forwarded downstream: %s", all)
	}
	if conv.Usage().TotalTokens != 10 {
		t.Fatalf("usage not hydrated: %+v", conv.Usage())
	}

	// The same hydration into an interactions downstream must not duplicate text.
	conv2 := NewStreamConverter(APIInteractions, APIInteractions, "gemini-4-hydrogen")
	var out2 []sse.Event
	for _, ev := range events {
		evs, err := conv2.Feed(ev)
		if err != nil {
			t.Fatal(err)
		}
		out2 = append(out2, evs...)
	}
	if conv2.Text() != "hi there" {
		t.Fatalf("text hydrated twice or lost: %q", conv2.Text())
	}
	var textDeltas int
	for _, ev := range out2 {
		if ev.Event != "step.delta" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Data, &m); err != nil {
			t.Fatal(err)
		}
		if m["delta"].(map[string]any)["type"] == "text" {
			textDeltas++
		}
	}
	if textDeltas != 1 {
		t.Fatalf("expected exactly one text delta, got %d", textDeltas)
	}
}

func TestInteractionsDownstreamNoDeltaAfterStepStop(t *testing.T) {
	conv := NewStreamConverter(APIChat, APIInteractions, "gemini-4-hydrogen")
	chunks := []string{
		`{"id":"chatcmpl-1","created":1767323045,"choices":[{"index":0,"delta":{"role":"assistant","content":"hi"}}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"f","arguments":"{\"a\":"}}]}}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"g","arguments":"{\"b\":"}}]}}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"2}"}}]},"finish_reason":"tool_calls"}]}`,
	}
	var out []sse.Event
	for _, chunk := range chunks {
		evs, err := conv.Feed(eventData(chunk))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, evs...)
	}
	final, err := conv.Feed(eventData(`[DONE]`))
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, final...)

	stopped := map[int]bool{}
	for _, ev := range out {
		if ev.Event == "done" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Data, &m); err != nil {
			t.Fatal(err)
		}
		idx := int(intValue(m["index"]))
		switch ev.Event {
		case "step.start":
			if stopped[idx] {
				t.Fatalf("step %d restarted after stop: %s", idx, ev.Data)
			}
		case "step.delta":
			if stopped[idx] {
				t.Fatalf("delta after step.stop for step %d: %s", idx, ev.Data)
			}
		case "step.stop":
			stopped[idx] = true
		}
	}
	calls := conv.ToolCalls()
	if len(calls) != 2 {
		t.Fatalf("expected two accumulated tool calls, got %+v", calls)
	}
	if calls[0].Arguments != `{"a":1}` || calls[1].Arguments != `{"b":2}` {
		t.Fatalf("interleaved argument deltas not accumulated per call: %+v", calls)
	}
}
