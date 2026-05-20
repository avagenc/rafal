// Package tools provides GenAI function declarations and HTTP-based
// tool execution for the Calendar Agent.
package tools

import (
	"google.golang.org/genai"
)

// ToolNameGetEvent is the function name for retrieving calendar events.
const ToolNameGetEvent = "get_event"

// ToolNameAddEvent is the function name for adding calendar events.
const ToolNameAddEvent = "add_event"

// ToolNameGetUserContact is the function name for retrieving user contacts.
const ToolNameGetUserContact = "get_user_contact"

// ToolNameAddUserContact is the function name for adding user contacts.
const ToolNameAddUserContact = "add_user_contact"

// CalendarTools returns the genai.Tool containing all function declarations
// available to the Calendar Agent.
func CalendarTools() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				getEventDeclaration(),
				addEventDeclaration(),
				getUserContactDeclaration(),
				addUserContactDeclaration(),
			},
		},
	}
}

// getEventDeclaration defines the get_event tool schema.
// At least one of query, date_start, or date_end must be provided.
func getEventDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        ToolNameGetEvent,
		Description: "Retrieve calendar events from Google Calendar. At least one of query, date_start, or date_end must be provided. Empty string means the filter is not activated.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"query": {
					Type:        genai.TypeString,
					Description: "Keyword search applied to event fields (Google Calendar full-text search). Use empty string if not needed.",
				},
				"date_start": {
					Type:        genai.TypeString,
					Description: "RFC3339 datetime string. Used as TimeMin filter (events starting after this time). Use empty string if not needed.",
				},
				"date_end": {
					Type:        genai.TypeString,
					Description: "RFC3339 datetime string. Used as TimeMax filter (events before this time). Use empty string if not needed.",
				},
				"calendar_id": {
					Type:        genai.TypeString,
					Description: "Calendar ID to query. Use empty string to default to 'primary'.",
				},
			},
			Required: []string{"query", "date_start", "date_end", "calendar_id"},
		},
	}
}

// addEventDeclaration defines the add_event tool schema.
func addEventDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        ToolNameAddEvent,
		Description: "Add a new event to Google Calendar. event_title, time_start, and time_end are mandatory. time_start and time_end must be valid RFC3339 datetime strings. guest must be a semicolon-separated string of emails, not a JSON array.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"event_title": {
					Type:        genai.TypeString,
					Description: "The title (summary) of the event.",
				},
				"time_start": {
					Type:        genai.TypeString,
					Description: "Event start time in RFC3339 format. Example: 2026-02-25T09:00:00+07:00",
				},
				"time_end": {
					Type:        genai.TypeString,
					Description: "Event end time in RFC3339 format. Must be after time_start.",
				},
				"calendar_id": {
					Type:        genai.TypeString,
					Description: "Target calendar ID. Use empty string to default to 'primary'.",
				},
				"description": {
					Type:        genai.TypeString,
					Description: "Event description text. Use empty string if not needed.",
				},
				"location": {
					Type:        genai.TypeString,
					Description: "Physical or virtual meeting location. Use empty string if not needed.",
				},
				"guest": {
					Type:        genai.TypeString,
					Description: "Attendee emails separated by semicolons. Example: user1@email.com;user2@email.com. Use empty string if not needed.",
				},
			},
			Required: []string{"event_title", "time_start", "time_end", "calendar_id", "description", "location", "guest"},
		},
	}
}

// getUserContactDeclaration defines the get_user_contact tool schema.
func getUserContactDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        ToolNameGetUserContact,
		Description: "Get the user's contact list from the database. It returns contacts including their gmail and phone numbers. Use the name parameter to filter the result specifically for the requested person.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"name": {
					Type:        genai.TypeString,
					Description: "The name of the contact to search for.",
				},
			},
			Required: []string{"name"},
		},
	}
}

// addUserContactDeclaration defines the add_user_contact tool schema.
func addUserContactDeclaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        ToolNameAddUserContact,
		Description: "Add a new user contact with Gmail or phone number. Use this when the user provides contact information that is not yet in the contact list. Either gmail or phone must be provided.",
		Parameters: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"name": {
					Type:        genai.TypeString,
					Description: "The name of the contact.",
				},
				"gmail": {
					Type:        genai.TypeString,
					Description: "The Gmail address of the contact. Use empty string if not provided.",
				},
				"phone": {
					Type:        genai.TypeString,
					Description: "The phone number of the contact. Use empty string if not provided.",
				},
			},
			Required: []string{"name", "gmail", "phone"},
		},
	}
}
