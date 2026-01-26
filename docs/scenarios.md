# A2A Interaction Scenarios

This document provides step-by-step examples of how to use the A2A CLI and Server to perform complex, multi-turn agentic workflows.

## Scenario 1: Deep Research with Continuity

This scenario demonstrates how a single Task can span multiple messages to refine a research goal.

1.  **Start the Research**:
    ```bash
    ./bin/client invoke "Research the history of the Go language" --skill ai_researcher
    ```
    *Note the **Task ID** at the bottom of the output (e.g., `019499de-...`). You can now safely Ctrl+C to detach.*

2.  **Resume Observation**:
    ```bash
    ./bin/client resume <TASK_ID>
    ```
    *This connects to the existing stream to show progress without interrupting the agent.*

3.  **Follow Up (After Completion)**:
    ```bash
    ./bin/client invoke "Explain the rationale behind the lack of generics until 1.18" --task <TASK_ID>
    ```
    *Once the task is COMPLETED, you can use `invoke --task` to send a new message with context.*

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

---

## Scenario 5: The Evolution of Discovery (Meta-Research)

This scenario performs a "Meta-Research" task, using the agent to analyze the evolution of the very technologies it is built upon: gRPC, xDS, and A2A.

1.  **Initiate the Deep Dive**:
    ```bash
    ./bin/client invoke "Analyze the evolution of service discovery from DNS to gRPC xDS, and explain how A2A's 'Semantic Discovery' differs from them." --skill ai_researcher
    ```

2.  **Observe the Process**:
    *   Watch as the agent breaks this down into sub-steps (e.g., "Researching Envoy xDS," "Analyzing Agent Card schema").
    *   This highlights the **Interactions API's** ability to plan complex, multi-step comparison tasks.

3.  **Retrieve the Artifact**:
    *   The final output will be a structured markdown report comparing **Technical Discovery** (IP/Port) vs. **Semantic Discovery** (Capabilities/Intent).

---

## Appendix: Recording Scenarios

To capture these scenarios for documentation or blog posts, we use `asciinema` and `agg` (Asciinema Gif Generator).

### Prerequisites
```bash
# via Homebrew
brew install asciinema agg

# OR via Cargo
cargo install --locked --git https://github.com/asciinema/asciinema
cargo install --git https://github.com/asciinema/agg
```

### Recording Workflow

1.  **Start Recording**:
    ```bash
    asciinema rec demo_research.cast
    ```

2.  **Perform the Scenario**:
    Run the client commands as described above. Type slowly and clearly.

3.  **End Recording**:
    Press `Ctrl+D` or type `exit`.

4.  **Generate GIF**:
    Use `agg` to render the `.cast` file into a high-quality GIF with a nice theme.
    ```bash
    agg --theme monokai demo_research.cast demo_research.gif
    ```

5.  **Embed**:
    Add the GIF to your markdown using standard image syntax: `![Demo of Deep Research](demo_research.gif)`



#### Example

Begin research

```bash
./bin/client invoke "Analyze the evolution of service discovery from DNS to gRPC xDS, and explain how A2A's 'Semantic Discovery' differs from them." --skill ai_researcher -u http://localhost:9002
```

Resume (with out dir)

```bash
./bin/client resume  019bfb36-79b0-780e-9e58-8fa8e1d279f8 -u http://localhost:9002 
```

Chain

```bash
./bin/client invoke "Summarize this for a slide deck" --skill summarize --ref 019bfb36-79b0-780e-9e58-8fa8e1d279f8 -u http://localhost:9002
```