# A2UI Showcase Agent Server

This is a sample Agent-to-Agent (A2A) server that demonstrates how to integrate the **A2UI (Agent-to-UI) v1.0** protocol with the A2A 1.0 JSON-RPC protocol. It showcases how an LLM can dynamically generate native client UIs (like forms, buttons, and cards) and react to client-side UI events.

## A2UI v1.0 wire format

This server emits the A2UI **v1.0** wire format:

- **Version:** every envelope sets `"version": "v1.0"`.
- **MIME type:** A2UI `DataPart`s are identified by `metadata.mimeType = "application/a2ui+json"` (the IANA-conformant type; v0.9 used the legacy `application/json+a2ui`).
- **Message list:** a single `DataPart` carries an **ordered array** of A2UI messages in its `data` field (not one `DataPart` per message). Receivers process the list sequentially; atomicity is per-message.
- **`surfaceProperties`:** `createSurface` uses `surfaceProperties` (e.g. `agentDisplayName`) instead of the v0.9 `theme`; `primaryColor` is removed.
- **Single-message instantiation:** `createSurface` may embed the initial `components` and `dataModel` inline (see `SINGLE_MESSAGE_CARD_EXAMPLE`).
- **Catalog id:** `https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json`.
- **Agent Card:** advertises the extension `https://a2ui.org/a2a-extension/a2ui/v1.0` under `capabilities.extensions`, with `params.supportedCatalogIds` and `params.acceptsInlineCatalogs`.

> **Conformance:** validate this server with the manual test plan in
> [`V1_CONFORMANCE_TESTPLAN.md`](./V1_CONFORMANCE_TESTPLAN.md) using `a2acli`.
> Tracked by Apex issue `apex-2xl` and a2acli issue `a2ac-8qf`.

### Not yet implemented (follow-ups)

The wire format and Agent Card are v1.0-conformant. These v1.0 *interactive*
features are intentionally left as follow-ups because they require interactive
server logic beyond LLM template emission:

- Synchronous action responses (`action` with `wantResponse`/`actionId` → `actionResponse`).
- Server-initiated function calls (`callFunction` → `functionResponse` / `error` `INVALID_FUNCTION_CALL`).
- `sendDataModel` client→server data-model synchronization.

## Features

- **Dynamic UI Generation:** The server prompts the LLM to output A2UI v1.0 JSON payloads. These payloads command the Flutter client to render native UI components rather than just flat text.
- **Stateful Context Tracking:** The server uses a `ServerState` struct to securely map the A2A `ContextID` to a dynamically generated `surfaceId`. This ensures that subsequent updates resulting from a user's interaction correctly target the proper historical card in the chat feed instead of causing state collisions.
- **Client Event Routing:** When a user interacts with a generated UI component (e.g., clicks a submit button), the client sends an `action` payload back to the server. The server intercepts this data and seamlessly injects it into the LLM's context, allowing the agent to respond to UI state changes conversationally.
- **Strict Schema Compliance:** The agent is guided by embedded A2UI schema rules and examples (like `BASIC_CARD_EXAMPLE`, `SINGLE_MESSAGE_CARD_EXAMPLE`, and `FORM_EXAMPLE`) to ensure the generated UI matches the A2UI v1.0 adjacency-list format.

## Architecture

The following diagram illustrates the flow of A2UI payloads and UI events between the client and the LLM:

![Architecture Flow](architecture.webp)

1. The **User** sends a natural language prompt (e.g., "Show me a form").
2. The **A2A Server** forwards the prompt to **Gemini**, along with strict A2UI JSON schema instructions.
3. **Gemini** generates a response containing raw A2UI JSON.
4. The **A2A Server** parses the JSON, packages it into an A2A `DataPart`, and streams an `ArtifactUpdate` to the client.
5. The **Flutter Client** intercepts the payload, resolves the GenUI components, and renders the native UI.
6. When the **User** interacts with the UI (e.g., button click), the client dispatches an `A2AMessage` containing a `userAction` DataPart.
7. The **A2A Server** parses this action and injects a hidden system event into the context for the next turn, closing the loop.

## Usage

```bash
# Requires .env file with GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION
go run main.go --port 9005
```
