package translate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gitsang/aigw/pkg/sse"
)

func TestRequestTranslationMatrix(t *testing.T) {
	requests := map[string][]byte{
		APIChat:         []byte(`{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}],"max_tokens":16,"stream":false}`),
		APIResponses:    []byte(`{"model":"m","instructions":"sys","input":"hi","max_output_tokens":16}`),
		APIMessages:     []byte(`{"model":"m","system":"sys","messages":[{"role":"user","content":"hi"}],"max_tokens":16}`),
		APIInteractions: []byte(`{"model":"m","system_instruction":"sys","input":"hi","generation_config":{"max_output_tokens":16}}`),
	}
	// Every inbound protocol must translate to every upstream protocol.
	if len(requests) != 4 {
		t.Fatalf("expected 4 inbound protocols, got %d", len(requests))
	}
	for inType, body := range requests {
		req, err := ParseRequest(inType, body)
		if err != nil {
			t.Fatalf("parse %s: %v", inType, err)
		}
		for outType := range requests {
			out, err := BuildUpstreamRequest(outType, req, "upstream/model")
			if err != nil {
				t.Fatalf("build %s -> %s: %v", inType, outType, err)
			}
			var m map[string]any
			if err := json.Unmarshal(out, &m); err != nil {
				t.Fatalf("invalid JSON for %s -> %s: %v", inType, outType, err)
			}
			if m["model"] != "upstream/model" {
				t.Fatalf("%s -> %s model not rewritten: %s", inType, outType, out)
			}
			// The system instruction must survive in the upstream shape.
			if outType == APIInteractions {
				if m["system_instruction"] != "sys" {
					t.Fatalf("%s -> %s lost system instruction: %s", inType, outType, out)
				}
				if _, ok := m["temperature"]; ok {
					t.Fatalf("%s -> %s must not send temperature: %s", inType, outType, out)
				}
				if _, ok := m["top_p"]; ok {
					t.Fatalf("%s -> %s must not send top_p: %s", inType, outType, out)
				}
				// The user turn must become a user_input step carrying "hi".
				if !strings.Contains(string(out), `"user_input"`) || !strings.Contains(string(out), `"hi"`) {
					t.Fatalf("%s -> %s lost the user turn: %s", inType, outType, out)
				}
				gen, ok := m["generation_config"].(map[string]any)
				if !ok || gen["max_output_tokens"].(float64) != 16 {
					t.Fatalf("%s -> %s lost max tokens: %s", inType, outType, out)
				}
			}
			if outType == APIChat {
				if !strings.Contains(string(out), `"hi"`) {
					t.Fatalf("%s -> %s lost the user turn: %s", inType, outType, out)
				}
			}
		}
	}
}

func TestResponseTranslationMatrix(t *testing.T) {
	responses := map[string]struct {
		body string
	}{
		APIChat:         {`{"id":"chatcmpl-1","model":"up","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`},
		APIResponses:    {`{"id":"resp_1","model":"up","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`},
		APIMessages:     {`{"id":"msg_1","model":"up","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`},
		APIInteractions: {`{"id":"v1_1","object":"interaction","status":"completed","model":"up","created":"2026-01-02T03:04:05Z","steps":[{"type":"model_output","content":[{"type":"text","text":"hello"}]}],"usage":{"total_input_tokens":3,"total_output_tokens":2,"total_tokens":5}}`},
	}
	for inType, fixture := range responses {
		parsed, err := ParseResponse(inType, []byte(fixture.body))
		if err != nil {
			t.Fatalf("parse response %s: %v", inType, err)
		}
		for outType := range responses {
			out, err := BuildResponse(outType, parsed, "virtual-model")
			if err != nil {
				t.Fatalf("build response %s -> %s: %v", inType, outType, err)
			}
			if !strings.Contains(string(out), `"virtual-model"`) {
				t.Fatalf("%s -> %s did not rewrite model: %s", inType, outType, out)
			}
			if !strings.Contains(string(out), "hello") {
				t.Fatalf("%s -> %s lost content: %s", inType, outType, out)
			}
		}
	}
}

func TestStreamTranslationMatrix(t *testing.T) {
	streams := map[string][]sse.Event{
		APIChat: {
			eventData(`{"id":"chatcmpl-1","created":1,"choices":[{"index":0,"delta":{"role":"assistant","content":"hello"},"finish_reason":null}]}`),
			eventData(`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
			eventData(`{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`),
			eventData(`[DONE]`),
		},
		APIResponses: {
			{Event: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_1","created_at":1,"model":"up","status":"in_progress"}}`)},
			{Event: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"hello","item_id":"msg_1","output_index":0,"content_index":0}`)},
			{Event: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_1","created_at":1,"status":"completed","model":"up","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}}`)},
		},
		APIMessages: {
			{Event: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_1","model":"up","usage":{"input_tokens":3,"output_tokens":0}}}`)},
			{Event: "content_block_start", Data: []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)},
			{Event: "content_block_delta", Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`)},
			{Event: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)},
			{Event: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
		},
		APIInteractions: {
			{Event: "interaction.created", Data: []byte(`{"interaction":{"id":"v1_1","status":"in_progress","object":"interaction","model":"up"},"event_type":"interaction.created"}`)},
			{Event: "step.start", Data: []byte(`{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`)},
			{Event: "step.delta", Data: []byte(`{"index":0,"delta":{"type":"text","text":"hello"},"event_type":"step.delta"}`)},
			{Event: "step.stop", Data: []byte(`{"index":0,"event_type":"step.stop"}`)},
			{Event: "interaction.completed", Data: []byte(`{"interaction":{"id":"v1_1","status":"completed","object":"interaction","model":"up","usage":{"total_input_tokens":3,"total_output_tokens":2,"total_tokens":5}},"event_type":"interaction.completed"}`)},
			{Event: "done", Data: []byte(`[DONE]`)},
		},
	}
	for inType, events := range streams {
		for outType := range streams {
			conv := NewStreamConverter(inType, outType, "virtual-model")
			var emitted []sse.Event
			for _, ev := range events {
				out, err := conv.Feed(ev)
				if err != nil {
					t.Fatalf("stream %s -> %s: %v", inType, outType, err)
				}
				emitted = append(emitted, out...)
			}
			if len(emitted) == 0 {
				t.Fatalf("stream %s -> %s produced no events", inType, outType)
			}
			if conv.Text() != "hello" {
				t.Fatalf("stream %s -> %s text=%q", inType, outType, conv.Text())
			}
			if conv.Usage().PromptTokens != 3 || conv.Usage().CompletionTokens != 2 {
				t.Fatalf("stream %s -> %s usage=%+v", inType, outType, conv.Usage())
			}
			if outType == APIInteractions {
				assertInteractionsStreamShape(t, inType, emitted)
			}
		}
	}
}

// assertInteractionsStreamShape verifies the Interactions SSE contract: named
// events, the event name duplicated as `event_type` inside each JSON payload,
// and a terminating `done` event carrying [DONE].
func assertInteractionsStreamShape(t *testing.T, inType string, events []sse.Event) {
	t.Helper()
	names := map[string]bool{}
	for _, ev := range events {
		if ev.Event == "done" {
			if string(ev.Data) != "[DONE]" {
				t.Fatalf("%s -> interactions done data=%q", inType, ev.Data)
			}
			continue
		}
		names[ev.Event] = true
		var m map[string]any
		if err := json.Unmarshal(ev.Data, &m); err != nil {
			t.Fatalf("%s -> interactions invalid JSON event %q: %v", inType, ev.Event, err)
		}
		if m["event_type"] != ev.Event {
			t.Fatalf("%s -> interactions event_type=%v does not match %q: %s",
				inType, m["event_type"], ev.Event, ev.Data)
		}
	}
	for _, want := range []string{"interaction.created", "step.start", "step.delta", "step.stop", "interaction.completed"} {
		if !names[want] {
			t.Fatalf("%s -> interactions missing %q in stream", inType, want)
		}
	}
}

// TestToolCallTranslationMatrix checks that a tool call and its result survive
// every inbound x upstream direction pair, in both the request and the response
// direction.
func TestToolCallTranslationMatrix(t *testing.T) {
	// Inbound bodies that all request one tool call and carry one tool result.
	requests := map[string][]byte{
		APIChat: []byte(`{"model":"m","messages":[
			{"role":"user","content":"weather?"},
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Boston\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"52F"}],
			"tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object"}}}]}`),
		APIResponses: []byte(`{"model":"m","input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"weather?"}]},
			{"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"location\":\"Boston\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"52F"}],
			"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]}`),
		APIMessages: []byte(`{"model":"m","messages":[
			{"role":"user","content":[{"type":"text","text":"weather?"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"get_weather","input":{"location":"Boston"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"52F"}]}],
			"tools":[{"name":"get_weather","input_schema":{"type":"object"}}]}`),
		APIInteractions: []byte(`{"model":"m","input":[
			{"type":"user_input","content":[{"type":"text","text":"weather?"}]},
			{"type":"function_call","id":"call_1","name":"get_weather","arguments":{"location":"Boston"}},
			{"type":"function_result","call_id":"call_1","name":"get_weather","result":[{"type":"text","text":"52F"}]}],
			"tools":[{"type":"function","name":"get_weather","parameters":{"type":"object"}}]}`),
	}
	apiTypes := []string{APIChat, APIResponses, APIMessages, APIInteractions}
	if len(requests) != 4 {
		t.Fatalf("expected 4 inbound protocols, got %d", len(requests))
	}
	for _, inType := range apiTypes {
		req, err := ParseRequest(inType, requests[inType])
		if err != nil {
			t.Fatalf("parse %s: %v", inType, err)
		}
		if len(req.Tools) != 1 {
			t.Fatalf("%s: tool declaration lost: %+v", inType, req.Tools)
		}
		var callSeen, resultSeen bool
		for _, msg := range req.Messages {
			if len(msg.ToolCalls) > 0 && msg.ToolCalls[0].Name == "get_weather" {
				callSeen = true
				if msg.ToolCalls[0].Arguments != `{"location":"Boston"}` {
					t.Fatalf("%s: arguments must be an IR JSON string: %q", inType, msg.ToolCalls[0].Arguments)
				}
			}
			if msg.Role == "tool" && msg.ToolCallID == "call_1" && textContent(msg.Content) == "52F" {
				resultSeen = true
			}
		}
		if !callSeen || !resultSeen {
			t.Fatalf("%s: tool call/result not parsed into the IR: %+v", inType, req.Messages)
		}
		for _, outType := range apiTypes {
			out, err := BuildUpstreamRequest(outType, req, "upstream")
			if err != nil {
				t.Fatalf("build %s -> %s: %v", inType, outType, err)
			}
			if !strings.Contains(string(out), "get_weather") {
				t.Fatalf("%s -> %s lost the tool declaration: %s", inType, outType, out)
			}
			if !strings.Contains(string(out), "52F") {
				t.Fatalf("%s -> %s lost the tool result: %s", inType, outType, out)
			}
			if !strings.Contains(string(out), "Boston") {
				t.Fatalf("%s -> %s lost the tool arguments: %s", inType, outType, out)
			}
		}
	}

	// Response direction: every inbound response parses one tool call which
	// every downstream builder must emit.
	responses := map[string]string{
		APIChat:         `{"id":"chatcmpl-1","model":"up","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"location\":\"Boston\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
		APIResponses:    `{"id":"resp_1","model":"up","status":"completed","output":[{"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"location\":\"Boston\"}"}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`,
		APIMessages:     `{"id":"msg_1","model":"up","type":"message","role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"get_weather","input":{"location":"Boston"}}],"stop_reason":"tool_use","usage":{"input_tokens":3,"output_tokens":2}}`,
		APIInteractions: `{"id":"v1_1","object":"interaction","status":"requires_action","model":"up","created":"2026-01-02T03:04:05Z","steps":[{"type":"function_call","id":"call_1","name":"get_weather","arguments":{"location":"Boston"}}],"usage":{"total_input_tokens":3,"total_output_tokens":2,"total_tokens":5}}`,
	}
	for _, inType := range apiTypes {
		parsed, err := ParseResponse(inType, []byte(responses[inType]))
		if err != nil {
			t.Fatalf("parse response %s: %v", inType, err)
		}
		if len(parsed.ToolCalls) != 1 || parsed.ToolCalls[0].Name != "get_weather" {
			t.Fatalf("response %s: tool call lost: %+v", inType, parsed.ToolCalls)
		}
		if parsed.FinishReason != "tool_calls" {
			t.Fatalf("response %s: finish reason must be tool_calls, got %q", inType, parsed.FinishReason)
		}
		for _, outType := range apiTypes {
			out, err := BuildResponse(outType, parsed, "virtual-model")
			if err != nil {
				t.Fatalf("build response %s -> %s: %v", inType, outType, err)
			}
			if !strings.Contains(string(out), "get_weather") {
				t.Fatalf("response %s -> %s lost the tool call: %s", inType, outType, out)
			}
			if !strings.Contains(string(out), "Boston") {
				t.Fatalf("response %s -> %s lost the tool arguments: %s", inType, outType, out)
			}
		}
	}
}

func eventData(data string) sse.Event { return sse.Event{Data: []byte(data)} }
