# Simple A2A Example

This project is an authoritative blueprint for building stateful, resilient A2A (Agent-to-Agent) services. It demonstrates how to bridge the **A2A Protocol** with the **Gemini Interactions API** to solve the "Resumption Problem" for long-running autonomous agents.

## 📖 Documentation

*   **[A2A Protocol Overview](docs/a2a_overview.md)**: Understanding the protocol and its interaction flows.
*   **[Agentic Design Patterns](docs/agentic_patterns.md)**: Best practices for long-running tasks, heartbeats, and artifacts.
*   **[Interaction Scenarios](docs/scenarios.md)**: Step-by-step examples of stateful workflows, session continuity, and cross-skill chaining.
*   **[Project Architecture](docs/architecture.md)**: Deep dive into the modular hybrid server and the "Explicit Store Pattern."
*   **[Authentication & Security](docs/authentication.md)**: Secure remote access and delegated identity in the Google Cloud ecosystem.

## 🛠 Components

- **Hybrid Server**: An A2A + HTTP agent featuring natural language intent detection, auth-gated skills, and a shared persistent TaskStore.
- **Cobra CLI (`a2acli`)**: A professional developer tool for discovering Agent Cards and invoking skills with streaming feedback.
- **Interactions Library**: A custom Go implementation of the Gemini Interactions REST API, supporting stateful turns and background execution.
- **`itest` Utility**: A direct CLI tool for verifying Gemini Interactions API behavior independent of the A2A layer.

## 🚀 Prerequisites

- **Go 1.24+**
- **Google Cloud Platform**: Access to Vertex AI (verified via `gcloud auth application-default login`).
- **Gemini API Key**: Required for stateful research and chat features.

### Configuration

1.  Copy the example environment file:
    ```bash
    cp .env.example .env
    ```
2.  Edit `.env` and provide your `GOOGLE_CLOUD_PROJECT` and `GEMINI_API_KEY`.

## ⚡️ Quick Start

### 1. Build and Start the Server

```bash
go build -o bin/server ./server
./bin/server
```
The server starts on `127.0.0.1:9001`.

### 2. Use the CLI Client

**Discovery & Basic Interaction**:
```bash
./bin/client describe
./bin/client invoke "hello"
```

**Stateful Deep Research (The "Resilience" Pattern)**:
```bash
# 1. Start the task (you can Ctrl+C to detach at any time)
./bin/client invoke "Research the history of Unix" --skill ai_researcher

# 2. Resume to watch progress or retrieve results later
./bin/client resume <TASK_ID>
```

**Cross-Skill Chaining (The "Reference" Pattern)**:
```bash
# Use the Task ID from a completed research mission
./bin/client invoke "Summarize the findings" --skill summarize --ref <TASK_ID>
```

---

## 🏗 The Skill Provider Pattern

To minimize boilerplate and maximize flexibility, this project follows the **Skill Provider Pattern**. This ensures business logic remains transport-agnostic:

1.  **Core Logic**: Transport-unaware Go functions.
2.  **Skill Mapping**: Thin `AgentExecutor` wrappers that handle A2A metadata and streaming.
3.  **Shared Interceptors**: Uniform security and logging across A2A and standard HTTP endpoints.

---

## 📂 Project Structure

- `server/`: Modular A2A server implementation (`main.go`, `skills.go`, `store.go`, `auth.go`).
- `client/`: Cobra CLI implementation.
- `pkg/interactions/`: Go library for Gemini Interactions API.
- `cmd/itest/`: Interactions API verification tool.
- `docs/`: Architectural documentation, diagrams, and scenarios.