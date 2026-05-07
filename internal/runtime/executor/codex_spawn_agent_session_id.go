package executor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var codexSpawnAgentSessionIDSchemaJSON = []byte(`{"type":["string","null"],"default":null,"description":"Existing subagent session id to continue. For creating a new subagent, omit this field or set it to JSON null. Never use empty string, /null, <null>, string null, or whitespace."}`)

type codexSpawnAgentSessionIDState struct {
	spawnAgentItemIDs map[string]struct{}
}

func normalizeCodexSpawnAgentSessionIDToolSchema(rawJSON []byte) []byte {
	tools := gjson.GetBytes(rawJSON, "tools")
	if !tools.IsArray() {
		return rawJSON
	}

	result := rawJSON
	for i, tool := range tools.Array() {
		if tool.Get("type").String() != "function" || tool.Get("name").String() != "spawn_agent" {
			continue
		}

		schemaPath := fmt.Sprintf("tools.%d.parameters.properties.session_id", i)
		if !gjson.GetBytes(result, schemaPath).Exists() {
			continue
		}
		if updated, err := sjson.SetRawBytes(result, schemaPath, codexSpawnAgentSessionIDSchemaJSON); err == nil {
			result = updated
		}

		requiredPath := fmt.Sprintf("tools.%d.parameters.required", i)
		result = removeCodexRequiredToolField(result, requiredPath, "session_id")
	}
	return result
}

func removeCodexRequiredToolField(rawJSON []byte, path string, field string) []byte {
	required := gjson.GetBytes(rawJSON, path)
	if !required.IsArray() {
		return rawJSON
	}

	changed := false
	filtered := []byte(`[]`)
	for _, item := range required.Array() {
		value := item.String()
		if value == field {
			changed = true
			continue
		}
		if updated, err := sjson.SetBytes(filtered, "-1", value); err == nil {
			filtered = updated
		}
	}
	if !changed {
		return rawJSON
	}

	updated, err := sjson.SetRawBytes(rawJSON, path, filtered)
	if err != nil {
		return rawJSON
	}
	return updated
}

func normalizeCodexSpawnAgentSessionIDInEvent(rawJSON []byte, state *codexSpawnAgentSessionIDState) []byte {
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
		recordCodexSpawnAgentItemID(state, itemResult.Get("id").String())
		return normalizeCodexSpawnAgentArgumentsAtPath(rawJSON, "item.arguments")

	case "response.function_call_arguments.done":
		if state == nil || !state.isSpawnAgentItem(rootResult.Get("item_id").String()) {
			return rawJSON
		}
		return normalizeCodexSpawnAgentArgumentsAtPath(rawJSON, "arguments")

	case "response.completed":
		outputResult := rootResult.Get("response.output")
		if !outputResult.IsArray() {
			return rawJSON
		}
		for i, value := range outputResult.Array() {
			if value.Get("type").String() == "function_call" && value.Get("name").String() == "spawn_agent" {
				rawJSON = normalizeCodexSpawnAgentArgumentsAtPath(rawJSON, "response.output."+strconv.Itoa(i)+".arguments")
			}
		}
		return rawJSON

	default:
		return rawJSON
	}
}

func recordCodexSpawnAgentItemID(state *codexSpawnAgentSessionIDState, itemID string) {
	if state == nil || itemID == "" {
		return
	}
	if state.spawnAgentItemIDs == nil {
		state.spawnAgentItemIDs = make(map[string]struct{})
	}
	state.spawnAgentItemIDs[itemID] = struct{}{}
}

func (s *codexSpawnAgentSessionIDState) isSpawnAgentItem(itemID string) bool {
	if s == nil || itemID == "" {
		return false
	}
	_, ok := s.spawnAgentItemIDs[itemID]
	return ok
}

func normalizeCodexSpawnAgentArgumentsAtPath(rawJSON []byte, path string) []byte {
	argumentsResult := gjson.GetBytes(rawJSON, path)
	if !argumentsResult.Exists() || argumentsResult.Type != gjson.String {
		return rawJSON
	}

	arguments, changed := normalizeCodexSpawnAgentArgumentsString(argumentsResult.String())
	if !changed {
		return rawJSON
	}

	updated, err := sjson.SetBytes(rawJSON, path, arguments)
	if err != nil {
		return rawJSON
	}
	return updated
}

func normalizeCodexSpawnAgentArgumentsString(arguments string) (string, bool) {
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
