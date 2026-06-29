# Agent Instructions

See [GEMINI.md](GEMINI.md) for the full engineering standards, design patterns, and architectural theses for this project.

## Critical Workflow Rules

### Issue Tracking (bd) — follow this order every time

Before touching code, claim the issue:

```bash
bd in-progress <id>   # claim before implementing
```

After implementing:

```bash
bd close <id> --reason "..."   # close when done
bd sync                         # sync at session end
```

Other useful commands:

```bash
bd ready                                      # find unblocked work to pick up
bd create "Title" --type task --priority 2   # create a new issue
```

### Engineering Checklist for New Skills

Before considering a new A2A skill complete, verify all of the following (from GEMINI.md):

- [ ] `Capabilities.Streaming` is `true` in the Agent Card
- [ ] **Stateful skills: `a2a.NewSubmittedTask(...)` is the very first `yield` call** — before any status updates, artifacts, or messages. Missing this causes "first event must be a Task" errors at the client.
- [ ] **Stateful skills: `finalizeTask()` (or equivalent `TaskStateCompleted` event) is called at the end** — missing this causes the client to hang indefinitely waiting for a terminal state.
- [ ] The final artifact sets `LastChunk = true` so the client knows the artifact stream is complete.
- [ ] The skill handler takes `ctx context.Context` (not `_`) so it can be passed to `finalizeTask()` and other context-aware calls.
- [ ] Long-polling loops emit `working` status heartbeats
- [ ] Work products are returned as Artifacts (`a2a.Text` or `a2a.Data` parts), not Messages
- [ ] Client-side transport timeout exceeds the maximum expected skill duration

> **Post-mortem (a2a-simple-voh):** All four items above were caught by the a2acli agent during integration testing of `multimodal_echo`. The fix was 3666aa9.
