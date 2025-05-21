package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	baseURL = "https://generativelanguage.googleapis.com/v1beta"
)

func main() {
	if len(os.Args) < 2 {
		printTopLevelHelp()
		os.Exit(1)
	}

	// Common flags for generate command
	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
	modelName := generateCmd.String("model", "", "Model name (e.g., models/gemini-1.5-flash-latest)")
	systemInstructionStr := generateCmd.String("system-instruction", "", "System instruction text (default: \"\")")

	// GenerationConfig flags
	temperature := generateCmd.Float64("temperature", -1.0, "Temperature for generation (e.g., 0.7). API default if not set or < 0.")
	maxOutputTokens := generateCmd.Int("max-output-tokens", -1, "Max output tokens. API default if not set or < 0.")
	topP := generateCmd.Float64("top-p", -1.0, "Top-P sampling. API default if not set or < 0.")
	topK := generateCmd.Int("top-k", -1, "Top-K sampling. API default if not set or < 0.")
	stopSequence := generateCmd.String("stop-sequence", "", "A single stop sequence string (default: \"\")")
	responseMimeType := generateCmd.String("response-mime-type", "", "Response MIME type (e.g., application/json) (default: \"\")")
	responseSchemaFileOrJSON := generateCmd.String("response-schema", "", "OpenAPI subset schema as JSON string or @/path/to/schema.json (default: \"\")")

	// ThinkingConfig flags
	thinkingBudget := generateCmd.Int("thinking-budget", -1, "Thinking budget for 2.5 models (0-24576). API default/behavior if not set or < 0.")
	includeThoughts := generateCmd.Bool("include-thoughts", false, "Include thought summaries (experimental for 2.5 models) (default: false)")

	// Tools flags
	toolURLContext := generateCmd.Bool("tool-url-context", false, "Enable URL context tool (default: false)")
	toolGoogleSearch := generateCmd.Bool("tool-google-search", false, "Enable Google Search tool (default: false)")
	toolGoogleSearchRetrieval := generateCmd.Bool("tool-google-search-retrieval", false, "Enable Google Search Retrieval tool (for 1.5 models) (default: false)")
	toolGoogleSearchRetrievalMode := generateCmd.String("tool-gsr-mode", "", "Mode for Google Search Retrieval (e.g., MODE_DYNAMIC). Used if --tool-google-search-retrieval is true. (default: \"\")")
	toolGoogleSearchRetrievalThreshold := generateCmd.Float64("tool-gsr-threshold", -1.0, "Threshold for dynamic Google Search Retrieval. Used if --tool-google-search-retrieval is true and mode is dynamic. API default if < 0. (default: -1.0)")

	// Safety Settings flag
	safetySettingsStr := generateCmd.String("safety-settings", "", "Comma-separated safety settings, e.g., \"HARM_CATEGORY_HARASSMENT:BLOCK_ONLY_HIGH,HARM_CATEGORY_HATE_SPEECH:BLOCK_MEDIUM_AND_ABOVE\" (default: \"\")")

	// Set-config command
	setConfigCmd := flag.NewFlagSet("set-config", flag.ExitOnError)
	apiKey := setConfigCmd.String("key", "", "Gemini API Key")

	// List-models command
	listModelsCmd := flag.NewFlagSet("list-models", flag.ExitOnError)

	generateCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s generate --model <model_name> [options] [part_type_1 part_value_1 ...]\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "\nOptions:")
		generateCmd.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nPart Types and Values:")
		fmt.Fprintln(os.Stderr, "  text \"your text string\"")
		fmt.Fprintln(os.Stderr, "  file \"@/path/to/local/file\"")
		fmt.Fprintln(os.Stderr, "  file \"http(s)://url/to/file\"")
		fmt.Fprintln(os.Stderr, "  file \"file:///path/to/local/file\"")
		fmt.Fprintln(os.Stderr, "  file \"data:mime/type;base64,ABC...\"")
		fmt.Fprintln(os.Stderr, "\nExample:")
		fmt.Fprintf(os.Stderr, "  %s generate --model models/gemini-1.5-flash-latest text \"Describe this image\" file \"@/path/to/image.jpg\"\n", os.Args[0])
	}

	switch os.Args[1] {
	case "set-config":
		setConfigCmd.Parse(os.Args[2:])
		if *apiKey == "" {
			fmt.Fprintln(os.Stderr, "Error: --key is required for set-config")
			setConfigCmd.Usage()
			os.Exit(1)
		}
		handleSetConfig(*apiKey)
	case "generate":
		generateCmd.Parse(os.Args[2:])
		if *modelName == "" {
			fmt.Fprintln(os.Stderr, "Error: --model is required for generate")
			generateCmd.Usage()
			os.Exit(1)
		}

		var genConfigInput GenerationConfigInput
		genConfigInput.Temperature = *temperature
		genConfigInput.MaxOutputTokens = *maxOutputTokens
		genConfigInput.TopP = *topP
		genConfigInput.TopK = *topK
		genConfigInput.StopSequence = *stopSequence
		genConfigInput.ResponseMimeType = *responseMimeType
		genConfigInput.ResponseSchemaFileOrJSON = *responseSchemaFileOrJSON
		genConfigInput.ThinkingBudget = *thinkingBudget
		genConfigInput.IncludeThoughts = *includeThoughts

		var toolsInput ToolsInput
		toolsInput.EnableURLContext = *toolURLContext
		toolsInput.EnableGoogleSearch = *toolGoogleSearch
		toolsInput.EnableGoogleSearchRetrieval = *toolGoogleSearchRetrieval
		toolsInput.GoogleSearchRetrievalMode = *toolGoogleSearchRetrievalMode
		toolsInput.GoogleSearchRetrievalThreshold = *toolGoogleSearchRetrievalThreshold

		parsedParts, err := parseInputParts(generateCmd.Args())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing input parts: %v\n", err)
			generateCmd.Usage()
			os.Exit(1)
		}
		if len(parsedParts) == 0 && *systemInstructionStr == "" {
			fmt.Fprintln(os.Stderr, "Error: At least one input part (text/file) or system-instruction is required for generate.")
			generateCmd.Usage()
			os.Exit(1)
		}

		currentApiKey, err := loadAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading API key: %v. Please run 'set-config --key YOUR_KEY'.\n", err)
			os.Exit(1)
		}

		handleGenerateContent(currentApiKey, *modelName, *systemInstructionStr, parsedParts, genConfigInput, toolsInput, *safetySettingsStr)

	case "list-models":
		listModelsCmd.Parse(os.Args[2:])
		currentApiKey, err := loadAPIKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading API key: %v. Please run 'set-config --key YOUR_KEY'.\n", err)
			os.Exit(1)
		}
		handleListModels(currentApiKey)
	default:
		printTopLevelHelp()
		os.Exit(1)
	}
}

func printTopLevelHelp() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  set-config        Set the Gemini API key")
	fmt.Fprintln(os.Stderr, "  generate          Generate content using a Gemini model")
	fmt.Fprintln(os.Stderr, "  list-models       List available Gemini models")
	fmt.Fprintf(os.Stderr, "Run '%s <command> --help' for more information on a command.\n", os.Args[0])
}

type ParsedPart struct {
	Type  string // "text" or "file"
	Value string
}

func parseInputParts(args []string) ([]ParsedPart, error) {
	var parts []ParsedPart
	if len(args)%2 != 0 {
		return nil, fmt.Errorf("input parts must be in pairs of type and value (e.g., text \"hello\")")
	}
	for i := 0; i < len(args); i += 2 {
		partType := args[i]
		partValue := args[i+1]
		if partType != "text" && partType != "file" {
			return nil, fmt.Errorf("invalid part type: %s. Must be 'text' or 'file'", partType)
		}
		parts = append(parts, ParsedPart{Type: partType, Value: partValue})
	}
	return parts, nil
}

// Helper struct to pass parsed CLI flags for GenerationConfig
type GenerationConfigInput struct {
	Temperature              float64
	MaxOutputTokens          int
	TopP                     float64
	TopK                     int
	StopSequence             string
	ResponseMimeType         string
	ResponseSchemaFileOrJSON string
	ThinkingBudget           int
	IncludeThoughts          bool
}

// Helper struct to pass parsed CLI flags for Tools
type ToolsInput struct {
	EnableURLContext               bool
	EnableGoogleSearch             bool
	EnableGoogleSearchRetrieval    bool
	GoogleSearchRetrievalMode      string
	GoogleSearchRetrievalThreshold float64
}
