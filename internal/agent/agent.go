// Package agent provides the core AI agent that uses Google GenAI (Gemini)
// to process calendar commands with function calling.
package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"avagenc-gmail/internal/tools"

	"go.naturallyfunny.dev/api/identity"
	"google.golang.org/genai"
)

// maxToolIterations prevents infinite tool-calling loops.
// The agent will stop after this many round-trips.
const maxToolIterations = 10

// iterationTimeout is the per-GenerateContent-call deadline.
// Bounds a single model round-trip so a hung upstream can't consume the full Lambda budget.
const iterationTimeout = 45 * time.Second

// Result is the structured output of an agent invocation.
type Result struct {
	// Output is the final text message from the agent.
	Output string `json:"output"`

	// UsageTool is a list of tool names that were invoked during this run.
	UsageTool []string `json:"usage_tool"`
}

// RunConfig contains all parameters needed for a single agent run.
type RunConfig struct {
	// GeminiAPIKey is the API key for Google Gemini.
	GeminiAPIKey string

	// Model is the Gemini model name (e.g., "gemini-2.5-flash").
	Model string

	// Message is the user's natural language command to execute.
	Message string

	// RefreshToken is the user's OAuth refresh token for Google Calendar.
	RefreshToken string

	// CalendarBaseURL is the base URL of the Calendar API server.
	CalendarBaseURL string

	// ContactBaseURL is the base URL of the Contact API server.
	ContactBaseURL string
}

// Run executes the Calendar Agent for a single command.
// It creates a GenAI client, sends the command with system instructions,
// and handles the tool-calling loop until the model produces a final text response.
func Run(ctx context.Context, cfg RunConfig) (*Result, error) {
	// Initialize the GenAI client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.GeminiAPIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("agent: failed to create GenAI client: %w", err)
	}

	// Build system instruction
	systemInstruction := buildSystemInstruction()

	// Retrieve user ID from context
	ownerID, err := identity.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("agent: %w", err)
	}

	// Build the initial user message
	userMessage := buildUserMessage(cfg.Message)

	// Prepare the conversation contents
	contents := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: userMessage}},
		},
	}

	// Configure the generation request with tools
	genConfig := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemInstruction}},
		},
		Tools: tools.CalendarTools(),
	}

	// Create the tool executor
	executor := tools.NewExecutor(cfg.CalendarBaseURL, cfg.ContactBaseURL, cfg.RefreshToken, ownerID)

	// Track which tools were used
	var usedTools []string

	// Tool-calling loop: iterate until the model returns a text response
	// or we hit the maximum number of iterations
	for i := 0; i < maxToolIterations; i++ {
		log.Printf("[agent] iteration %d: sending request to model %s", i+1, cfg.Model)

		// Bound each model call independently so a single hung request
		// can't exhaust the full Lambda timeout.
		iterCtx, cancel := context.WithTimeout(ctx, iterationTimeout)
		resp, err := client.Models.GenerateContent(iterCtx, cfg.Model, contents, genConfig)
		cancel()

		if err != nil {
			return nil, fmt.Errorf("agent: GenerateContent failed at iteration %d: %w", i+1, err)
		}

		// Validate response
		if len(resp.Candidates) == 0 {
			return nil, fmt.Errorf("agent: no candidates returned at iteration %d", i+1)
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			return nil, fmt.Errorf("agent: empty content in candidate at iteration %d", i+1)
		}

		// Check if the model wants to call functions
		functionCalls := extractFunctionCalls(candidate.Content.Parts)

		// If no function calls, extract the final text response and return
		if len(functionCalls) == 0 {
			finalText := extractText(candidate.Content.Parts)
			if finalText == "" {
				return nil, fmt.Errorf("agent: model returned neither function calls nor text at iteration %d", i+1)
			}

			log.Printf("[agent] final response received after %d iteration(s)", i+1)
			return &Result{
				Output:    finalText,
				UsageTool: usedTools,
			}, nil
		}

		// Append the model's response (with function calls) to the conversation
		contents = append(contents, candidate.Content)

		// Execute each function call and build function response parts
		var functionResponseParts []*genai.Part

		for _, fc := range functionCalls {
			log.Printf("[agent] executing tool: %s with args: %v", fc.Name, fc.Args)
			usedTools = append(usedTools, fc.Name)

			result, err := executor.Execute(ctx, fc.Name, fc.Args)
			if err != nil {
				log.Printf("[agent] tool execution error for %s: %v", fc.Name, err)
				result = map[string]any{"error": err.Error()}
			}

			// Build function response part
			functionResponseParts = append(functionResponseParts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		// Append function responses to the conversation
		contents = append(contents, &genai.Content{
			Role:  "user",
			Parts: functionResponseParts,
		})
	}

	return nil, fmt.Errorf("agent: exceeded maximum tool iterations (%d)", maxToolIterations)
}

// buildSystemInstruction constructs the system prompt for the Calendar Agent.
func buildSystemInstruction() string {
	currentTime := time.Now().Format(time.RFC3339)

	return fmt.Sprintf(`<role>
You are Calendar Agent, a deterministic and tool-driven assistant responsible exclusively for executing Google Calendar operations via the tools provided to you.
current time: %s
</role>

<instructions>
You operate strictly through the following tools:

1. get_event — Retrieve calendar events with filtering by query, date_start, and/or date_end.
2. add_event — Add a new event to Google Calendar with title, time, and optional details.
3. get_user_contact — Look up a user's contact list. Returns all contacts; filter by name from the result.
4. add_user_contact — Add a new contact entry when the user provides contact info not yet saved.

IMPORTANT RULES:
- For get_event and add_event, always provide ALL parameters. If a parameter is not needed, pass an empty string ("").
- time_start and time_end must always be in RFC3339 format.
- guest must be a semicolon-separated string of emails, NOT a JSON array.
- When searching for events without specific dates, use the current time as date_start.
- Always respond in the same language as the command.
- Be concise and informative in your final response.
- If a tool returns an error, inform the user clearly about what went wrong.
</instructions>`, currentTime)
}

// buildUserMessage wraps the raw message in the expected format.
func buildUserMessage(message string) string {
	return fmt.Sprintf(`<command>
the main agent send command:
%s
</command>`, message)
}

// extractFunctionCalls extracts all FunctionCall parts from the model response.
func extractFunctionCalls(parts []*genai.Part) []*genai.FunctionCall {
	var calls []*genai.FunctionCall
	for _, part := range parts {
		if part.FunctionCall != nil {
			calls = append(calls, part.FunctionCall)
		}
	}
	return calls
}

// extractText concatenates all text parts from the model response.
func extractText(parts []*genai.Part) string {
	var texts []string
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}
