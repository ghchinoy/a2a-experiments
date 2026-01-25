# A2A Interaction Scenarios

This document provides step-by-step examples of how to use the A2A CLI and Server to perform complex, multi-turn agentic workflows.

## Scenario 1: Deep Research with Continuity

This scenario demonstrates how a single Task can span multiple messages to refine a research goal.

1.  **Start the Research**:
    ```bash
    ./bin/client invoke "Research the history of the Go language" --skill ai_researcher
    ```
    *Note the **Task ID** at the bottom of the output (e.g., `019499de-...`).*

2.  **Refine the Result**:
    ```bash
    ./bin/client invoke "Explain the rationale behind the lack of generics until 1.18" --task <TASK_ID>
    ```
    *Because we passed the `--task` flag, Gemini retains the context of the previous research turn.*

---

## Scenario 2: Cross-Skill Chaining (Artifact Reference)

This scenario shows how one skill (`summarize`) can programmatically consume the "Work Product" (Artifact) of another skill (`ai_researcher`).

1.  **Generate a Report**:
    ```bash
    ./bin/client invoke "Research the A2A protocol standards" --skill ai_researcher
    ```
    *Wait for the `Deep Research Report` artifact to be delivered. Copy the **Task ID**.*

2.  **Summarize the Artifact**:
    ```bash
    # Use --ref (or -r) to point to the COMPLETED task
    ./bin/client invoke "Please summarize" --skill summarize --ref <TASK_ID>
    ```
    *The `summarize` skill doesn't need you to paste the report. It uses the Reference Pattern to look into the previous Task's history, find the artifact, and process it.*

---

## Technical Note: --task vs --ref
*   **`--task` (-k)**: Used to **continue** an active, non-terminal task (e.g., a multi-turn chat session).
*   **`--ref` (-r)**: Used to **reference** a terminal (completed) task as context for a brand new operation.

## Scenario 3: Secure Admin Access

Demonstrates how skills can be gated with authentication.

1.  **Attempt unauthorized access**:
    ```bash
    ./bin/client invoke "Hello" --skill admin_echo
    ```
    *Result: `REJECTED - Unauthorized`*

2.  **Access with a token**:
    ```bash
    ./bin/client invoke "Hello" --skill admin_echo --token "secret-token"
    ```
    *Result: `Admin Admin says: Hello`*

---

## Scenario 4: Natural Language Intent Discovery

Demonstrates the server's ability to route messages based on content.

1.  **Invoke without a skill flag**:
    ```bash
    ./bin/client invoke "I want to research the history of Unix"
    ```
    *The server detects the keyword "research" and automatically dispatches to the `ai_researcher` skill.*
