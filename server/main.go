package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"a2a-simple/pkg/interactions"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"github.com/a2aproject/a2a-go/a2asrv/eventqueue"
	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

// agentExecutor handles A2A request execution and skill dispatching.
type agentExecutor struct {
	genaiClient        *genai.Client
	model              string
	interactionsClient *interactions.Client
}

var _ a2asrv.AgentExecutor = (*agentExecutor)(nil)

// Execute is the entry point for all A2A skill invocations.
func (e *agentExecutor) Execute(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue) error {
	var textInput string
	if reqCtx.Message != nil {
		for _, part := range reqCtx.Message.Parts {
			if tp, ok := part.(a2a.TextPart); ok {
				textInput = tp.Text
				break
			}
		}
	}

	// Dispatch logic: mapping input or metadata to Skill ID
	var selectedSkillID string
	if sid, ok := reqCtx.Metadata["skillId"].(string); ok && sid != "" {
		selectedSkillID = sid
	} else {
		switch {
		case textInput == "hello" || textInput == "hi":
			selectedSkillID = "hello_world"
		case strings.Contains(strings.ToLower(textInput), "research"):
			selectedSkillID = "ai_researcher"
		case strings.Contains(strings.ToLower(textInput), "summarize"):
			selectedSkillID = "summarize"
		case textInput != "":
			selectedSkillID = "chat"
		default:
			selectedSkillID = "unknown"
		}
	}

	log.Printf("[Task: %s] Dispatching to Skill: %q", reqCtx.TaskID, selectedSkillID)

	// Authentication Gating
	if selectedSkillID == "admin_echo" {
		callCtx, ok := a2asrv.CallContextFrom(ctx)
		if !ok || !callCtx.User.Authenticated() {
			log.Printf("[Task: %s] Unauthorized access attempt to %q", reqCtx.TaskID, selectedSkillID)
			event := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateRejected, a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: "Unauthorized: this skill requires a valid bearer token."}))
			return q.Write(ctx, event)
		}
	}

	// Route to specific skill handlers (defined in skills.go)
	switch selectedSkillID {
	case "hello_world":
		return e.handleHelloWorld(ctx, reqCtx, q, textInput)
	case "echo":
		return e.handleEcho(ctx, reqCtx, q, textInput)
	case "admin_echo":
		return e.handleAdminEcho(ctx, reqCtx, q, textInput)
	case "ai_researcher":
		return e.handleStatefulInteraction(ctx, reqCtx, q, textInput, true)
	case "chat":
		return e.handleStatefulInteraction(ctx, reqCtx, q, textInput, false)
	case "summarize":
		return e.handleSummarize(ctx, reqCtx, q, textInput)
	default:
		log.Printf("[Task: %s] Skill %q not found.", reqCtx.TaskID, selectedSkillID)
		response := a2a.NewMessageForTask(a2a.MessageRoleAgent, reqCtx, a2a.TextPart{Text: fmt.Sprintf("Skill %q not found.", selectedSkillID)})
		return q.Write(ctx, response)
	}
}

// Cancel handles A2A task cancellation requests.
func (*agentExecutor) Cancel(ctx context.Context, reqCtx *a2asrv.RequestContext, q eventqueue.Queue) error {
	log.Printf("[Task: %s] Cancellation requested", reqCtx.TaskID)
	return nil
}

var port = flag.Int("port", 9001, "Port for the A2A server")

func main() {
	flag.Parse()
	godotenv.Load()

	// 1. Initialize Core Dependencies
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	model := os.Getenv("GEMINI_MODEL")
	interactionsKey := os.Getenv("GEMINI_API_KEY")

	var gClient *genai.Client
	if project != "" && location != "" {
		ctx := context.Background()
		gClient, _ = genai.NewClient(ctx, &genai.ClientConfig{
			Project:  project,
			Location: location,
			Backend:  genai.BackendVertexAI,
		})
	}

	var iClient *interactions.Client
	if interactionsKey != "" {
		iClient = interactions.NewClient(interactionsKey)
	}

	if model == "" {
		model = "gemini-2.0-flash"
	}

	// 2. Setup Shared Persistence (Explicit Store Pattern)
	store := newMemStore()

	// 3. Define the Agent Identity
	agentCard := &a2a.AgentCard{
		Name:               "Refactored A2A Agent",
		Description:        "A clean, modular A2A service example",
		URL:                fmt.Sprintf("http://127.0.0.1:%d/invoke", *port),
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		Capabilities:       a2a.AgentCapabilities{Streaming: true},
		Skills: []a2a.AgentSkill{
			{ID: "hello_world", Name: "Hello World", Description: "Friendly Gemini greeting"},
			{ID: "echo", Name: "Echo", Description: "Stateless echo"},
			{ID: "admin_echo", Name: "Admin Echo", Description: "Auth-gated skill", Security: []a2a.SecurityRequirements{{"bearerAuth": {}}}},
			{ID: "ai_researcher", Name: "AI Researcher", Description: "Deep research via Interactions API"},
			{ID: "summarize", Name: "Summarizer", Description: "Cross-task artifact summarization"},
			{ID: "chat", Name: "Stateful Chat", Description: "Conversation with session history"},
		},
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"bearerAuth": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "JWT"},
		},
	}

	// 4. Wire up the A2A Request Handler
	executor := &agentExecutor{gClient, model, iClient}
	requestHandler := a2asrv.NewHandler(
		executor,
		a2asrv.WithCallInterceptor(&authInterceptor{}),
		a2asrv.WithTaskStore(store),
		a2asrv.WithRequestContextInterceptor(&a2asrv.ReferencedTasksLoader{Store: store}),
	)

	// 5. Start HTTP Server
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	log.Printf("A2A Server starting on :%d", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), mux); err != nil {
		log.Fatal(err)
	}
}
