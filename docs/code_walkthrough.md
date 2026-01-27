# Code Walkthrough: A Guided Tour for Go Beginners

This document is designed to help you navigate the `a2a-experiments` codebase. It maps the high-level concepts (A2A Protocol, Agentic Patterns) to the specific Go files and functions that implement them.

## 1. The Big Picture

Before diving into files, remember the core flow:
1.  **Client** sends a message (JSON-RPC).
2.  **Server** authenticates it.
3.  **Server** dispatches it to a specific **Skill** (function).
4.  **Skill** does work (maybe calling Gemini).
5.  **Skill** streams updates back to the Client.

## 2. The Server (`server/`)

The server is the heart of the agent. It is built using the standard `net/http` package and the `a2asrv` SDK.

### Entry Point: `server/main.go`
This file is the "wiring diagram." It doesn't contain business logic; it just connects components.

*   **`agentExecutor` struct**: This holds the dependencies (Gemini client, Interactions client) that your skills need.
*   **`a2asrv.NewHandler`**: This creates the A2A protocol handler. Notice the **Explicit Store Pattern**:
    ```go
    store := newMemStore()
    requestHandler := a2asrv.NewHandler(
        executor,
        a2asrv.WithTaskStore(store), // We explicitly inject the store so we can share it
        // ...
    )
    ```

### Business Logic: `server/skills.go`
This is where the actual work happens. Each function here corresponds to a "Skill" in the Agent Card.

*   **`Execute` method**: This is the "Router." It looks at the user input and decides which function to call (`handleHelloWorld`, `handleStatefulInteraction`, etc.).
*   **`handleStatefulInteraction`**: This is the most complex skill. It shows how to:
    1.  **Check for Recovery**: Look for `gemini_interaction_id` in metadata.
    2.  **Call External API**: Use `interactionsClient` to talk to Google.
    3.  **Stream Updates**: Use `q.Write(ctx, event)` to send heartbeats back to the user.

### Security: `server/auth.go`
Refer to `docs/auth_flow.dot`. This file implements an **Interceptor**.

*   **`authInterceptor`**: It sits *before* the handler.
*   **`Before` method**: It checks the `Authorization` header. If valid, it populates `callCtx.User`. This allows skills (like `handleAdminEcho`) to check `callCtx.User.Authenticated()` later.

## 3. The Client (`client/`)

The client is a CLI tool built with **Cobra** (for commands) and **Bubble Tea** (for the UI).

### The Command Structure: `client/main.go`
*   **`invokeCmd`**: The main command. It sets up the A2A client.
*   **`resumeCmd`**: The resilience command. Notice how it calls `GetTask` first to check if the task is already done, handling the "Resumption Problem" we discuss in the blog post.

### The UI Loop: `client/tui.go`
If you are new to Go, this might look strange. It uses the **Model-View-Update (ELM)** architecture.

*   **`model` struct**: Holds the state (messages, spinner, status).
*   **`Init`**: Starts the `waitForActivity` goroutine.
*   **`Update`**: The main event loop. It handles incoming `streamMsg` (from the server) and updates the model.
*   **`View`**: Renders the string that appears in your terminal.

## 4. The Library (`pkg/interactions/`)

This acts as a "Driver" for the Gemini Interactions API.

*   **`client.go`**: Contains the HTTP logic.
*   **`types.go`**: Defines the JSON structures that match the Gemini API.

## Key Takeaways for Reading

*   **Interfaces**: We rely heavily on interfaces (`AgentExecutor`, `TaskStore`). This makes testing easier.
*   **Context**: You'll see `ctx context.Context` everywhere. In Go, this carries deadlines, cancellation signals, and request-scoped values (like User Identity).
*   **Channels**: The Client uses Go channels (`<-chan streamMsg`) to pipe data from the network layer to the UI layer safely.
