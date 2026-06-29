package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ghchinoy/cloud-interactions-go"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/push"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	a2alog "github.com/a2aproject/a2a-go/v2/log"
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
func (e *agentExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		var textInput string
		if execCtx.Message != nil {
			for _, part := range execCtx.Message.Parts {
				if text, ok := part.Content.(a2a.Text); ok {
					textInput = string(text)
					break
				}
			}
		}

		// Dispatch logic: mapping input or metadata to Skill ID
		var selectedSkillID string
		if sid, ok := execCtx.Metadata["skillId"].(string); ok && sid != "" {
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

		slog.Info("dispatching skill", "task", execCtx.TaskID, "skill", selectedSkillID)

		// Authentication Gating
		if selectedSkillID == "admin_echo" {
			callCtx, ok := a2asrv.CallContextFrom(ctx)
			if !ok || !callCtx.User.Authenticated {
				slog.Warn("unauthorized skill access", "task", execCtx.TaskID, "skill", selectedSkillID)
				event := a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateRejected, a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Unauthorized: this skill requires a valid bearer token.")))
				yield(event, nil)
				return
			}
		}

		// Route to specific skill handlers (defined in skills.go)
		var err error
		switch selectedSkillID {
		case "hello_world":
			err = e.handleHelloWorld(ctx, execCtx, yield, textInput)
		case "echo":
			err = e.handleEcho(ctx, execCtx, yield, textInput)
		case "admin_echo":
			err = e.handleAdminEcho(ctx, execCtx, yield, textInput)
		case "ai_researcher":
			err = e.handleStatefulInteraction(ctx, execCtx, yield, textInput, true)
		case "chat":
			err = e.handleStatefulInteraction(ctx, execCtx, yield, textInput, false)
		case "summarize":
			err = e.handleSummarize(ctx, execCtx, yield, textInput)
		case "multimodal_echo":
			err = e.handleMultimodalEcho(ctx, execCtx, yield)
		default:
			slog.Warn("skill not found", "task", execCtx.TaskID, "skill", selectedSkillID)
			response := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("Skill %q not found.", selectedSkillID)))
			yield(response, nil)
		}
		
		if err != nil {
			yield(nil, err)
		}
	}
}

// Cancel handles A2A task cancellation requests.
func (*agentExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		slog.Info("task cancellation requested", "task", execCtx.TaskID)
	}
}

var (
	port    = flag.Int("port", 9001, "Port for the A2A server")
	level   = flag.String("level", "info", "Log level: debug, info, warn, error")
	logFile = flag.String("log-file", "", "Path to log file (default: stderr only)")
	payload = flag.Bool("payload", false, "Log request/response payloads")
)

func main() {
	flag.Parse()
	godotenv.Load()

	// 1. Configure structured logging.
	var slogLevel slog.Level
	switch *level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	var output io.Writer = os.Stderr
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			slog.Error("failed to open log file", "path", *logFile, "error", err)
			os.Exit(1)
		}
		defer f.Close()
		output = io.MultiWriter(os.Stderr, f)
	}

	logger := slog.New(a2alog.AttachFormatter(
		slog.NewTextHandler(output, &slog.HandlerOptions{
			Level:     slogLevel,
			AddSource: slogLevel == slog.LevelDebug,
		}),
		a2alog.DefaultA2ATypeFormatter,
	))
	slog.SetDefault(logger)

	// 2. Initialize core dependencies.
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	model := os.Getenv("GEMINI_MODEL")
	interactionsKey := os.Getenv("GEMINI_API_KEY")

	var gClient *genai.Client
	if project != "" && location != "" {
		ctx := context.Background()
		var err error
		gClient, err = genai.NewClient(ctx, &genai.ClientConfig{
			Project:  project,
			Location: location,
			Backend:  genai.BackendVertexAI,
		})
		if err != nil {
			logger.Error("failed to create GenAI client", "error", err)
		}
	}

	var iClient *interactions.Client
	if interactionsKey != "" {
		iClient = interactions.NewClient("https://generativelanguage.googleapis.com/v1beta/interactions").WithAPIKey(interactionsKey)
	}

	if model == "" {
		model = "gemini-2.0-flash"
	}

	// 3. Setup shared persistence (Explicit Store Pattern).
	store := taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: a2asrv.NewTaskStoreAuthenticator()})
	pushStore := push.NewInMemoryStore()
	pushSender := push.NewHTTPPushSender(nil)

	// 4. Define the agent identity.
	agentCard := &a2a.AgentCard{
		Name:        "Refactored A2A Agent",
		Description: "A clean, modular A2A service example",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", *port), a2a.TransportProtocolJSONRPC),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities:       a2a.AgentCapabilities{Streaming: true, PushNotifications: true},
		Skills: []a2a.AgentSkill{
			{ID: "hello_world", Name: "Hello World", Description: "Friendly Gemini greeting"},
			{ID: "echo", Name: "Echo", Description: "Stateless echo"},
			{ID: "admin_echo", Name: "Admin Echo", Description: "Auth-gated skill", SecurityRequirements: a2a.SecurityRequirementsOptions{{"bearerAuth": {}}}},
			{ID: "ai_researcher", Name: "AI Researcher", Description: "Deep research via Interactions API"},
			{ID: "summarize", Name: "Summarizer", Description: "Cross-task artifact summarization"},
			{ID: "chat", Name: "Stateful Chat", Description: "Conversation with session history"},
			{ID: "multimodal_echo", Name: "Multimodal Echo", Description: "Echoes all received message parts back as artifacts. Use to validate multi-modal message construction."},
		},
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"bearerAuth": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "JWT"},
		},
	}

	// 5. Wire up the A2A request handler.
	loggingInterceptor := a2asrv.NewLoggingInterceptor(&a2asrv.LoggingConfig{
		LogPayload: *payload,
	})
	executor := &agentExecutor{gClient, model, iClient}
	requestHandler := a2asrv.NewHandler(
		executor,
		a2asrv.WithLogger(logger),
		a2asrv.WithCallInterceptors(loggingInterceptor, &authInterceptor{}),
		a2asrv.WithTaskStore(store),
		a2asrv.WithExecutorContextInterceptor(&a2asrv.ReferencedTasksLoader{Store: store}),
		a2asrv.WithPushNotifications(pushStore, pushSender),
	)

	// 6. Bind and start the HTTP server.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logger.Error("failed to bind port", "port", *port, "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	logger.Info("A2A server starting",
		"protocol", a2a.Version,
		"addr", listener.Addr().String(),
		"level", *level,
		"payload_logging", *payload,
	)
	if err := http.Serve(listener, mux); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
