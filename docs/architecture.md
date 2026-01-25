# Project Architecture

This project implements a hybrid A2A/HTTP service and a versatile CLI client, following modern architectural best practices.

## 1. CLI Client (`a2acli`)

The client is built using the **Cobra** CLI framework. It is designed to be a developer tool for inspecting and debugging A2A services.

### Components
*   **Command Router**: Uses Cobra to provide `describe` and `invoke` subcommands.
*   **AgentCard Resolver**: Uses the A2A Go SDK to automatically fetch and parse agent cards from base service URLs.
*   **Interceptors**: Implements a `tokenInterceptor` that dynamically injects Bearer tokens into the A2A metadata stream.

## 2. The Hybrid Server

The server is a multi-modal application that serves both A2A protocol requests and standard HTTP traffic.

### The Skill Provider Pattern
The server is structured to decouple the *logic* of a skill from the *transport* used to access it. This pattern is essential for integrating external logic, such as the `read-aloud` pipeline or the upcoming `Interactions API` library.

*   **Logic Layer**: Business functions (like the Gemini prompt generator) are transport-agnostic and ideally reside in reusable packages.
*   **A2A Layer**: The `AgentExecutor` implements the A2A interface, handling request context, streaming status updates, and delivering artifacts.
*   **HTTP Layer**: A standard `http.ServeMux` hosts both the A2A JSON-RPC handler and traditional REST-style endpoints.

## 3. Interactions API Library (Planned)

To support sophisticated, long-running research tasks, a new Go library will be implemented to wrap the **Gemini Interactions REST API**.

*   **State Management**: Leverage `previous_interaction_id` for multi-turn server-side context.
*   **Background Tasks**: Support `background=true` for deep research operations.
*   **A2A Integration**: The library will be consumed by the `ai_researcher` skill to provide "Agent-grade" research capabilities.

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
*   **Cobra & Pflag**: Provides the robust CLI interface.
*   **Godotenv**: Manages local environment configuration for GCP project settings.

## 4. Interaction Diagram

```mermaid
graph TD
    Client[Cobra CLI] -->|Discovery| WellKnown[/.well-known/agent-card.json]
    Client -->|JSON-RPC| Invoke[/invoke]
    
    subgraph Server [Hybrid A2A Server]
        Invoke --> Handler[A2A Request Handler]
        Handler --> Interceptor[Auth Interceptor]
        Interceptor --> Dispatcher[Skill Dispatcher]
        
        Dispatcher -->|Skill: hello_world| Gemini[Gemini GenAI Client]
        Dispatcher -->|Skill: echo| Echo[Local Logic]
        
        REST[/public/hello] --> Echo
    end
```

## 5. Directory Structure

*   `/server`: Core server logic and A2A service definition.
*   `/client`: Cobra CLI implementation.
*   `/docs`: Detailed architecture and protocol documentation.
*   `/bin`: Compiled binaries.
