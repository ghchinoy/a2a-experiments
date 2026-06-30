package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"a2a-simple/internal/a2autil"
	"a2a-simple/internal/auth"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/push"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	a2alog "github.com/a2aproject/a2a-go/v2/log"
	"github.com/joho/godotenv"
)

// multimodalExecutor implements a2asrv.AgentExecutor.
type multimodalExecutor struct {
	assetsDir string
}

var _ a2asrv.AgentExecutor = (*multimodalExecutor)(nil)

// Execute manages multimodal skills and task states.
func (e *multimodalExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		// Initialize task
		if execCtx.StoredTask == nil {
			if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
				return
			}
		}

		skillID := ""
		if sid, ok := execCtx.Metadata["skillId"].(string); ok && sid != "" {
			skillID = sid
		} else if execCtx.Message != nil {
			// Fallback: extract skill ID from text input keywords
			for _, part := range execCtx.Message.Parts {
				if text, ok := part.Content.(a2a.Text); ok {
					txt := strings.ToLower(string(text))
					switch {
					case strings.Contains(txt, "all-artifacts"):
						skillID = "all-artifacts"
					case strings.Contains(txt, "state-working"):
						skillID = "state-working"
					case strings.Contains(txt, "state-input-required"):
						skillID = "state-input-required"
					case strings.Contains(txt, "state-auth-required"):
						skillID = "state-auth-required"
					case strings.Contains(txt, "state-completed"):
						skillID = "state-completed"
					case strings.Contains(txt, "state-failed"):
						skillID = "state-failed"
					case strings.Contains(txt, "state-canceled"):
						skillID = "state-canceled"
					}
				}
			}
		}

		slog.Info("executing multimodal skill", "task", execCtx.TaskID, "skill", skillID)

		switch skillID {
		case "all-artifacts":
			e.handleAllArtifacts(ctx, execCtx, yield)
		case "state-working":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateWorking)
		case "state-input-required":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateInputRequired)
		case "state-auth-required":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateAuthRequired)
		case "state-completed":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateCompleted)
		case "state-failed":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateFailed)
		case "state-canceled":
			e.handleState(ctx, execCtx, yield, a2a.TaskStateCanceled)
		default:
			// Default fallback: return unknown skill error
			msg := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(fmt.Sprintf("Unknown multimodal skill %q.", skillID)))
			yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateFailed, msg), nil)
		}
	}
}

// Cancel handles A2A task cancellation requests.
func (e *multimodalExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

// handleAllArtifacts generates one of each part type (Text, Data, Raw, FileURL).
func (e *multimodalExecutor) handleAllArtifacts(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool) {
	if !yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil) {
		return
	}

	// 1. Check for local deterministic files
	requiredFiles := []string{"sample.png", "sample.wav", "sample.mp3", "sample.mp4", "sample.pdf"}
	for _, filename := range requiredFiles {
		path := filepath.Join(e.assetsDir, filename)
		if _, err := os.Stat(path); err != nil {
			errMessage := fmt.Sprintf("Required test asset %q not found in directory %q. Please run \"./scripts/gen-assets.sh\" to generate local test fixtures.", filename, e.assetsDir)
			slog.Error("missing asset", "file", filename, "path", e.assetsDir)
			msg := a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart(errMessage))
			yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateFailed, msg), nil)
			return
		}
	}

	// 2. Yield Text Artifact
	textEvt := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart("This is a deterministic test text part."))
	textEvt.Artifact.Name = "text-artifact"
	textEvt.Artifact.Description = "A basic text artifact"
	if !yield(textEvt, nil) {
		return
	}

	// 3. Yield Data Artifact
	dataObj := map[string]any{
		"status":    "ok",
		"service":   "multimodal-kitchen-sink",
		"checksums": "deterministic",
	}
	dataPart := a2a.NewDataPart(dataObj)
	dataPart.MediaType = "application/json"
	dataEvt := a2a.NewArtifactEvent(execCtx, dataPart)
	dataEvt.Artifact.Name = "data-artifact"
	dataEvt.Artifact.Description = "A structured JSON data artifact"
	if !yield(dataEvt, nil) {
		return
	}

	// 4. Yield Raw Binary Artifact (Image)
	pngBytes, err := os.ReadFile(filepath.Join(e.assetsDir, "sample.png"))
	if err != nil {
		yield(nil, fmt.Errorf("failed to read sample.png: %w", err))
		return
	}
	rawPart := a2a.NewRawPart(pngBytes)
	rawPart.MediaType = "image/png"
	rawEvt := a2a.NewArtifactEvent(execCtx, rawPart)
	rawEvt.Artifact.Name = "raw-artifact"
	rawEvt.Artifact.Description = "An inline PNG raw byte artifact"
	if !yield(rawEvt, nil) {
		return
	}

	// 5. Yield FileURL Artifact (Audio)
	urlPath := fmt.Sprintf("http://127.0.0.1:%d/assets/sample.mp3", *port)
	urlPart := a2a.NewFileURLPart(a2a.URL(urlPath), "audio/mp3")
	urlEvt := a2a.NewArtifactEvent(execCtx, urlPart)
	urlEvt.Artifact.Name = "fileurl-artifact"
	urlEvt.Artifact.Description = "A self-hosted FileURL audio download artifact"
	urlEvt.LastChunk = true
	if !yield(urlEvt, nil) {
		return
	}

	// Complete task
	_ = a2autil.FinalizeTask(ctx, execCtx, yield)
}

// handleState transitions task into specific requested operational states.
func (e *multimodalExecutor) handleState(ctx context.Context, execCtx *a2asrv.ExecutorContext, yield func(a2a.Event, error) bool, state a2a.TaskState) {
	var statusMsg *a2a.Message

	switch state {
	case a2a.TaskStateWorking:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Task state is now: Working"))
	case a2a.TaskStateInputRequired:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Please provide verification code:"))
	case a2a.TaskStateAuthRequired:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Authentication is required to proceed with this task."))
	case a2a.TaskStateCompleted:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Task completed successfully."))
	case a2a.TaskStateFailed:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Task failed due to deterministic test error condition."))
	case a2a.TaskStateCanceled:
		statusMsg = a2a.NewMessageForTask(a2a.MessageRoleAgent, execCtx, a2a.NewTextPart("Task was canceled on demand."))
	}

	yield(a2a.NewStatusUpdateEvent(execCtx, state, statusMsg), nil)
}

var (
	port      = flag.Int("port", 9004, "Port for the kitchen-sink multimodal A2A server")
	assetsDir = flag.String("assets", "cmd/multimodal/testdata/assets", "Directory containing tiny static test assets")
	level     = flag.String("level", "info", "Log level: debug, info, warn, error")
	logFile   = flag.String("log-file", "", "Path to log file (default: stderr only)")
	payload   = flag.Bool("payload", false, "Log request/response payloads")
)

func main() {
	flag.Parse()
	_ = godotenv.Load()

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
		defer func() { _ = f.Close() }()
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

	// Verify assets existence on startup and log friendly warning if missing
	requiredFiles := []string{"sample.png", "sample.wav", "sample.mp3", "sample.mp4", "sample.pdf"}
	for _, filename := range requiredFiles {
		path := filepath.Join(*assetsDir, filename)
		if _, err := os.Stat(path); err != nil {
			logger.Warn("deterministic test asset missing, 'all-artifacts' skill will fail until generated", "file", filename, "path", *assetsDir, "solution", "Run \"./scripts/gen-assets.sh\" to generate assets.")
		}
	}

	// 2. Setup shared persistence.
	store := taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: a2asrv.NewTaskStoreAuthenticator()})
	pushStore := push.NewInMemoryStore()
	pushSender := push.NewHTTPPushSender(nil)

	// 3. Define Agent Card.
	agentCard := &a2a.AgentCard{
		Name:        "A2A Multimodal Reference Server",
		Description: "A high-fidelity kitchen-sink reference server for deterministically testing all artifact types and task states.",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", *port), a2a.TransportProtocolJSONRPC),
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d", *port), a2a.TransportProtocolHTTPJSON),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities:       a2a.AgentCapabilities{Streaming: true, PushNotifications: true},
		Skills: []a2a.AgentSkill{
			{ID: "all-artifacts", Name: "All Artifacts", Description: "Returns Text, Data, Raw PNG, and local FileURL MP3 download in a single task execution."},
			{ID: "state-working", Name: "State Working", Description: "Drives task into TASK_STATE_WORKING."},
			{ID: "state-input-required", Name: "State Input Required", Description: "Drives task into TASK_STATE_INPUT_REQUIRED."},
			{ID: "state-auth-required", Name: "State Auth Required", Description: "Drives task into TASK_STATE_AUTH_REQUIRED."},
			{ID: "state-completed", Name: "State Completed", Description: "Drives task into TASK_STATE_COMPLETED."},
			{ID: "state-failed", Name: "State Failed", Description: "Drives task into TASK_STATE_FAILED."},
			{ID: "state-canceled", Name: "State Canceled", Description: "Drives task into TASK_STATE_CANCELED."},
		},
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"bearerAuth": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "JWT"},
		},
	}

	// 4. Wire up Request Handler.
	loggingInterceptor := a2asrv.NewLoggingInterceptor(&a2asrv.LoggingConfig{
		LogPayload: *payload,
	})
	executor := &multimodalExecutor{assetsDir: *assetsDir}
	requestHandler := a2asrv.NewHandler(
		executor,
		a2asrv.WithLogger(logger),
		a2asrv.WithCallInterceptors(loggingInterceptor, &auth.Interceptor{}),
		a2asrv.WithTaskStore(store),
		a2asrv.WithExecutorContextInterceptor(&a2asrv.ReferencedTasksLoader{Store: store}),
		a2asrv.WithPushNotifications(pushStore, pushSender),
	)

	// 5. Setup multiplexer and serve static assets
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle("/", a2asrv.NewRESTHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	// Mount local static directory `/assets/` to serve embedded or generated bytes dynamically
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(*assetsDir))))

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logger.Error("failed to bind port", "port", *port, "error", err)
		os.Exit(1)
	}

	srv := &http.Server{Handler: mux}

	// 6. Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Multimodal reference server starting (JSON-RPC & REST)", "addr", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped with error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down multimodal server gracefully...")
	_ = srv.Shutdown(context.Background())
	logger.Info("multimodal server stopped")
}
