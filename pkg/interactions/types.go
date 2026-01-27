// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interactions

import (
	"encoding/json"
	"time"
)

// Role defines the sender of a message part.
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
)

// Part represents a single segment of an interaction input or output.
type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *Blob       `json:"inline_data,omitempty"`
	FileData   *File       `json:"file_data,omitempty"`
	Thought    bool        `json:"thought,omitempty"`
	Call       *ToolCall   `json:"tool_call,omitempty"`
	Response   *ToolResult `json:"tool_response,omitempty"`
}

// Blob represents inline binary data.
type Blob struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // Base64 encoded
}

// File represents a reference to a stored file.
type File struct {
	MimeType string `json:"mime_type"`
	FileURI  string `json:"file_uri"`
}

// ToolCall represents a request from the model to call a function.
type ToolCall struct {
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

// FunctionCall represents the actual function name and arguments.
type FunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// ToolResult represents the result of a function call.
type ToolResult struct {
	FunctionResponse *FunctionResponse `json:"function_response,omitempty"`
}

// FunctionResponse represents the output data from a function.
type FunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

// Content represents a single turn in an interaction, containing the role and the actual data (parts or text).
type Content struct {
	Role  Role   `json:"role,omitempty"`
	Parts []Part `json:"parts,omitempty"`
	Text  string `json:"text,omitempty"` // Interactions API often flattens text here
}

// InteractionRequest defines the payload for creating a new interaction.
// It supports both standard model input and specialized agent execution.
type InteractionRequest struct {
	Model                 string            `json:"model,omitempty"`
	Agent                 string            `json:"agent,omitempty"`
	Input                 any               `json:"input,omitempty"` // Can be string or []Content
	PreviousInteractionID string            `json:"previous_interaction_id,omitempty"`
	Store                 *bool             `json:"store,omitempty"`
	Background            bool              `json:"background,omitempty"`
	ResponseModalities    []string          `json:"response_modalities,omitempty"`
	GenerationConfig      *GenerationConfig `json:"generation_config,omitempty"`
	Tools                 []Tool            `json:"tools,omitempty"`
}

// GenerationConfig defines model sampling and output parameters.
type GenerationConfig struct {
	Temperature     *float32     `json:"temperature,omitempty"`
	TopP            *float32     `json:"top_p,omitempty"`
	TopK            *int         `json:"top_k,omitempty"`
	MaxOutputTokens *int         `json:"max_output_tokens,omitempty"`
	StopSequences   []string     `json:"stop_sequences,omitempty"`
	ResponseMimeType string      `json:"response_mime_type,omitempty"`
	ImageConfig     *ImageConfig `json:"image_config,omitempty"`
}

// ImageConfig defines parameters for image generation.
type ImageConfig struct {
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImageSize   string `json:"image_size,omitempty"`
}

// Tool represents an external capability the model can use.
type Tool struct {
	FunctionDeclarations []FunctionDeclaration `json:"function_declarations,omitempty"`
}

// FunctionDeclaration defines a tool that the model can call.
type FunctionDeclaration struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"` // JSON Schema
}

// InteractionResponse defines the result of an interaction.
type InteractionResponse struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Status                string    `json:"status"` // e.g., "COMPLETED", "WORKING"
	Outputs               []Content `json:"outputs,omitempty"`
	Error                 *Error    `json:"error,omitempty"`
	PreviousInteractionID string    `json:"previous_interaction_id,omitempty"`
	CreateTime            time.Time `json:"createTime,omitempty"`
	UpdateTime            time.Time `json:"updateTime,omitempty"`
}

// Error represents an API error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// UnmarshalJSON handles the dynamic 'input' field which can be string or array.
func (r *InteractionRequest) MarshalJSON() ([]byte, error) {
	type Alias InteractionRequest
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}