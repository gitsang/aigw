package translate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gitsang/aigw/pkg/sse"
)

func TestRequestTranslationMatrix(t *testing.T) {
	requests := map[string][]byte{
		APIChat:      []byte(`{"model":"m","messages":[{"role":"system","content":"sys"},{"role":"user","content":"hi"}],"max_tokens":16,"stream":false}`),
		APIResponses: []byte(`{"model":"m","instructions":"sys","input":"hi","max_output_tokens":16}`),
		APIMessages:  []byte(`{"model":"m","system":"sys","messages":[{"role":"user","content":"hi"}],"max_tokens":16}`),
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
		}
	}
}

func TestResponseTranslationMatrix(t *testing.T) {
	responses := map[string]struct {
		body string
	}{
		APIChat:      {`{"id":"chatcmpl-1","model":"up","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`},
		APIResponses: {`{"id":"resp_1","model":"up","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":2,"total_tokens":5}}`},
		APIMessages:  {`{"id":"msg_1","model":"up","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`},
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
	}
	for inType, events := range streams {
		for outType := range streams {
			conv := NewStreamConverter(inType, outType, "virtual-model")
			var outCount int
			for _, ev := range events {
				out, err := conv.Feed(ev)
				if err != nil {
					t.Fatalf("stream %s -> %s: %v", inType, outType, err)
				}
				outCount += len(out)
			}
			if outCount == 0 {
				t.Fatalf("stream %s -> %s produced no events", inType, outType)
			}
			if conv.Text() != "hello" {
				t.Fatalf("stream %s -> %s text=%q", inType, outType, conv.Text())
			}
			if conv.Usage().PromptTokens != 3 || conv.Usage().CompletionTokens != 2 {
				t.Fatalf("stream %s -> %s usage=%+v", inType, outType, conv.Usage())
			}
		}
	}
}

func eventData(data string) sse.Event { return sse.Event{Data: []byte(data)} }
