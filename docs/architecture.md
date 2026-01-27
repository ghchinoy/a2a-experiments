# Project Architecture

This project implements a hybrid A2A/HTTP service and a versatile CLI client, following modern architectural best practices.

## 1. CLI Client (`a2acli`)

The client is built using the **Cobra** CLI framework and enhanced with **Bubble Tea** for a rich terminal user interface (TUI).

### Key Features
*   **Command Router**: Provides `describe`, `invoke`, and `resume` subcommands.
*   **Rich TUI**: Uses `bubbletea` and `lipgloss` to display streaming status, text wrapping, and spinners during long-running tasks.
*   **Resilience (The "Resume" Pattern)**: Implements `client.ResubscribeToTask` via the `resume` command, allowing users to detach (Ctrl+C) and re-attach to active tasks without interrupting the server-side process.
*   **Artifact Management**: Automatically parses `DataPart` and `FilePart` outputs, with support for saving full content to disk via `--out-dir`.
*   **AgentCard Resolver**: Automatically discovers service capabilities from the `/.well-known/agent-card.json` endpoint.

## 2. The Hybrid Server

The server is a multi-modal application that serves both A2A protocol requests and standard HTTP traffic.

### The Skill Provider Pattern
The server decouples the *logic* of a skill from the *transport*.

*   **Logic Layer**: Business functions (like the Gemini prompt generator or Interactions client) are transport-agnostic.
*   **A2A Layer**: The `AgentExecutor` implements the A2A interface, handling request context, streaming status updates, and delivering artifacts.
*   **HTTP Layer**: A standard `http.ServeMux` hosts both the A2A JSON-RPC handler and traditional REST-style endpoints.

## 3. Interactions API Library (`pkg/interactions`)

The project includes a custom Go library that wraps the **Gemini Interactions REST API**. This library enables the `ai_researcher` skill to perform long-running reasoning tasks.

*   **State Management**: Manages `previous_interaction_id` to maintain multi-turn context on the Gemini server, reducing client-side token overhead.
*   **Background Execution**: Uses `background=true` to allow the model to "think" and research asynchronously without blocking the A2A handler.
*   **Polling & Observability**: Implements a polling loop that translates Gemini's internal state into A2A `TaskStatusUpdateEvents` (Heartbeats).

## 4. Persistence & TaskStore

The **`TaskStore`** is the source of truth for all A2A Tasks. It persists the task's state, message history, and artifacts.

### The Explicit Store Pattern
By default, the A2A SDK creates an internal, private memory store. However, for advanced features like **Cross-Task Referencing** (`--ref`), multiple components must share the same data.

*   **The Handler**: Needs the store to save new messages and task updates.
*   **The Reference Loader**: Needs the store to look up *previous* tasks by their ID.

**Best Practice**: Always initialize a `TaskStore` explicitly at the top of your `main()` function and inject it into both the `RequestHandler` and any `RequestContextInterceptors`. This ensures that all components are looking at the same "workspace."

## 5. Technology Stack

*   **A2A Go SDK**: The foundation for protocol handling, card generation, and JSON-RPC serving.
*   **Go GenAI SDK**: Powers the `hello_world` skill by connecting to **Vertex AI Gemini**.
*   **Cobra & Pflag**: Provides the robust CLI command structure.
*   **Bubble Tea & Lip Gloss**: Powers the interactive, resilient CLI TUI.
*   **Godotenv**: Manages local environment configuration.

## 6. Interaction Diagram

```mermaid
graph TD
    Client[Cobra CLI] -->|Discovery| WellKnown[/.well-known/agent-card.json]
    Client -->|JSON-RPC| Invoke[/invoke]
    Client -->|JSON-RPC| Resume[/resume]
    
    subgraph Server [Hybrid A2A Server]
        Invoke --> Handler[A2A Request Handler]
        Resume --> Handler
        Handler --> Interceptor[Auth Interceptor]
        Interceptor --> Dispatcher[Skill Dispatcher]
        
        Dispatcher -->|Skill: hello_world| Gemini[Gemini GenAI Client]
        Dispatcher -->|Skill: ai_researcher| InteractionsLib[pkg/interactions]
        InteractionsLib -->|REST| GoogleGemini[Google Gemini API]
        
        REST[/public/hello] --> Echo[Local Logic]
    end
```

## 7. Directory Structure

*   `/server`: Core server logic and A2A service definition.
*   `/client`: Cobra CLI with Bubble Tea TUI.
*   `/pkg/interactions`: Custom library for Gemini Interactions API.
*   `/docs`: Detailed architecture, patterns, and scenarios.
*   `/bin`: Compiled binaries.