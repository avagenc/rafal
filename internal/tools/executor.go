package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Executor handles the actual execution of tool calls by dispatching
// HTTP requests to the Calendar API server or returning mock data.
type Executor struct {
	calendarBaseURL string
	contactBaseURL  string
	refreshToken    string
	ownerID         string
	httpClient      *http.Client
}

// NewExecutor creates a new tool executor with the given calendar server
// base URL, contact base URL, user refresh token, and owner ID.
func NewExecutor(calendarBaseURL, contactBaseURL, refreshToken, ownerID string) *Executor {
	return &Executor{
		calendarBaseURL: calendarBaseURL,
		contactBaseURL:  contactBaseURL,
		refreshToken:    refreshToken,
		ownerID:         ownerID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Execute dispatches a tool call by function name and arguments,
// returning the result as a map suitable for genai.FunctionResponse.
func (e *Executor) Execute(ctx context.Context, funcName string, args map[string]any) (map[string]any, error) {
	switch funcName {
	case ToolNameGetEvent:
		return e.executeGetEvent(ctx, args)
	case ToolNameAddEvent:
		return e.executeAddEvent(ctx, args)
	case ToolNameGetUserContact:
		return e.executeGetUserContact(ctx, args)
	case ToolNameAddUserContact:
		return e.executeAddUserContact(ctx, args)
	default:
		return nil, fmt.Errorf("executor: unknown tool function: %s", funcName)
	}
}

// executeGetEvent calls the Calendar API GET endpoint.
func (e *Executor) executeGetEvent(ctx context.Context, args map[string]any) (map[string]any, error) {
	body := map[string]any{
		"refreshToken": e.refreshToken,
		"calendarId":   getStringArg(args, "calendar_id", "primary"),
		"query":        getStringArg(args, "query", ""),
		"dateStart":    getStringArg(args, "date_start", ""),
		"dateEnd":      getStringArg(args, "date_end", ""),
	}

	result, err := e.postJSON(ctx, "/calendar/get", body)
	if err != nil {
		return nil, err
	}

	return map[string]any{"events": result}, nil
}

// executeAddEvent calls the Calendar API ADD endpoint.
func (e *Executor) executeAddEvent(ctx context.Context, args map[string]any) (map[string]any, error) {
	body := map[string]any{
		"refreshToken": e.refreshToken,
		"eventTitle":   getStringArg(args, "event_title", ""),
		"timeStart":    getStringArg(args, "time_start", ""),
		"timeEnd":      getStringArg(args, "time_end", ""),
		"calendarId":   getStringArg(args, "calendar_id", "primary"),
		"description":  getStringArg(args, "description", ""),
		"location":     getStringArg(args, "location", ""),
		"guest":        getStringArg(args, "guest", ""),
	}

	result, err := e.postJSON(ctx, "/calendar/add", body)
	if err != nil {
		return nil, err
	}

	return map[string]any{"result": result}, nil
}

// executeGetUserContact calls the Contact API GET endpoint.
// Retrieves all contacts for the owner; the agent filters by name from the result.
func (e *Executor) executeGetUserContact(ctx context.Context, args map[string]any) (map[string]any, error) {
	name := getStringArg(args, "name", "")
	if name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}

	body := map[string]any{
		"owner_id": e.ownerID,
	}

	url := e.contactBaseURL + "/get"
	result, err := e.executeJSONRequest(ctx, http.MethodGet, url, body)
	if err != nil {
		return nil, err
	}

	return map[string]any{"result": result}, nil
}

// executeAddUserContact calls the Contact API CREATE endpoint.
func (e *Executor) executeAddUserContact(ctx context.Context, args map[string]any) (map[string]any, error) {
	name := getStringArg(args, "name", "")
	gmail := getStringArg(args, "gmail", "")
	phone := getStringArg(args, "phone", "")

	if name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}
	if gmail == "" && phone == "" {
		return nil, fmt.Errorf("either gmail or phone must be provided")
	}

	body := map[string]any{
		"owner_id": e.ownerID,
		"name":     name,
		"gmail":    gmail,
		"number":   phone,
	}

	url := e.contactBaseURL + "/create"
	result, err := e.executeJSONRequest(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	return map[string]any{"result": result}, nil
}

// postJSON is a wrapper for executeJSONRequest with POST.
func (e *Executor) postJSON(ctx context.Context, path string, body map[string]any) (any, error) {
	url := e.calendarBaseURL + path
	return e.executeJSONRequest(ctx, http.MethodPost, url, body)
}

// executeJSONRequest safely creates a payload and dispatch the request to target endpoints.
func (e *Executor) executeJSONRequest(ctx context.Context, method string, url string, body map[string]any) (any, error) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("executor: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executor: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("executor: failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("executor: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		// If the response isn't valid JSON, return it as a string
		return string(respBody), nil
	}

	return result, nil
}

// getStringArg safely extracts a string argument from the model's args map.
// Returns defaultVal if the key doesn't exist or the value is not a string.
// Note: empty string is NOT substituted with defaultVal — only missing/non-string keys are.
func getStringArg(args map[string]any, key, defaultVal string) string {
	val, ok := args[key]
	if !ok {
		return defaultVal
	}
	str, ok := val.(string)
	if !ok {
		return defaultVal
	}
	return str
}
