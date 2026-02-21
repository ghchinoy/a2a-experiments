# A2A Experiments

This project is an experimental blueprint for building stateful, resilient A2A (Agent-to-Agent) services. It demonstrates how to bridge the **A2A Protocol v1.0** with the **Gemini Interactions API** to solve the "Resumption Problem" for long-running autonomous agents.

## 📖 Documentation

*   **[A2A Protocol Overview](docs/a2a_overview.md)**: Understanding the protocol and its interaction flows.
*   **[Agentic Design Patterns](docs/agentic_patterns.md)**: Best practices for long-running tasks, heartbeats, and artifacts.
*   **[Interaction Scenarios](docs/scenarios.md)**: Step-by-step examples of stateful workflows, session continuity, and cross-skill chaining.
*   **[Project Architecture](docs/architecture.md)**: Deep dive into the modular hybrid server and the "Explicit Store Pattern."
*   **[Code Walkthrough](docs/code_walkthrough.md)**: A guided tour of the codebase for Go beginners, mapping concepts to files.
*   **[Authentication & Security](docs/authentication.md)**: Secure remote access and delegated identity in the Google Cloud ecosystem.

## 🛠 Components

- **Hybrid Server**: An A2A + HTTP agent featuring natural language intent detection, auth-gated skills, and a shared persistent TaskStore. Fully compliant with **A2A Protocol v1.0**.
- **Cobra CLI (`a2acli`)**: *(Deprecated)* A professional developer tool for discovering Agent Cards and invoking skills with streaming feedback. **Note: The client in this repo is deprecated. Please use the standalone client at [github.com/ghchinoy/a2acli](https://github.com/ghchinoy/a2acli).**
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

### 2. Install and Use the CLI Client

The local client has been deprecated in favor of a standalone project. Please install the [A2A CLI](https://github.com/ghchinoy/a2acli):

```bash
go install github.com/ghchinoy/a2acli/cmd/a2acli@latest
```

**Discovery & Basic Interaction**:
```bash
a2acli describe --service-url http://127.0.0.1:9001
a2acli invoke "hello" --service-url http://127.0.0.1:9001
```

**Stateful Deep Research (The "Resilience" Pattern)**:
```bash
# 1. Start the task (you can Ctrl+C to detach at any time)
a2acli invoke "Research the history of Unix" --skill ai_researcher --service-url http://127.0.0.1:9001

# 2. Resume to watch progress or retrieve results later
a2acli resume <TASK_ID> --service-url http://127.0.0.1:9001
```

**Cross-Skill Chaining (The "Reference" Pattern)**:
```bash
# Use the Task ID from a completed research mission
a2acli invoke "Summarize the findings" --skill summarize --ref <TASK_ID> --service-url http://127.0.0.1:9001
```

---

## 🏗 The Skill Provider Pattern

To minimize boilerplate and maximize flexibility, this project follows the **Skill Provider Pattern**. This ensures business logic remains transport-agnostic:

1.  **Core Logic**: Transport-unaware Go functions.
2.  **Skill Mapping**: Thin `AgentExecutor` wrappers that handle A2A metadata and streaming.
3.  **Shared Interceptors**: Uniform security and logging across A2A and standard HTTP endpoints.

---

## 📂 Project Structure

- `server/`: Modular A2A server implementation (`main.go`, `skills.go`, `auth.go`).
- `client/`: *(Deprecated)* Legacy Cobra CLI implementation. Use [ghchinoy/a2acli](https://github.com/ghchinoy/a2acli) instead.
- `pkg/interactions/`: Go library for Gemini Interactions API.
- `cmd/itest/`: Interactions API verification tool.
- `docs/`: Architectural documentation, diagrams, and scenarios.