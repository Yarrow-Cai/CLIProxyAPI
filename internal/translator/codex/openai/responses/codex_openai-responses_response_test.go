package responses

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertCodexResponseToOpenAIResponses_NormalizesSpawnAgentSessionIDInDoneEvent(t *testing.T) {
	raw := []byte(`data: {"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"/null\"}"}}`)

	out := ConvertCodexResponseToOpenAIResponses(context.Background(), "gpt-5.5", nil, nil, raw, nil)
	if len(out) != 1 {
		t.Fatalf("chunks = %d, want 1", len(out))
	}

	payload := string(out[0][len("data: "):])
	args := gjson.Get(payload, "item.arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s payload=%s", got.Type, args, payload)
	}
}

func TestConvertCodexResponseToOpenAIResponses_NormalizesSpawnAgentSessionIDInArgumentsDoneEvent(t *testing.T) {
	var state any
	added := []byte(`data: {"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":""}}`)
	done := []byte(`data: {"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"<null>\"}"}`)

	_ = ConvertCodexResponseToOpenAIResponses(context.Background(), "gpt-5.5", nil, nil, added, &state)
	out := ConvertCodexResponseToOpenAIResponses(context.Background(), "gpt-5.5", nil, nil, done, &state)
	if len(out) != 1 {
		t.Fatalf("chunks = %d, want 1", len(out))
	}

	payload := string(out[0][len("data: "):])
	args := gjson.Get(payload, "arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s payload=%s", got.Type, args, payload)
	}
}

func TestConvertCodexResponseToOpenAIResponsesNonStream_NormalizesSpawnAgentSessionID(t *testing.T) {
	raw := []byte(`{"type":"response.completed","response":{"id":"resp_1","output":[{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"\"}"}]}}`)

	out := ConvertCodexResponseToOpenAIResponsesNonStream(context.Background(), "gpt-5.5", nil, nil, raw, nil)

	args := gjson.GetBytes(out, "output.0.arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s output=%s", got.Type, args, string(out))
	}
}
