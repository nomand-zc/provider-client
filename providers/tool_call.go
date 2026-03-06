package providers

import "encoding/json"

// ToolCall represents a call to a tool (function) in the model response.
type ToolCall struct {
	// Type of the tool. Currently, only `function` is supported.
	Type string `json:"type"`
	// Function definition for the tool
	Function FunctionDefinitionParam `json:"function,omitempty"`
	// The ID of the tool call returned by the model.
	ID string `json:"id,omitempty"`

	// Index is the index of the tool call in the message for streaming responses.
	Index *int `json:"index,omitempty"`

	// ExtraFields stores additional provider-specific fields for transparent passthrough.
	// For example, Gemini 3's thought_signature for multi-turn function calling.
	ExtraFields map[string]any `json:"extra_fields,omitempty"`
}

// FunctionDefinitionParam represents the parameters for a function definition in tool calls.
type FunctionDefinitionParam struct {
	// The name of the function to be called. Must be a-z, A-Z, 0-9, or contain
	// underscores and dashes, with a maximum length of 64.
	Name string `json:"name"`
	// Whether to enable strict schema adherence when generating the function call. If
	// set to true, the model will follow the exact schema defined in the `parameters`
	// field. Only a subset of JSON Schema is supported when `strict` is `true`. Learn
	// more about Structured Outputs in the
	// [function calling guide](docs/guides/function-calling).
	Strict bool `json:"strict,omitempty"`
	// A description of what the function does, used by the model to choose when and
	// how to call the function.
	Description string `json:"description,omitempty"`

	// Optional arguments to pass to the function, json-encoded.
	Arguments []byte `json:"arguments,omitempty"`
}

// MarshalJSON customizes JSON marshaling for FunctionDefinitionParam.
// This prevents double-encoding of the Arguments field by treating it as a string.
func (f FunctionDefinitionParam) MarshalJSON() ([]byte, error) {
	type Alias FunctionDefinitionParam
	return json.Marshal(&struct {
		Arguments string `json:"arguments,omitempty"`
		*Alias
	}{
		Arguments: string(f.Arguments),
		Alias:     (*Alias)(&f),
	})
}

// UnmarshalJSON customizes JSON unmarshaling for FunctionDefinitionParam.
// This ensures the Arguments field is properly decoded from JSON string to []byte.
func (f *FunctionDefinitionParam) UnmarshalJSON(data []byte) error {
	type Alias FunctionDefinitionParam
	aux := &struct {
		Arguments string `json:"arguments,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	f.Arguments = []byte(aux.Arguments)
	return nil
}
