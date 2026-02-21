# Gemini CLI: Engineering Operations

This document defines our operational standards, architectural theses, and the "Definition of Done" for building high-authority agentic A2A services.

## Core Operating Principles

1.  **Resilience-First Design**: Always assume the client's network will drop during a long-running mission. Architecture must solve the **Resumption Problem** by default.
2.  **Impedance Matching**: When bridging A2A to external stateful APIs (like Gemini Interactions), formalize the mapping of IDs via the **State Bridge Pattern**.
3.  **Architectural Observability**: Negate the "Black Box" of agentic labor. Every poll of a downstream API must be translated into a transparent A2A `[working]` heartbeat.
4.  **Artifacts as Deliverables**: Distinguish between "Conversation" (Messages) and "Work Products" (Artifacts). If a skill produces a result, it must be delivered as a machine-consumable Artifact.
5.  **Authoritative Documentation**: Documentation in `docs/` must focus on the architectural "Why" (the thesis) rather than just the "How" (the tutorial).

## Design Patterns

### The Stateful Event Sequence (A2A 1.0)
*   **Thesis**: In the A2A 1.0 specification, the very first event yielded by an agent determines if an interaction is stateful (stored, resume-able) or stateless (request-response).
*   **Implementation**: 
    *   For streaming, long-running, or artifact-producing skills, **always** explicitly establish the task in the store by yielding a Task first: `yield(a2a.NewSubmittedTask(execCtx, execCtx.Message), nil)`.
    *   For stateless answers (like simple echoes), yield a single `a2a.Message` and exit. Do not mix status updates with stateless responses.

### The State Bridge Pattern
*   **Thesis**: Maps the A2A `TaskID` (The persistent workspace) to a downstream session ID (e.g., `gemini_interaction_id`).
*   **Implementation**: Store the external ID in A2A task `Metadata`. On subsequent turns (via `--task` or `--ref`), recover the ID to resume the exact session state.

### The Explicit Store Pattern
*   **Requirement**: Never rely on obscure or implicit default memory stores inside the SDK.
*   **Standard**: Explicitly initialize the provided SDK stores (e.g., `taskstore.NewInMemory(...)`) in `main()` and deliberately inject them via `a2asrv.WithTaskStore(...)` and cross-task loaders. This prevents nil-pointer panics and exposes the underlying storage mechanism to developers for easy database swaps later.

### The Reference Pattern (Data Gravity)
*   **Concept**: Use `msg.ReferenceTasks` to allow new tasks to pull the results of completed tasks into their orbit. 
*   **Implementation**: Inject a `ReferencedTasksLoader` as an `ExecutorContextInterceptor` to "hydrate" the current execution context with historical artifacts.

## Engineering Checklist for New Skills

When adding a new A2A skill, the agent must verify:
- [ ] **Streaming Enabled**: Is `Capabilities.Streaming` set to `true` in the Agent Card?
- [ ] **Event Yield Order**: (Stateful) Does the skill yield an `a2a.Task` before yielding any statuses or artifacts? Or (Stateless) does it yield exactly one `a2a.Message`?
- [ ] **Heartbeats Implemented**: Does the logic emit `working` status updates during remote API polling waits?
- [ ] **Artifact Extraction**: Is the "Work Product" returned as a strongly-typed Artifact containing `a2a.Data` or `a2a.Text` parts?
- [ ] **Timeout Alignment**: Does the client-side transport timeout exceed the maximum expected skill duration?

## Issue Tracking (bd)

This project uses **bd (beads)** for issue tracking. 
**Quick reference:**
- `bd ready` - Find unblocked work.
- `bd create "Title" --type task --priority 2` - Create issue.
- `bd close <id>` - Complete work.
- `bd sync` - Sync with git (run at session end).
