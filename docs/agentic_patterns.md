# Agentic Design Patterns in A2A

A2A is uniquely suited for "Agentic" workloads—tasks that are long-running, multi-step, and produce structured deliverables rather than just simple text responses.

## 1. Handling Long-Running Tasks

In standard REST, a long-running request often leads to a timeout. A2A solves this using three primary interaction patterns:

### Pattern A: Live Streaming
The client opens a streaming connection (`SendStreamingMessage`). The server emits `TaskStatusUpdateEvent` heartbeats.
*   **Use Case**: Real-time feedback in a UI or CLI.
*   **Pros**: Low latency, immediate user feedback.
*   **Cons**: Brittle over unstable networks for very long tasks (> 5 mins).

### Pattern B: Asynchronous Resubscription
The server immediately returns a `TaskID` and moves the task to a `working` state. The client can safely disconnect.
*   **Workflow**: The client reconnects later using `ResubscribeToTask(taskId)` (exposed via the `resume` CLI command).
*   **Use Case**: Deep research, video rendering, or complex data processing.
*   **Pros**: Highly robust; survives network drops; allows "passive observation" of locked tasks.

### Pattern C: Push Notifications (Webhooks)
The client provides a `PushConfig` (URL and Token). The server pushes the final `Task` and `Artifacts` to the client when finished.
*   **Use Case**: Multi-agent orchestration where one agent triggers another and waits for a callback.

---

## 2. Messages vs. Artifacts

A2A distinguishes between **Human-readable conversation** and **Machine-readable results**.

| Feature | Message (`a2a.Message`) | Artifact (`a2a.Artifact`) |
| :--- | :--- | :--- |
| **Primary Audience** | Humans / Chat Interfaces | Software Agents / Programmatic Tools |
| **Content** | `TextPart` (Conversation) | `DataPart` (JSON) or `FilePart` (MP3/PDF) |
| **Lifecycle** | Transient part of history | Permanent deliverable of the Task |
| **Example** | "I've finished your research report." | `report.json` or `summary.pdf` |

**Design Tip**: Always return the "Product" of your skill as an **Artifact**. This allows downstream agents to use the data without needing to use an LLM to "scrape" your text responses.

---

## 3. Negotiating Timeouts

Long-running tasks often exceed standard HTTP or gRPC timeouts (typically 30-60 seconds). A2A provides several ways to "negotiate" or communicate these requirements:

### Human-Readable Hints
The simplest way is to include duration expectations in the `AgentSkill` description within the Agent Card.
*   **Example**: "Performs deep research. Note: This process typically takes 5-10 minutes."
*   **Result**: A developer or a sophisticated agent can see this and manually increase the client-side timeout.

### A2A Extensions (Programmatic)
For automated negotiation, you can use **A2A Extensions**. You can define a custom URI (e.g., `https://a2a.dev/extensions/v1/expected-duration`) in the `Capabilities.Extensions` list.
*   **Server-side**: Declare the extension and provide a parameter like `max_timeout_seconds: 600`.
*   **Client-side**: Programmatically inspect the Agent Card for this extension and adjust the underlying `http.Client` or gRPC deadline automatically.

### Dynamic Progress Estimates
While a task is `working`, the server can send a `TaskStatusUpdateEvent` with metadata containing an `estimated_completion_at` timestamp. This allows the client to dynamically decide whether to stay connected or switch to the **Resubscription Pattern**.

---

## 4. Stateful Chaining and Cross-Skill Tasks

A2A allows for sophisticated multi-turn workflows where the output of one step becomes the context for the next. This is achieved through **Task Continuity**.

### Session Continuity (Intra-Skill)
A client can "continue" a task by sending a new message that includes the `TaskID` of a previous response.
*   **The Workflow**: 
    1. Turn 1: "Research Go history." -> Server returns `TaskID: abc`.
    2. Turn 2: "Summarize that." + `TaskID: abc`.
*   **Implementation**: The server sees the `TaskID`, retrieves the previous session state (e.g., the Gemini `InteractionID`), and continues the conversation with full context.

### Cross-Skill Task Sharing
A2A allows a message to target a *different* skill while still referencing an existing `TaskID`.
*   **The Workflow**: 
    1. You use the `ai_researcher` skill to generate a deep report (Task #123).
    2. You then call the `summarize` skill, referencing Task #123.
*   **Data Retrieval**: The `summarize` skill doesn't need the report text in its own message; it can "look back" into Task #123 to find the `Research Report` artifact and process it directly.

### The "Reference" Pattern
A2A Messages also support a `referenceTaskIds` field. This allows an agent to pull context from *multiple* previous tasks to perform a new action (e.g., "Summarize the findings from Task A and Task B into a single presentation").

---

## 5. Real-World Examples

### A. Deep Research (Interactions API)
This project uses the Gemini Interactions API to perform multi-minute research tasks.
1.  **Initiation**: Client calls the `ai_researcher` skill.
2.  **Server-Side Polling**: The A2A server starts a background interaction and polls the Gemini API every 10 seconds.
3.  **Heartbeats**: Each poll result is streamed back to the A2A client as a `TaskStatusUpdateEvent` (e.g., `[working] - Status: in_progress`).
4.  **Artifact Delivery**: Once Gemini finishes, the server extracts the long-form report and returns it as a named **Artifact** for structured interpretation.

### B. Audio Synthesis (Read-Aloud Service)
A planned integration where a long-running pipeline converts documents to speech.
1.  **Multi-Modal Input**: Client sends a PDF or URL via `a2a.Message` parts.
2.  **Pipeline Mapping**: 
    *   `extracting` -> `TaskStatusUpdateEvent`
    *   `synthesizing` -> `TaskStatusUpdateEvent` (with % completion)
3.  **Final Deliverable**: The server produces an MP3 file and delivers it as an **Artifact** using a `FileURI` or `FileBytes` part.
