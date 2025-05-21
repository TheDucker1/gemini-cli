package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Request Structures
type TextPart struct {
	Text string `json:"text"`
}

type InlinePart struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"` // base64 encoded
}

type Part struct {
	Text       *string     `json:"text,omitempty"`
	InlineData *InlinePart `json:"inline_data,omitempty"`
}

type Content struct {
	Role  string `json:"role,omitempty"` // For future multi-turn chat if input format changes
	Parts []Part `json:"parts"`
}

type SystemInstruction struct {
	Parts []TextPart `json:"parts"`
}

// --- New/Updated Structs for Features ---
type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type DynamicRetrievalConfig struct {
	Mode             string   `json:"mode,omitempty"`
	DynamicThreshold *float64 `json:"dynamic_threshold,omitempty"`
}
type GoogleSearchRetrievalConfig struct {
	DynamicRetrievalConfig *DynamicRetrievalConfig `json:"dynamic_retrieval_config,omitempty"`
}

type Tool struct {
	URLContext            *map[string]interface{}      `json:"url_context,omitempty"`   // Should be an empty object {}
	GoogleSearch          *map[string]interface{}      `json:"google_search,omitempty"` // Should be an empty object {}
	GoogleSearchRetrieval *GoogleSearchRetrievalConfig `json:"google_search_retrieval,omitempty"`
}

type ThinkingConfig struct {
	ThinkingBudget  *int  `json:"thinkingBudget,omitempty"`
	IncludeThoughts *bool `json:"includeThoughts,omitempty"`
}

type GenerationConfig struct {
	StopSequences    []string        `json:"stopSequences,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxOutputTokens  *int            `json:"maxOutputTokens,omitempty"`
	TopP             *float64        `json:"topP,omitempty"`
	TopK             *int            `json:"topK,omitempty"`
	ResponseMimeType *string         `json:"responseMimeType,omitempty"`
	ResponseSchema   json.RawMessage `json:"responseSchema,omitempty"` // OpenAPI subset
	ThinkingConfig   *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GenerateContentRequest struct {
	SystemInstruction *SystemInstruction `json:"system_instruction,omitempty"`
	Contents          []Content          `json:"contents"`
	Tools             []Tool             `json:"tools,omitempty"`
	SafetySettings    []SafetySetting    `json:"safetySettings,omitempty"`
	GenerationConfig  *GenerationConfig  `json:"generationConfig,omitempty"`
}

// Response Structures
type ModelInfo struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	Temperature                *float64 `json:"temperature,omitempty"` // Pointer to allow null
	TopP                       *float64 `json:"topP,omitempty"`        // Pointer to allow null
	TopK                       *int     `json:"topK,omitempty"`        // Pointer to allow null
}

type ListModelsResponse struct {
	Models []ModelInfo `json:"models"`
}

func makeAPIRequest(apiKey, method, endpointURL string, body io.Reader, target interface{}) error {
	client := &http.Client{}
	fullURL := fmt.Sprintf("%s%s?key=%s", baseURL, endpointURL, apiKey)

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s, Body: %s", resp.Status, string(responseBody))
	}

	if target != nil {
		if err := json.Unmarshal(responseBody, target); err != nil {
			return fmt.Errorf("failed to unmarshal response into target: %w. Raw response: %s", err, string(responseBody))
		}
	} else {
		// Output raw JSON response body if no target for unmarshalling
		fmt.Println(string(responseBody))
	}
	return nil
}

func buildGenerateContentRequest(
	systemInstructionStr string,
	parsedParts []ParsedPart,
	genConfigInput GenerationConfigInput,
	toolsInput ToolsInput,
	safetySettingsStr string) (*GenerateContentRequest, error) {

	req := &GenerateContentRequest{}
	var genCfg GenerationConfig
	genCfgChanged := false

	if systemInstructionStr != "" {
		req.SystemInstruction = &SystemInstruction{
			Parts: []TextPart{{Text: systemInstructionStr}},
		}
	}

	if len(parsedParts) > 0 {
		var apiParts []Part
		for _, p := range parsedParts {
			switch p.Type {
			case "text":
				textVal := p.Value
				apiParts = append(apiParts, Part{Text: &textVal})
			case "file":
				mimeType, data, err := processFileArgument(p.Value)
				if err != nil {
					return nil, fmt.Errorf("failed to process file argument '%s': %w", p.Value, err)
				}
				apiParts = append(apiParts, Part{InlineData: &InlinePart{MIMEType: mimeType, Data: data}})
			default:
				return nil, fmt.Errorf("unknown parsed part type: %s", p.Type)
			}
		}
		req.Contents = []Content{{Parts: apiParts}}
	} else if systemInstructionStr == "" {
		return nil, fmt.Errorf("at least one input part or system instruction is required")
	}

	// --- Populate GenerationConfig ---
	if genConfigInput.Temperature >= 0 {
		t := genConfigInput.Temperature
		genCfg.Temperature = &t
		genCfgChanged = true
	}
	if genConfigInput.MaxOutputTokens >= 0 {
		m := genConfigInput.MaxOutputTokens
		genCfg.MaxOutputTokens = &m
		genCfgChanged = true
	}
	if genConfigInput.TopP >= 0 {
		p := genConfigInput.TopP
		genCfg.TopP = &p
		genCfgChanged = true
	}
	if genConfigInput.TopK >= 0 {
		k := genConfigInput.TopK
		genCfg.TopK = &k
		genCfgChanged = true
	}
	if genConfigInput.StopSequence != "" {
		genCfg.StopSequences = []string{genConfigInput.StopSequence}
		genCfgChanged = true
	}
	if genConfigInput.ResponseMimeType != "" {
		rmt := genConfigInput.ResponseMimeType
		genCfg.ResponseMimeType = &rmt
		genCfgChanged = true
	}
	if genConfigInput.ResponseSchemaFileOrJSON != "" {
		schemaContent, err := readFileOrString(genConfigInput.ResponseSchemaFileOrJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to read response-schema: %w", err)
		}
		genCfg.ResponseSchema = json.RawMessage(schemaContent)
		genCfgChanged = true
	}

	// Thinking Config
	var thinkingCfg ThinkingConfig
	thinkingCfgChanged := false
	if genConfigInput.ThinkingBudget >= 0 {
		tb := genConfigInput.ThinkingBudget
		thinkingCfg.ThinkingBudget = &tb
		thinkingCfgChanged = true
	}
	if genConfigInput.IncludeThoughts { // Default is false, so only set if true
		it := genConfigInput.IncludeThoughts
		thinkingCfg.IncludeThoughts = &it
		thinkingCfgChanged = true
	}
	if thinkingCfgChanged {
		genCfg.ThinkingConfig = &thinkingCfg
		genCfgChanged = true
	}

	if genCfgChanged {
		req.GenerationConfig = &genCfg
	}

	// --- Populate Tools ---
	var tools []Tool
	if toolsInput.EnableURLContext {
		tools = append(tools, Tool{URLContext: &map[string]interface{}{}})
	}
	if toolsInput.EnableGoogleSearch {
		tools = append(tools, Tool{GoogleSearch: &map[string]interface{}{}})
	}
	if toolsInput.EnableGoogleSearchRetrieval {
		gsrConfig := GoogleSearchRetrievalConfig{}
		gsrConfigChanged := false
		if toolsInput.GoogleSearchRetrievalMode != "" || toolsInput.GoogleSearchRetrievalThreshold >= 0 {
			dynConfig := DynamicRetrievalConfig{}
			dynConfigChanged := false
			if toolsInput.GoogleSearchRetrievalMode != "" {
				dynConfig.Mode = toolsInput.GoogleSearchRetrievalMode
				dynConfigChanged = true
			}
			if toolsInput.GoogleSearchRetrievalThreshold >= 0 {
				threshold := toolsInput.GoogleSearchRetrievalThreshold
				dynConfig.DynamicThreshold = &threshold
				dynConfigChanged = true
			}
			if dynConfigChanged {
				gsrConfig.DynamicRetrievalConfig = &dynConfig
				gsrConfigChanged = true
			}
		}
		if gsrConfigChanged {
			tools = append(tools, Tool{GoogleSearchRetrieval: &gsrConfig})
		} else { // Add empty GoogleSearchRetrieval if flag is true but no sub-options
			tools = append(tools, Tool{GoogleSearchRetrieval: &GoogleSearchRetrievalConfig{}})
		}
	}
	if len(tools) > 0 {
		req.Tools = tools
	}

	// --- Populate Safety Settings ---
	if safetySettingsStr != "" {
		pairs := strings.Split(safetySettingsStr, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
			if len(parts) == 2 {
				category := strings.TrimSpace(parts[0])
				threshold := strings.TrimSpace(parts[1])
				if category != "" && threshold != "" {
					req.SafetySettings = append(req.SafetySettings, SafetySetting{Category: category, Threshold: threshold})
				} else {
					return nil, fmt.Errorf("invalid safety setting pair: '%s'. Must be CATEGORY:THRESHOLD", pair)
				}
			} else if strings.TrimSpace(pair) != "" { // Allow if only one non-empty pair and it's invalid
				return nil, fmt.Errorf("invalid safety setting format: '%s'. Must be CATEGORY:THRESHOLD", pair)
			}
		}
	}

	return req, nil
}
