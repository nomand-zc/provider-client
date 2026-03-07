package providers

type Tool interface {
	// Declaration returns the metadata describing the tool.
	Declaration() *Declaration
}

type Declaration struct {
	// Name is the unique identifier of the tool
	Name string `json:"name"`

	// Description explains the tool's purpose and functionality
	Description string `json:"description"`

	// InputSchema defines the expected input for the tool in JSON schema format.
	InputSchema *Schema `json:"inputSchema"`

	// OutputSchema defines the expected output for the tool in JSON schema format.
	OutputSchema *Schema `json:"outputSchema,omitempty"`
}

type Schema struct {
	//  Type Specifies the data type (e.g., "object", "array", "string", "number")
	Type        string   `json:"type,omitempty"`
	Description string   `json:"description,omitempty"`
	Required    []string `json:"required,omitempty"`
	// Properties of the arguments, each with its own schema
	Properties map[string]*Schema `json:"properties,omitempty"`
	// For array types, defines the schema of items in the array
	Items *Schema `json:"items,omitempty"`
	// // AdditionalProperties: Controls whether properties not defined in Properties are allowed
	// AdditionalProperties any `json:"additionalProperties,omitempty"`
	// // Default value for the parameter
	// Default any `json:"default,omitempty"`
	// // Enum contains the list of allowed values for the parameter
	// Enum []any `json:"enum,omitempty"`
}
