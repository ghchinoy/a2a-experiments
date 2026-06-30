# A2UI v1.0 Conformance — Manual Test Plan

This plan validates that the **`a2a_a2ui` sample server** emits a conformant
**A2UI v1.0** stream and that **`a2acli`** can detect conformance (and
non-conformance) at the transport/wire level — **without a Flutter renderer**.

It supports two issues:

- Apex: `apex-2xl` — *Migrate a2a_a2ui Go sample server to A2UI v1.0 wire format*
- a2acli: `a2ac-8qf` — *Add A2UI v1.0 extension conformance validation (and report it)*

> **Why no renderer?** A2UI is transport-decoupled and rides inside A2A
> `DataPart`s. The byte-level contract can be fully verified with an A2A client
> plus JSON-Schema validation. The Flutter/genui renderer work only begins once
> this plan passes.

---

## 0. Prerequisites

| Tool | Source | Check |
|------|--------|-------|
| `a2acli` | `../a2acli` (or `brew install ghchinoy/tap/a2acli`) | `a2acli version` |
| Go toolchain | go.dev | `go version` |
| `jq` | `brew install jq` | `jq --version` |
| A2UI v1.0 JSON schemas | `a2ui-project/a2ui` `specification/v1_0/json/` | cloned/vendored locally |
| A schema validator | `check-jsonschema` (`pipx install check-jsonschema`) or `ajv-cli` | `check-jsonschema --version` |

Set convenience vars (adjust paths/ports):

```bash
export SVC=http://localhost:9005
export A2UI_SPEC=$HOME/projects/a2ui/specification/v1_0   # local clone of a2ui-project/a2ui
export OUT=/tmp/a2ui_conf && mkdir -p "$OUT"
```

Start the server:

```bash
cd cmd/a2ui
go run .        # serves on :9005 by default
```

---

## 1. Agent Card advertises the A2UI v1.0 extension

**Goal:** the AgentCard declares the extension and its capability params.

```bash
a2acli discover --service-url "$SVC"
# raw card for assertions:
a2acli discover --service-url "$SVC" -n --json 2>/dev/null | tee "$OUT/card.json" | jq '.capabilities.extensions'
```

**PASS criteria:**

- [ ] `capabilities.extensions[]` contains an entry with
      `uri == "https://a2ui.org/a2a-extension/a2ui/v1.0"`.
- [ ] That entry has `params.supportedCatalogIds` (array of strings).
- [ ] `params.acceptsInlineCatalogs` is present (boolean).

Quick check:

```bash
jq -e '.capabilities.extensions[]
        | select(.uri=="https://a2ui.org/a2a-extension/a2ui/v1.0")
        | .params.supportedCatalogIds' "$OUT/card.json"
```

---

## 2. Streamed DataParts use the v1.0 envelope

**Goal:** server output is `application/a2ui+json`, a **list** of messages, each
`version: "v1.0"`.

```bash
a2acli send "show me the showcase card" --service-url "$SVC" -n --wait \
  | tee "$OUT/resp.json" | jq '.'
```

Extract the A2UI DataPart(s):

```bash
# Adjust the filter to the actual a2acli JSON shape (artifacts[].parts[] / message.parts[])
jq '[.. | objects | select(.kind=="data" and (.metadata.mimeType? // "" | test("a2ui")))]' \
   "$OUT/resp.json" | tee "$OUT/dataparts.json" | jq 'length'
```

**PASS criteria:**

- [ ] At least one DataPart has `metadata.mimeType == "application/a2ui+json"`
      (NOT the legacy `application/json+a2ui`).
- [ ] Each such DataPart's `data` is a **JSON array** (not a single object).
- [ ] Every message envelope in `data[]` has `"version": "v1.0"`.
- [ ] Exactly one envelope key per message
      (`createSurface` | `updateComponents` | `updateDataModel` | `deleteSurface`
      | `callFunction` | `actionResponse`).
- [ ] Exactly one component across the stream has `id == "root"`.

Assertions:

```bash
jq -e 'all(.[]; .metadata.mimeType=="application/a2ui+json")' "$OUT/dataparts.json"
jq -e 'all(.[]; (.data|type)=="array")' "$OUT/dataparts.json"
jq -e 'all(.[].data[]; .version=="v1.0")' "$OUT/dataparts.json"
```

---

## 3. Schema validation against the official v1.0 schemas

**Goal:** each server→client message validates against
`server_to_client_list.json`.

```bash
# Flatten all message lists into individual envelopes and validate the lists.
jq '[.[].data]' "$OUT/dataparts.json" > "$OUT/message_lists.json"
119: 
120: # Validate each captured list (one per DataPart) against the list schema.
121: jq -c '.[]' "$OUT/message_lists.json" | nl -ba | while read -r n list; do
122:   echo "$list" > "$OUT/list_$n.json"
123:   check-jsonschema --schemafile "$A2UI_SPEC/json/server_to_client_list.json" "$OUT/list_$n.json" \
124:     && echo "list $n: PASS" || echo "list $n: FAIL"
125: done
```

**PASS criteria:**

- [ ] Every captured list validates against `server_to_client_list.json`.
- [ ] If the server ships an inline/custom catalog, it validates against
      `catalog_definition.json`, and all entity names match UAX #31
      (`^[\p{XID_Start}_][\p{XID_Continue}]*$`).

---

## 4. Decoupled branding (`surfaceProperties`, no `primaryColor`)

```bash
jq '[.. | objects | select(has("createSurface")) | .createSurface]' "$OUT/dataparts.json" \
  > "$OUT/creates.json"; jq '.' "$OUT/creates.json"
```

**PASS criteria:**

- [ ] No `theme` key on any `createSurface`.
- [ ] `surfaceProperties` is used where surface metadata is set.
- [ ] No `primaryColor` anywhere.

---

## 5. Single-message UI instantiation

**Goal:** a `createSurface` can carry inline `components` + `dataModel`.

**PASS criteria:**

- [ ] At least one `createSurface` includes a non-empty `components` array AND a
      `dataModel` object, producing a complete UI in one message.
- [ ] That `components` array contains an `id: "root"` entry.

```bash
jq -e 'any(.[]; .components and .dataModel and (any(.components[]; .id=="root")))' "$OUT/creates.json"
```

---

## 6. Synchronous action response (`actionId` / `actionResponse`)

**Goal:** server answers a client action that set `wantResponse: true`.

Drive a client→server action with a `DataPart` (adjust to a2acli's data-part flag
once `a2ac-79d` lands; until then use `--json`/`--data`):

```bash
a2acli send --service-url "$SVC" -n --wait --data '{
  "data":[{"version":"v1.0","action":{
    "name":"get_typeahead_suggestions","surfaceId":"contact_form_1",
    "sourceComponentId":"myinput","context":{"prefix":"app"},
    "wantResponse":true,"actionId":"ta_1"}}],
  "kind":"data","metadata":{"mimeType":"application/a2ui+json"}}' \
  | tee "$OUT/actionresp.json" | jq '.'
```

**PASS criteria:**

- [ ] Response contains an `actionResponse` whose `actionId == "ta_1"`.
- [ ] Exactly one of `value` / `error` is present in the `actionResponse`.

---

## 7. Server-initiated function call (`callFunction`)

**Goal:** server can request a client-side function; boundaries enforced at runtime.

**PASS criteria (server side of contract):**

- [ ] Server can emit `callFunction` with `functionCallId`, optional
      `wantResponse`, and `{call, args}`.
- [ ] Server accepts a client `functionResponse` echoing `functionCallId`
      verbatim, OR an `error` with `code: "INVALID_FUNCTION_CALL"` for
      `clientOnly`/unregistered functions.

> Full execution is renderer-side; here we only confirm the server emits a
> schema-valid `callFunction` and tolerates both reply shapes. `a2acli` may
> simulate either reply.

---

## 8. Surface uniqueness

**PASS criteria:**

- [ ] A second `createSurface` for an already-active `surfaceId` (without an
      intervening `deleteSurface`) is rejected/flagged as an error by the server
      logic (and would be by a conformant client).

---

## 9. Negative tests (the validator must catch these)

Temporarily toggle the server (or hand-craft payloads) to confirm `a2acli`'s
A2UI validation FAILS on:

- [ ] Legacy MIME `application/json+a2ui`.
- [ ] `data` as a single object instead of an array.
- [ ] `version: "v0.9"`.
- [ ] `theme`/`primaryColor` present on `createSurface`.
- [ ] A component name violating UAX #31 (e.g. `1stItem`, `submit-form`).
- [ ] Two `createSurface` with the same `surfaceId`.

---

## 10. Conformance report

After §1–§9 pass against the sample server:

```bash
cd ../../../a2acli      # or absolute path to ../a2acli
make conformance-report     # should now include an "A2UI Extension v1.0" block
```

**PASS criteria:**

- [ ] `docs/CONFORMANCE_REPORT.md` shows an **A2UI Extension v1.0: PASSING** block
      generated against this sample server as SUT.

---

## Result summary template

| # | Check | Result | Notes |
|---|-------|--------|-------|
| 1 | Agent Card extension | ☐ | |
| 2 | DataPart wire format | ☐ | |
| 3 | Schema validation | ☐ | |
| 4 | surfaceProperties / no primaryColor | ☐ | |
| 5 | Single-message instantiation | ☐ | |
| 6 | actionResponse round-trip | ☐ | |
| 7 | callFunction contract | ☐ | |
| 8 | Surface uniqueness | ☐ | |
| 9 | Negative tests caught | ☐ | |
| 10 | Conformance report updated | ☐ | |

> `jq`/filter expressions above assume a particular a2acli JSON shape; adjust the
> selectors to the actual `--json`/`-n --wait` output. The intent of each
> assertion is normative; the exact path is not.
