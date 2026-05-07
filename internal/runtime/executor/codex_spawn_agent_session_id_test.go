package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeCodexSpawnAgentSessionIDToolSchema(t *testing.T) {
	inputJSON := []byte(`{
		"model": "gpt-5.5",
		"tools": [
			{
				"type": "function",
				"name": "diagnostics",
				"parameters": {"type":"object","properties":{}}
			},
			{
				"type": "function",
				"name": "spawn_agent",
				"parameters": {
					"type": "object",
					"required": ["label", "message", "session_id"],
					"properties": {
						"label": {"type": "string"},
						"message": {"type": "string"},
						"session_id": {
							"type": "string",
							"default": null,
							"nullable": true,
							"description": "Session ID of an existing agent session."
						}
					}
				}
			}
		]
	}`)

	output := normalizeCodexSpawnAgentSessionIDToolSchema(inputJSON)

	if got := gjson.GetBytes(output, "tools.1.parameters.properties.session_id.type.0").String(); got != "string" {
		t.Fatalf("session_id type[0] = %q, want string: %s", got, string(output))
	}
	if got := gjson.GetBytes(output, "tools.1.parameters.properties.session_id.type.1").String(); got != "null" {
		t.Fatalf("session_id type[1] = %q, want null: %s", got, string(output))
	}
	if gjson.GetBytes(output, "tools.1.parameters.properties.session_id.nullable").Exists() {
		t.Fatalf("session_id nullable should be removed: %s", string(output))
	}
	for _, required := range gjson.GetBytes(output, "tools.1.parameters.required").Array() {
		if required.String() == "session_id" {
			t.Fatalf("session_id should not be required: %s", string(output))
		}
	}
}

func TestNormalizeCodexSpawnAgentSessionIDInOutputItemDone(t *testing.T) {
	raw := []byte(`{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"/null\"}"}}`)

	out := normalizeCodexSpawnAgentSessionIDInEvent(raw, nil)

	args := gjson.GetBytes(out, "item.arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s payload=%s", got.Type, args, string(out))
	}
}

func TestNormalizeCodexSpawnAgentSessionIDInArgumentsDone(t *testing.T) {
	var state codexSpawnAgentSessionIDState
	added := []byte(`{"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":""}}`)
	done := []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"<null>\"}"}`)

	_ = normalizeCodexSpawnAgentSessionIDInEvent(added, &state)
	out := normalizeCodexSpawnAgentSessionIDInEvent(done, &state)

	args := gjson.GetBytes(out, "arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s payload=%s", got.Type, args, string(out))
	}
}

func TestNormalizeCodexSpawnAgentSessionIDInCompletedResponse(t *testing.T) {
	raw := []byte(`{"type":"response.completed","response":{"id":"resp_1","output":[{"id":"fc_1","type":"function_call","name":"spawn_agent","call_id":"call_1","arguments":"{\"label\":\"Investigate\",\"message\":\"check it\",\"session_id\":\"\"}"}]}}`)

	out := normalizeCodexSpawnAgentSessionIDInEvent(raw, nil)

	args := gjson.GetBytes(out, "response.output.0.arguments").String()
	if got := gjson.Get(args, "session_id"); got.Type != gjson.Null {
		t.Fatalf("session_id type = %v, want null; args=%s payload=%s", got.Type, args, string(out))
	}
}
