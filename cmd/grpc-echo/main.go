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
	"syscall"

	"a2a-simple/internal/a2autil"
	"a2a-simple/internal/auth"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2agrpc/v1"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/a2aproject/a2a-go/v2/a2asrv/push"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
	a2alog "github.com/a2aproject/a2a-go/v2/log"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

// echoExecutor implements a2asrv.AgentExecutor for a deterministic echo agent.
type echoExecutor struct{}

var _ a2asrv.AgentExecutor = (*echoExecutor)(nil)

// Execute reflects the received message parts back as named artifacts.
func (e *echoExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		if execCtx.StoredTask == nil {
			if !yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil) {
				return
			}
		}
		if !yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateWorking, nil), nil) {
			return
		}

		if execCtx.Message == nil || len(execCtx.Message.Parts) == 0 {
			evt := a2a.NewArtifactEvent(execCtx, a2a.NewTextPart("No content received"))
			evt.Artifact.Name = "echo"
			evt.LastChunk = true
			if yield(evt, nil) {
				_ = a2autil.FinalizeTask(ctx, execCtx, yield)
			}
			return
		}

		for i, part := range execCtx.Message.Parts {
			partEvt := a2a.NewArtifactEvent(execCtx, part)
			
			typeName := "unknown"
			switch part.Content.(type) {
			case a2a.Text:
				typeName = "text"
			case a2a.Data:
				typeName = "data"
			case a2a.Raw:
				typeName = "raw"
			case a2a.URL:
				typeName = "url"
			}
			partEvt.Artifact.Name = fmt.Sprintf("part-%d-%s", i, typeName)

			if i == len(execCtx.Message.Parts)-1 {
				partEvt.LastChunk = true
			}
			if !yield(partEvt, nil) {
				return
			}
		}

		_ = a2autil.FinalizeTask(ctx, execCtx, yield)
	}
}

// Cancel handles A2A task cancellation requests.
func (e *echoExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		yield(a2a.NewStatusUpdateEvent(execCtx, a2a.TaskStateCanceled, nil), nil)
	}
}

var (
	port     = flag.Int("port", 9002, "Port for the HTTP A2A server (JSON-RPC & REST)")
	grpcPort = flag.Int("grpc-port", 9003, "Port for the gRPC A2A server")
	level    = flag.String("level", "info", "Log level: debug, info, warn, error")
	logFile  = flag.String("log-file", "", "Path to log file (default: stderr only)")
	payload  = flag.Bool("payload", false, "Log request/response payloads")
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

	// 2. Setup shared persistence.
	store := taskstore.NewInMemory(&taskstore.InMemoryStoreConfig{Authenticator: a2asrv.NewTaskStoreAuthenticator()})
	pushStore := push.NewInMemoryStore()
	pushSender := push.NewHTTPPushSender(nil)

	// 3. Define the Agent Card with all three transports.
	agentCard := &a2a.AgentCard{
		Name:        "A2A Multi-Transport Echo Server",
		Description: "A reference server implementing gRPC, JSON-RPC, and REST/HTTP+JSON transports for deterministic A2A protocol validation.",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d/invoke", *port), a2a.TransportProtocolJSONRPC),
			a2a.NewAgentInterface(fmt.Sprintf("http://127.0.0.1:%d", *port), a2a.TransportProtocolHTTPJSON),
			a2a.NewAgentInterface(fmt.Sprintf("127.0.0.1:%d", *grpcPort), a2a.TransportProtocolGRPC),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text"},
		Capabilities:       a2a.AgentCapabilities{Streaming: true, PushNotifications: true},
		Skills: []a2a.AgentSkill{
			{ID: "echo", Name: "Echo", Description: "Echoes the received message parts back as named artifacts."},
		},
		SecuritySchemes: a2a.NamedSecuritySchemes{
			"bearerAuth": a2a.HTTPAuthSecurityScheme{Scheme: "Bearer", BearerFormat: "JWT"},
		},
	}

	// 4. Wire up the Request Handler.
	loggingInterceptor := a2asrv.NewLoggingInterceptor(&a2asrv.LoggingConfig{
		LogPayload: *payload,
	})
	executor := &echoExecutor{}
	requestHandler := a2asrv.NewHandler(
		executor,
		a2asrv.WithLogger(logger),
		a2asrv.WithCallInterceptors(loggingInterceptor, &auth.Interceptor{}),
		a2asrv.WithTaskStore(store),
		a2asrv.WithExecutorContextInterceptor(&a2asrv.ReferencedTasksLoader{Store: store}),
		a2asrv.WithPushNotifications(pushStore, pushSender),
	)

	// 5. Listeners setup.
	httpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		logger.Error("failed to bind HTTP port", "port", *port, "error", err)
		os.Exit(1)
	}

	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", *grpcPort))
	if err != nil {
		logger.Error("failed to bind gRPC port", "port", *grpcPort, "error", err)
		_ = httpListener.Close()
		os.Exit(1)
	}

	// 6. Multiplex HTTP handlers.
	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle("/", a2asrv.NewRESTHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	httpServer := &http.Server{Handler: mux}

	// 7. Initialize gRPC server.
	grpcServer := grpc.NewServer()
	a2agrpc.NewHandler(requestHandler).RegisterWith(grpcServer)

	// 8. Handle shutdown gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("gRPC server starting", "addr", grpcListener.Addr().String())
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("gRPC server failed", "error", err)
		}
	}()

	go func() {
		logger.Info("HTTP server starting (JSON-RPC & REST)", "addr", httpListener.Addr().String())
		if err := httpServer.Serve(httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
		}
	}()

	logger.Info("A2A Multi-Transport Echo Server running")
	<-stop
	logger.Info("shutting down servers gracefully...")

	grpcServer.GracefulStop()
	_ = httpServer.Shutdown(context.Background())
	logger.Info("servers stopped")
}
