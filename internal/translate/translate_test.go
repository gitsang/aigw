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
