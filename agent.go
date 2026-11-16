package rafal

import (
	_ "embed"
	"fmt"

	adkgworkspace "go.trueardian.com/agentkit/gworkspace/adk"
	"go.trueardian.com/gworkspace"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
)

// Config holds the dependencies a consumer must supply. Rafal's identity and
// system instruction are owned by the module, not configured here. Per-channel
// or per-run instruction is the consumer's concern — append it from a
// llmagent.BeforeModelCallback / plugin on your own runner.
type Config struct {
	Model model.LLM
	// WorkspaceClient is the single Google Workspace client backing Gmail,
	// Calendar, and Contacts alike — one OAuth token covers all three.
	WorkspaceClient *gworkspace.Client
	// AdditionalInstruction is appended to Rafal's base system instruction.
	// Use it to supply channel-specific or deployment-specific context that
	// the module itself cannot know.
	AdditionalInstruction string
}

//go:embed internal/description.txt
var description string

//go:embed internal/instruction.txt
var systemInstruction string

// New builds the Rafal agent — a unified Gmail/Calendar/Contacts LLM agent.
// Running it (runner, session, and any per-run instruction) is the
// consumer's responsibility.
func New(cfg Config) (agent.Agent, error) {
	calendar, err := gworkspace.NewCalendar(cfg.WorkspaceClient)
	if err != nil {
		return nil, fmt.Errorf("rafal: calendar: %w", err)
	}
	gmail, err := gworkspace.NewGmail(cfg.WorkspaceClient)
	if err != nil {
		return nil, fmt.Errorf("rafal: gmail: %w", err)
	}
	contacts, err := gworkspace.NewContacts(cfg.WorkspaceClient)
	if err != nil {
		return nil, fmt.Errorf("rafal: contacts: %w", err)
	}

	calendarTools, err := adkgworkspace.CalendarTools(calendar)
	if err != nil {
		return nil, fmt.Errorf("rafal: calendar tools: %w", err)
	}
	gmailTools, err := adkgworkspace.GmailTools(gmail)
	if err != nil {
		return nil, fmt.Errorf("rafal: gmail tools: %w", err)
	}
	contactTools, err := adkgworkspace.ContactTools(contacts)
	if err != nil {
		return nil, fmt.Errorf("rafal: contact tools: %w", err)
	}

	tools := make([]adktool.Tool, 0, len(calendarTools)+len(gmailTools)+len(contactTools))
	tools = append(tools, calendarTools...)
	tools = append(tools, gmailTools...)
	tools = append(tools, contactTools...)

	instruction := "[SYSTEM_INSTRUCTION]" + systemInstruction + "\n[/SYSTEM_INSTRUCTION]"
	if cfg.AdditionalInstruction != "" {
		instruction = "[SYSTEM_INSTRUCTION]" + systemInstruction + "\n\n" + cfg.AdditionalInstruction + "\n[/SYSTEM_INSTRUCTION]"
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "rafal",
		Model:       cfg.Model,
		Tools:       tools,
		Description: description,
		Instruction: instruction,
	})
	if err != nil {
		return nil, fmt.Errorf("rafal: agent: %w", err)
	}
	return a, nil
}
