package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func handleSetConfig(apiKey string) {
	err := saveAPIKey(apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving API key: %v\n", err)
		os.Exit(1)
	}
}

func handleGenerateContent(
	apiKey,
	modelName,
	systemInstructionStr string,
	parsedParts []ParsedPart,
	genConfigInput GenerationConfigInput,
	toolsInput ToolsInput,
	safetySettingsStr string) {

	if !strings.HasPrefix(modelName, "models/") {
		modelName = "models/" + modelName
	}

	requestPayload, err := buildGenerateContentRequest(systemInstructionStr, parsedParts, genConfigInput, toolsInput, safetySettingsStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building request: %v\n", err)
		os.Exit(1)
	}

	if len(requestPayload.Contents) == 0 && requestPayload.SystemInstruction == nil {
		fmt.Fprintln(os.Stderr, "Error: Request must contain 'contents' or 'system_instruction'.")
		os.Exit(1)
	}

	jsonData, err := json.Marshal(requestPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling request to JSON: %v\n", err)
		os.Exit(1)
	}

	endpoint := fmt.Sprintf("/%s:generateContent", modelName)

	err = makeAPIRequest(apiKey, "POST", endpoint, bytes.NewBuffer(jsonData), nil) // Target is nil to print raw JSON
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error making API request: %v\n", err)
		os.Exit(1)
	}
}

type ModelOutputInfo struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName"`
	Version                    string   `json:"version"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
	SupportedForTextOutput     string   `json:"supportedForTextOutput"` // Added by CLI
}

func handleListModels(apiKey string) {
	var response ListModelsResponse
	// Pass target to unmarshal, makeAPIRequest will not print raw JSON if target is provided
	err := makeAPIRequest(apiKey, "GET", "/models", nil, &response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing models: %v\n", err)
		os.Exit(1)
	}

	var processedModels []ModelOutputInfo
	for _, m := range response.Models {
		isTextSupported := "No"
		supportsGenContent := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" { // A primary indicator for text generation capabilities
				supportsGenContent = true
				break
			}
		}

		// Further refine based on display name patterns for non-text models
		// Models supporting generateContent are generally text-output capable.
		// Specialized models (TTS, Imagen, Veo) might support generateContent in specific ways
		// but their primary output isn't general text in the same vein as chat models.
		if supportsGenContent {
			isTextSupported = "Yes" // Default assumption if generateContent is supported
			dnLower := strings.ToLower(m.DisplayName)
			if strings.Contains(dnLower, "tts") ||
				strings.HasPrefix(dnLower, "imagen") ||
				strings.HasPrefix(dnLower, "veo") ||
				strings.Contains(dnLower, "embedding") {
				isTextSupported = "No (Specialized: Audio/Image/Video/Embedding)"
			}
		}

		processedModels = append(processedModels, ModelOutputInfo{
			Name:                       m.Name,
			DisplayName:                m.DisplayName,
			Version:                    m.Version,
			InputTokenLimit:            m.InputTokenLimit,
			OutputTokenLimit:           m.OutputTokenLimit,
			SupportedGenerationMethods: m.SupportedGenerationMethods,
			SupportedForTextOutput:     isTextSupported,
		})
	}

	outputData, err := json.MarshalIndent(map[string][]ModelOutputInfo{"models": processedModels}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling processed model list: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(outputData))
}
