package main

const A2UISchema = `
{
  "title": "A2UI Message Schema (v1.0)",
  "description": "Describes a JSON payload for an A2UI v1.0 message, which is used to dynamically construct and update user interfaces. A message MUST set 'version' to 'v1.0' and contain exactly ONE of the action properties: 'createSurface', 'updateComponents', 'updateDataModel', or 'deleteSurface'.",
  "type": "object",
  "properties": {
    "version": { "type": "string", "const": "v1.0" },
    "createSurface": {
      "type": "object",
      "description": "Signals the client to begin rendering a surface. In v1.0 a createSurface MAY carry the initial 'components' and 'dataModel' inline, allowing a complete UI to be built in a single message. 'surfaceId' must be globally unique for the renderer's lifetime; recreating an existing id (without deleting it first) is an error.",
      "properties": {
        "surfaceId": { "type": "string" },
        "catalogId": { "type": "string" },
        "surfaceProperties": {
          "type": "object",
          "description": "Surface properties (e.g. agentDisplayName) defined in the catalog's surfaceProperties schema. Replaces the v0.9 'theme' object; 'primaryColor' has been removed to separate layout from branding."
        },
        "sendDataModel": {
          "type": "boolean",
          "description": "If true, the client returns the full surface data model in the metadata of every message it sends to the surface owner."
        },
        "components": {
          "type": "array",
          "description": "Optional initial component list (same shape as updateComponents.components).",
          "items": {
            "type": "object",
            "properties": {
              "id": { "type": "string" },
              "component": { "type": "string" },
              "weight": { "type": "integer" }
            },
            "required": ["id", "component"],
            "additionalProperties": true
          }
        },
        "dataModel": {
          "type": "object",
          "description": "Optional initial root state of the surface data model."
        }
      },
      "required": ["surfaceId", "catalogId"]
    },
    "updateComponents": {
      "type": "object",
      "properties": {
        "surfaceId": { "type": "string" },
        "components": {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "object",
            "properties": {
              "id": { "type": "string" },
              "component": { "type": "string" },
              "weight": { "type": "integer" }
            },
            "required": ["id", "component"],
            "additionalProperties": true
          }
        }
      },
      "required": ["surfaceId", "components"]
    },
    "updateDataModel": {
      "type": "object",
      "description": "Upserts data at 'path'. In v1.0, setting 'value' to null deletes the key at 'path'. If 'path' is omitted (or '/'), the entire data model is replaced.",
      "properties": {
        "surfaceId": { "type": "string" },
        "path": { "type": "string" },
        "value": {
          "description": "The new value for the path. May be any JSON type (object, array, string, number, boolean, or null)."
        }
      },
      "required": ["value", "surfaceId"]
    },
    "deleteSurface": {
      "type": "object",
      "properties": {
        "surfaceId": { "type": "string" }
      },
      "required": ["surfaceId"]
    }
  }
}
`

const A2UIExamples = `
---BEGIN BASIC_CARD_EXAMPLE---
[
  { "version": "v1.0", "createSurface": { "surfaceId": "default", "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json" } },
  { "version": "v1.0", "updateComponents": {
    "surfaceId": "default",
    "components": [
      { "id": "root", "component": "Column", "children": ["title-heading", "image-card", "action-button"] },
      { "id": "title-heading", "component": "Text", "variant": "body", "text": "A2UI Showcase" },
      { "id": "image-card", "component": "Card", "child": "card-layout" },
      { "id": "card-layout", "component": "Column", "children": ["main-image", "description-text"] },
      { "id": "main-image", "component": "Image", "url": { "path": "imageUrl" } },
      { "id": "description-text", "component": "Text", "text": { "path": "description" } },
      {
        "id": "action-button",
        "component": "Button",
        "child": "button-text-id",
        "action": {
          "event": {
            "name": "button_clicked"
          }
        }
      },
      {
        "id": "button-text-id",
        "component": "Text",
        "text": { "path": "buttonLabel" }
      }
    ]
  } },
  { "version": "v1.0", "updateDataModel": {
    "surfaceId": "default",
    "path": "/",
    "value": {
      "imageUrl": "https://picsum.photos/400/200",
      "description": "This is a dynamically generated A2UI response.",
      "buttonLabel": "Click Me!"
    }
  } }
]
---END BASIC_CARD_EXAMPLE---

---BEGIN SINGLE_MESSAGE_CARD_EXAMPLE---
[
  { "version": "v1.0", "createSurface": {
    "surfaceId": "default",
    "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json",
    "surfaceProperties": { "agentDisplayName": "A2UI Showcase Agent" },
    "components": [
      { "id": "root", "component": "Column", "children": ["title-heading", "summary-text"] },
      { "id": "title-heading", "component": "Text", "variant": "body", "text": { "path": "title" } },
      { "id": "summary-text", "component": "Text", "text": { "path": "summary" } }
    ],
    "dataModel": {
      "title": "Single-Message UI",
      "summary": "This entire surface (components + data) was created in one v1.0 createSurface message."
    }
  } }
]
---END SINGLE_MESSAGE_CARD_EXAMPLE---

---BEGIN FORM_EXAMPLE---
[
  { "version": "v1.0", "createSurface": { "surfaceId": "default", "catalogId": "https://a2ui.org/specification/v1_0/catalogs/basic/catalog.json" } },
  { "version": "v1.0", "updateComponents": {
    "surfaceId": "default",
    "components": [
      { "id": "root", "component": "Column", "children": ["form-heading", "name-input", "date-input", "submit-button"] },
      { "id": "form-heading", "component": "Text", "variant": "body", "text": "User Form" },
      { "id": "name-input", "component": "TextField", "label": "Name", "value": { "path": "userName" } },
      { "id": "date-input", "component": "DateTimeInput", "value": { "path": "userDate" }, "enableTime": false },
      {
        "id": "submit-button",
        "component": "Button",
        "child": "btn-text",
        "action": {
          "event": {
            "name": "submit_form"
          }
        }
      },
      { "id": "btn-text", "component": "Text", "text": "Submit" }
    ]
  } },
  { "version": "v1.0", "updateDataModel": {
    "surfaceId": "default",
    "path": "/",
    "value": {
      "userName": "",
      "userDate": "2024-01-01"
    }
  } }
]
---END FORM_EXAMPLE---

---BEGIN SILENT_STATE_MUTATION_EXAMPLE---
[
  { "version": "v1.0", "updateDataModel": {
    "surfaceId": "default",
    "path": "/",
    "value": {
      "buttonLabel": "Clicked!"
    }
  } }
]
---END SILENT_STATE_MUTATION_EXAMPLE---

---BEGIN HYBRID_FORM_RECEIPT_EXAMPLE---
[
  { "version": "v1.0", "updateDataModel": {
    "surfaceId": "default",
    "path": "/",
    "value": {
      "isSubmitted": true
    }
  } }
]
---END HYBRID_FORM_RECEIPT_EXAMPLE---
`
