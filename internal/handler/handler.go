// Package handler provides the AWS Lambda handler that orchestrates
// the Calendar Agent execution flow.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http/httptest"
	"strings"

	"avagenc-gmail/internal/agent"
	"avagenc-gmail/internal/db"
	"avagenc-gmail/pkg/config"

	"go.naturallyfunny.dev/api"
	"go.naturallyfunny.dev/api/identity"

	"github.com/aws/aws-lambda-go/events"
)

// Request represents the expected JSON body from the API Gateway.
type Request struct {
	Message string `json:"message"`
}

// AgentData represents the JSON body returned in the data field.
type AgentData struct {
	UsedTool []string `json:"used_tool"`
	Response string   `json:"response"`
}

// HandleRequest is the main AWS Lambda handler function.
// Flow:
//  1. Parse request body (command + ownerId)
//  2. Fetch user data from Neon DB (calendar_connect table)
//  3. Validate scope — return early if null
//  4. Run the AI agent with the user's command
//  5. Return the agent's output and tool usage
func HandleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Load application config (cached after first Lambda cold start)
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[handler] config load error: %v", err)
		return errorResponse(api.Internal, "Internal configuration error"), nil
	}

	// Parse the request body
	var req Request
	if err := json.Unmarshal([]byte(event.Body), &req); err != nil {
		log.Printf("[handler] request body parse error: %v", err)
		return errorResponse(api.InvalidArgument, "Invalid request body: expected JSON with 'message'"), nil
	}

	// Validate required fields
	if req.Message == "" {
		return errorResponse(api.InvalidArgument, "Missing required field: 'message'"), nil
	}

	// Extract x-user-id header; try all common casings since API Gateway
	// may preserve or lowercase headers depending on the integration type.
	ownerID := extractHeader(event.Headers, "x-user-id")
	if ownerID == "" {
		return errorResponse(api.Unauthenticated, "Missing x-user-id header"), nil
	}

	ctx, err = identity.NewContextWithUserID(ctx, ownerID)
	if err != nil {
		return errorResponse(api.Unauthenticated, "Invalid user ID"), nil
	}

	// Fetch user calendar connection from the database
	calendarConn, err := db.FetchCalendarConnect(ctx, cfg.DatabaseURL, ownerID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return errorResponse(api.NotFound, "User calendar connection not found"), nil
		}
		log.Printf("[handler] db error: %v", err)
		return errorResponse(api.Internal, "Failed to fetch user data"), nil
	}

	// Check scope — if null, user has not granted calendar access
	if calendarConn.Scope == nil || *calendarConn.Scope == "" {
		return successResponse("Access Denied", AgentData{
			Response: "You do not have access to the calendar agent. Please connect your Google Calendar first.",
			UsedTool: []string{},
		})
	}

	// Run the AI agent
	result, err := agent.Run(ctx, agent.RunConfig{
		GeminiAPIKey:    cfg.GeminiAPIKey,
		Model:           cfg.Model,
		Message:         req.Message,
		RefreshToken:    calendarConn.RefreshToken,
		CalendarBaseURL: cfg.CalendarBaseURL,
		ContactBaseURL:  cfg.ContactBaseURL,
	})
	if err != nil {
		log.Printf("[handler] agent execution error: %v", err)
		return errorResponse(api.Internal, "Agent execution failed"), nil
	}

	log.Printf("[handler] agent completed successfully, tools used: %v", result.UsageTool)

	return successResponse("Message Processed", AgentData{
		Response: result.Output,
		UsedTool: result.UsageTool,
	})
}

// extractHeader does a case-insensitive lookup over the headers map.
func extractHeader(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// successResponse creates a well-formed API Gateway response with status 200.
func successResponse(message string, data AgentData) (events.APIGatewayProxyResponse, error) {
	rec := httptest.NewRecorder()
	api.WriteSuccess(rec, api.OK, message, data, nil)

	return events.APIGatewayProxyResponse{
		StatusCode: rec.Code,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: rec.Body.String(),
	}, nil
}

// errorResponse creates a well-formed API Gateway error response.
func errorResponse(code api.Code, message string) events.APIGatewayProxyResponse {
	rec := httptest.NewRecorder()
	api.WriteError(rec, api.NewError(code, message))

	return events.APIGatewayProxyResponse{
		StatusCode: rec.Code,
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
		Body: rec.Body.String(),
	}
}
