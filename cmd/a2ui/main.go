package main

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"iter"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"a2a-simple/internal/auth"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
	a2alog "github.com/a2aproject/a2a-go/v2/log"
	"github.com/joho/godotenv"
	"google.golang.org/genai"
)

const (
	// a2uiMimeType is the IANA-conformant A2UI v1.0 media type used to identify
	// A2UI DataParts.
	a2uiMimeType = "application/a2ui+json"

	// a2uiVersion is stamped onto every emitted A2UI envelope.
	a2uiVersion = "v1.0"

	// a2uiExtensionURI is the A2A extension URI advertised on the Agent Card so
	// clients can negotiate A2UI v1.0 support.
	a2uiExtensionURI = "https://a2ui.org/a2a-extension/a2ui/v1.0"

	// basicCatalogID is the canonical v1.0 basic catalog identifier.
	basicCatalogID = "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json"
)

func init() {
	// The A2A SDK clones artifacts via gob; the A2UI DataPart now carries a
	// []map[string]any (the ordered message list), so register that concrete
	// type for the gob-encoded Part.Content (Data.Value any).
	gob.Register([]map[string]any{})
}

type ServerState struct {
	mu         sync.RWMutex
	surfaceIDs map[string]string // Maps ContextID to SurfaceID
}

func newServerState() *ServerState {
	return &ServerState{
		surfaceIDs: make(map[string]string),
	}
}

type agentExecutor struct {
	client *genai.Client
	state  *ServerState
}

func (e *agentExecutor) Execute(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		var inputText string
		var incomingSurfaceId string
		if execCtx.Message != nil {
			for _, p := range execCtx.Message.Parts {
				if p.Text() != "" {
					inputText += p.Text()
				} else if p.Data() != nil {
					slog.Info("Received A2A Data Part", "data", p.Data())
					dataMap, ok := p.Data().(map[string]any)
					if ok {
						if actionMap, ok := dataMap["action"].(map[string]any); ok {
							if name, ok := actionMap["name"].(string); ok {
								inputText += fmt.Sprintf("\n[System Event: The user triggered UI action '%s']", name)
							}
							if sID, ok := actionMap["surfaceId"].(string); ok {
								incomingSurfaceId = sID
							}
						} else if userAction, ok := dataMap["userAction"].(map[string]any); ok { // Fallback for alternate schema
							if name, ok := userAction["actionName"].(string); ok {
								inputText += fmt.Sprintf("\n[System Event: The user triggered UI action '%s']", name)
							}
							if sID, ok := userAction["surfaceId"].(string); ok {
								incomingSurfaceId = sID
							}
						} else if errorMap, ok := dataMap["error"].(map[string]any); ok {
							if msg, ok := errorMap["message"].(string); ok {
								inputText += fmt.Sprintf("\n[System Event: The client failed to render your UI. Error: %s]", msg)
							}
						}
					}
				}
			}
		}

		targetSurfaceId := incomingSurfaceId
		e.state.mu.Lock()
		if targetSurfaceId == "" {
			// Check if we have one for this context
			if sID, exists := e.state.surfaceIDs[execCtx.ContextID]; exists {
				targetSurfaceId = sID
			} else {
				targetSurfaceId = string(a2a.NewTaskID())
				e.state.surfaceIDs[execCtx.ContextID] = targetSurfaceId
			}
		} else {
			// Update the context mapping with the incoming one
			e.state.surfaceIDs[execCtx.ContextID] = targetSurfaceId
		}
		e.state.mu.Unlock()

		if strings.TrimSpace(inputText) == "" {
			inputText = "Hello"
		}

		systemPrompt := fmt.Sprintf(`
You are a helpful assistant. Your final output MUST be an A2UI v1.0 UI JSON response.

To generate the response, you MUST follow these rules:
1. Your response MUST be in two parts, separated by the delimiter: '---a2ui_JSON---'.
2. The first part is your conversational text response.
3. The second part is a single, raw JSON array which is a list of A2UI messages.
4. Every message MUST set "version": "v1.0".
5. The JSON part MUST validate against the A2UI JSON SCHEMA provided below.

--- UI TEMPLATE RULES ---
- You MUST use the 'BASIC_CARD_EXAMPLE' template for most visual responses, populating the 'updateDataModel.value' intelligently based on the user's query.
- You MAY use the 'SINGLE_MESSAGE_CARD_EXAMPLE' template (which embeds 'components' and 'dataModel' directly inside 'createSurface') for simple, self-contained cards that can be built in one message.
- You MAY use the 'FORM_EXAMPLE' template if the user asks to see a form, input fields, or submit data. You can populate 'updateDataModel.value' with form defaults.
- For Text components, choose an appropriate 'variant' ('body' or 'caption') based on the hierarchy of the information. Do NOT use unsupported variant values like 'h1', 'h2', 'h3', etc., as the catalog only allows 'body' and 'caption'.
- If the user triggers a simple state toggle or standalone button (e.g., clicking 'Like' or 'Confirm'), you MUST ONLY output an 'updateDataModel' payload to visually acknowledge the action on the card (e.g., changing the button label to 'Clicked!'). You MUST NOT output any conversational text. Use the 'SILENT_STATE_MUTATION_EXAMPLE'. Keep the interaction silent.
- If the user submits a form containing data, you must perform two actions simultaneously using the 'HYBRID_FORM_RECEIPT_EXAMPLE': 1) Use 'updateDataModel' to mutate the historical form card to a disabled or 'submitted' state to prevent duplicate entries. 2) Provide a conversational text response acknowledging the receipt of the data.

%s

---BEGIN A2UI JSON SCHEMA---
%s
---END A2UI JSON SCHEMA---
`, A2UIExamples, A2UISchema)

		req := &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{
					{Text: systemPrompt},
				},
			},
		}

		resp, err := e.client.Models.GenerateContent(
			ctx,
			"gemini-3.1-flash-lite-preview",
			genai.Text(inputText),
			req,
		)

		if err != nil {
			yield(nil, fmt.Errorf("failed to generate content from Gemini: %w", err))
			return
		}

		if resp != nil && len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			var outText string
			for _, p := range resp.Candidates[0].Content.Parts {
				outText += p.Text
			}

			slog.Info("RAW LLM RESPONSE", "text", outText)

			// Parse A2UI delimiter
			parts := strings.SplitN(outText, "---a2ui_JSON---", 2)
			textContent := strings.TrimSpace(parts[0])

			var textParts []*a2a.Part
			var uiParts []*a2a.Part

			if textContent != "" {
				textParts = append(textParts, a2a.NewTextPart(textContent))
			}

			if len(parts) > 1 {
				jsonString := strings.TrimSpace(parts[1])
				jsonString = strings.TrimPrefix(jsonString, "```json")
				jsonString = strings.TrimSuffix(jsonString, "```")
				jsonString = strings.TrimSpace(jsonString)

				var jsonMessages []map[string]any
				if err := json.Unmarshal([]byte(jsonString), &jsonMessages); err != nil {
					slog.Error("Failed to parse A2UI JSON", "error", err)
					textParts = append(textParts, a2a.NewTextPart(fmt.Sprintf("\n[Error parsing A2UI JSON: %v]\n%s", err, jsonString)))
				} else {
					for _, msg := range jsonMessages {
						// Stamp the protocol version so the stream is conformant
						// regardless of what the LLM emitted.
						msg["version"] = a2uiVersion

						// Inject targetSurfaceId into the message if it has a default surfaceId
						for _, key := range []string{"createSurface", "updateComponents", "updateDataModel", "deleteSurface"} {
							if val, ok := msg[key].(map[string]any); ok {
								if sID, ok := val["surfaceId"].(string); ok && (sID == "default" || sID == "") {
									val["surfaceId"] = targetSurfaceId
								}
							}
						}
					}

					// A2UI v1.0: a single DataPart carries the ORDERED LIST of
					// messages in its `data` field (not one DataPart per message).
					dataPart := a2a.NewDataPart(jsonMessages)
					dataPart.MediaType = a2uiMimeType
					dataPart.Metadata = map[string]any{"mimeType": a2uiMimeType}
					uiParts = append(uiParts, dataPart)
				}
			}

			tID := execCtx.TaskID
			if tID == "" {
				tID = a2a.NewTaskID()
			}

			// Always yield Task first so the SDK knows we are streaming task events
			task := &a2a.Task{
				ID:        tID,
				ContextID: execCtx.ContextID,
				Status:    a2a.TaskStatus{State: a2a.TaskStateWorking},
			}
			yield(task, nil)

			var allParts []*a2a.Part
			allParts = append(allParts, textParts...)

			if len(uiParts) > 0 {
				artifact := &a2a.Artifact{
					ID:    a2a.NewArtifactID(),
					Name:  "text_card",
					Parts: uiParts,
				}

				yield(&a2a.TaskArtifactUpdateEvent{
					Artifact:  artifact,
					ContextID: execCtx.ContextID,
					TaskID:    tID,
				}, nil)
			}

			if len(allParts) > 0 {
				msg := a2a.NewMessage(a2a.MessageRoleAgent, allParts...)
				yield(&a2a.TaskStatusUpdateEvent{
					TaskID:    tID,
					ContextID: execCtx.ContextID,
					Status: a2a.TaskStatus{
						State:   a2a.TaskStateCompleted,
						Message: msg,
					},
				}, nil)
			} else {
				msg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("No response generated."))
				yield(&a2a.TaskStatusUpdateEvent{
					TaskID:    tID,
					ContextID: execCtx.ContextID,
					Status: a2a.TaskStatus{
						State:   a2a.TaskStateCompleted,
						Message: msg,
					},
				}, nil)
			}
		} else {
			yield(a2a.NewMessage(a2a.MessageRoleAgent, a2a.NewTextPart("No response generated.")), nil)
		}
	}
}

func (e *agentExecutor) Cancel(ctx context.Context, execCtx *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {}
}

var (
	port    = flag.Int("port", 9005, "Port for the server to listen on.")
	level   = flag.String("level", "info", "Log level: debug, info, warn, error.")
	logFile = flag.String("log-file", "", "Path to a log file. Logs go to stderr when empty.")
	payload = flag.Bool("payload", false, "Enable request/response payload logging.")
)

func main() {
	flag.Parse()

	_ = godotenv.Load()

	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	location := os.Getenv("GOOGLE_CLOUD_LOCATION")

	if project == "" || location == "" {
		log.Fatalf("GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION environment variables must be set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}

	var slogLevel slog.Level
	switch *level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		log.Fatalf("unknown log level %q, use debug|info|warn|error", *level)
	}

	var output io.Writer = os.Stderr
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer func() { _ = f.Close() }()
		output = io.MultiWriter(os.Stderr, f)
	}

	jsonHandler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: slogLevel == slog.LevelDebug,
	})
	logger := slog.New(a2alog.AttachFormatter(jsonHandler, a2alog.DefaultA2ATypeFormatter))

	loggingInterceptor := a2asrv.NewLoggingInterceptor(&a2asrv.LoggingConfig{
		LogPayload: *payload,
	})

	serverState := newServerState()
	executor := &agentExecutor{client: client, state: serverState}

	requestHandler := a2asrv.NewHandler(executor,
		a2asrv.WithLogger(logger),
		a2asrv.WithCallInterceptors(loggingInterceptor, &auth.Interceptor{}),
	)

	addr := fmt.Sprintf("http://127.0.0.1:%d/invoke", *port)
	agentCard := &a2a.AgentCard{
		Name:        "A2UI Showcase Agent",
		Description: "An agent powered by Gemini that returns A2UI v1.0 dynamic components.",
		Version:     "1.0.0",
		SupportedInterfaces: []*a2a.AgentInterface{
			a2a.NewAgentInterface(addr, a2a.TransportProtocolJSONRPC),
		},
		DefaultInputModes:  []string{"text"},
		DefaultOutputModes: []string{"text", a2uiMimeType},
		Capabilities: a2a.AgentCapabilities{
			Streaming: true,
			// Advertise A2UI v1.0 support so clients can negotiate the extension.
			Extensions: []a2a.AgentExtension{
				{
					URI:         a2uiExtensionURI,
					Description: "Ability to render A2UI v1.0",
					Required:    false,
					Params: map[string]any{
						"supportedCatalogIds":   []string{basicCatalogID},
						"acceptsInlineCatalogs": false,
					},
				},
			},
		},
		Skills: []a2a.AgentSkill{
			{
				ID:          "ui",
				Name:        "A2UI Generator",
				Description: "Responds to chat messages with A2UI components.",
				Tags:        []string{"chat", "ui"},
				Examples:    []string{"show me a dog"},
			},
		},
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to bind to a port: %v", err)
	}
	logger.Info("server starting", "port", *port, "level", *level)

	mux := http.NewServeMux()
	mux.Handle("/invoke", a2asrv.NewJSONRPCHandler(requestHandler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))

	if err := http.Serve(listener, mux); err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
	}
}
