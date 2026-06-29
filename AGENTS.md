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
- [ ] Stateful skills yield `a2a.NewSubmittedTask(...)` as the **first** event; stateless skills yield exactly one `a2a.Message`
- [ ] Long-polling loops emit `working` status heartbeats
- [ ] Work products are returned as Artifacts (`a2a.Text` or `a2a.Data` parts), not Messages
- [ ] Client-side transport timeout exceeds the maximum expected skill duration
