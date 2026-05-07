package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertCodexResponseToOpenAIResponses converts OpenAI Chat Completions streaming chunks
// to OpenAI Responses SSE events (response.*).

func ConvertCodexResponseToOpenAIResponses(_ context.Context, _ string, _, _, rawJSON []byte, param *any) [][]byte {
	if bytes.HasPrefix(rawJSON, []byte("data:")) {
		rawJSON = bytes.TrimSpace(rawJSON[5:])
		rawJSON = normalizeSpawnAgentSessionIDInCodexEvent(rawJSON, param)
		out := make([]byte, 0, len(rawJSON)+len("data: "))
		out = append(out, []byte("data: ")...)
		out = append(out, rawJSON...)
		return [][]byte{out}
	}
	return [][]byte{normalizeSpawnAgentSessionIDInCodexEvent(rawJSON, param)}
}

// ConvertCodexResponseToOpenAIResponsesNonStream builds a single Responses JSON
// from a non-streaming OpenAI Chat Completions response.
func ConvertCodexResponseToOpenAIResponsesNonStream(_ context.Context, _ string, _, _, rawJSON []byte, _ *any) []byte {
	rawJSON = normalizeSpawnAgentSessionIDInCodexEvent(rawJSON, nil)
	rootResult := gjson.ParseBytes(rawJSON)
	// Verify this is a response.completed event
	if rootResult.Get("type").String() != "response.completed" {
		return []byte{}
	}
	responseResult := rootResult.Get("response")
	return []byte(responseResult.Raw)
}

type codexOpenAIResponsesState struct {
	spawnAgentItemIDs map[string]struct{}
}

func getCodexOpenAIResponsesState(param *any) *codexOpenAIResponsesState {
	if param == nil {
		return nil
	}
	if state, ok := (*param).(*codexOpenAIResponsesState); ok && state != nil {
		return state
	}
	state := &codexOpenAIResponsesState{spawnAgentItemIDs: map[string]struct{}{}}
	*param = state
	return state
}

func normalizeSpawnAgentSessionIDInCodexEvent(rawJSON []byte, param *any) []byte {
	rootResult := gjson.ParseBytes(rawJSON)
	if !rootResult.Exists() {
		return rawJSON
	}

	switch rootResult.Get("type").String() {
	case "response.output_item.added", "response.output_item.done":
		itemResult := rootResult.Get("item")
		if itemResult.Get("type").String() != "function_call" || itemResult.Get("name").String() != "spawn_agent" {
			return rawJSON
		}
		if itemID := itemResult.Get("id").String(); itemID != "" {
			if state := getCodexOpenAIResponsesState(param); state != nil {
				state.spawnAgentItemIDs[itemID] = struct{}{}
			}
		}
		return normalizeSpawnAgentArgumentsAtPath(rawJSON, "item.arguments")

	case "response.function_call_arguments.done":
		itemID := rootResult.Get("item_id").String()
		state := getCodexOpenAIResponsesState(param)
		if state == nil {
			return rawJSON
		}
		if _, ok := state.spawnAgentItemIDs[itemID]; !ok {
			return rawJSON
		}
		return normalizeSpawnAgentArgumentsAtPath(rawJSON, "arguments")

	case "response.completed":
		outputResult := rootResult.Get("response.output")
		if !outputResult.IsArray() {
			return rawJSON
		}
		for i, value := range outputResult.Array() {
			if value.Get("type").String() == "function_call" && value.Get("name").String() == "spawn_agent" {
				rawJSON = normalizeSpawnAgentArgumentsAtPath(rawJSON, "response.output."+strconv.Itoa(i)+".arguments")
			}
		}
		return rawJSON

	default:
		return rawJSON
	}
}

func normalizeSpawnAgentArgumentsAtPath(rawJSON []byte, path string) []byte {
	argumentsResult := gjson.GetBytes(rawJSON, path)
	if !argumentsResult.Exists() || argumentsResult.Type != gjson.String {
		return rawJSON
	}

	arguments, changed := normalizeSpawnAgentArgumentsString(argumentsResult.String())
	if !changed {
		return rawJSON
	}

	updated, err := sjson.SetBytes(rawJSON, path, arguments)
	if err != nil {
		return rawJSON
	}
	return updated
}

func normalizeSpawnAgentArgumentsString(arguments string) (string, bool) {
	var args map[string]any
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return arguments, false
	}

	sessionID, ok := args["session_id"].(string)
	if !ok {
		return arguments, false
	}

	switch strings.ToLower(strings.TrimSpace(sessionID)) {
	case "", "null", "/null", "<null>":
		args["session_id"] = nil
	default:
		return arguments, false
	}

	normalized, err := json.Marshal(args)
	if err != nil {
		return arguments, false
	}
	return string(normalized), true
}
