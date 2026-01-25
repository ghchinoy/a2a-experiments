# Interactions API

Google Gemini Interactions API

Docs: https://ai.google.dev/gemini-api/docs/interactions

Deep Research with Interactions: https://ai.google.dev/gemini-api/docs/deep-research
https://ai.google.dev/gemini-api/docs/deep-research.md.txt

API Reference: https://ai.google.dev/static/api/interactions.md.txt

OAS: https://ai.google.dev/static/api/interactions.openapi.json

## Overview

How the Interactions API works
The Interactions API is designed around a central resource: the Interaction. An Interaction represents a complete turn in a conversation or task. It acts as a session record, containing the entire history of an interaction, including all user inputs, model thoughts, tool calls, tool results, and final model outputs.

When you make a call to interactions.create, you are creating a new Interaction resource.

Optionally, you can use the id of this resource in a subsequent call using the previous_interaction_id parameter to continue the conversation. The server uses this ID to retrieve the full context, saving you from having to resend the entire chat history. This server-side state management is optional; you can also operate in stateless mode by sending the full conversation history in each request.

Data storage and retention
By default, all Interaction objects are stored (store=true) in order to simplify use of server-side state management features (with previous_interaction_id), background execution (using background=true) and observability purposes.

Paid Tier: Interactions are retained for 55 days.
Free Tier: Interactions are retained for 1 day.
If you do not want this, you can set store=false in your request. This control is separate from state management; you can opt out of storage for any interaction. However, note that store=false is incompatible with background=true and prevents using previous_interaction_id for subsequent turns.

You can delete stored interactions at any time using the delete method found in the API Reference. You can only delete interactions if you know the interaction ID.

After the retention period expires, your data will be deleted automatically.

Interactions objects are processed according to the terms.

Best practices
Cache hit rate: Using previous_interaction_id to continue conversations allows the system to more easily utilize implicit caching for the conversation history, which improves performance and reduces costs.
Mixing interactions: You have the flexibility to mix and match Agent and Model interactions within a conversation. For instance, you can use a specialized agent, like the Deep Research agent, for initial data collection, and then use a standard Gemini model for follow-up tasks such as summarizing or reformatting, linking these steps with the previous_interaction_id.



## General use

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Tell me a short joke about programming."
}'
```

## Stateful conversation

```curl
# 1. First turn
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Hi, my name is Phil."
}'

# 2. Second turn (Replace INTERACTION_ID with the ID from the previous interaction)
# curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
# -H "Content-Type: application/json" \
# -H "x-goog-api-key: $GEMINI_API_KEY" \
# -d '{
#     "model": "gemini-3-flash-preview",
#     "input": "What is my name?",
#     "previous_interaction_id": "INTERACTION_ID"
# }'
```

### retrieve past stateful interactions

```curl
curl -X GET "https://generativelanguage.googleapis.com/v1beta/interactions/<YOUR_INTERACTION_ID>" \
-H "x-goog-api-key: $GEMINI_API_KEY"
```


## Stateless converation

```curl
 curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
 -H "Content-Type: application/json" \
 -H "x-goog-api-key: $GEMINI_API_KEY" \
 -d '{
    "model": "gemini-3-flash-preview",
    "input": [
        {
            "role": "user",
            "content": "What are the three largest cities in Spain?"
        },
        {
            "role": "model",
            "content": "The three largest cities in Spain are Madrid, Barcelona, and Valencia."
        },
        {
            "role": "user",
            "content": "What is the most famous landmark in the second one?"
        }
    ]
}'
```

## Multimodal capabilities

### Image understanding

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-H "Content-Type: application/json" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": [
    {
        "type": "text",
        "text": "Describe the image."
    },
    {
        "type": "image",
        "uri": "YOUR_URL",
        "mime_type": "image/png"
    }
    ]
}'
```

### Multimodal generation

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-pro-image-preview",
    "input": "Generate an image of a futuristic city.",
    "response_modalities": ["IMAGE"]
}'
```

#### Configure output

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-pro-image-preview",
    "input": "Generate an image of a futuristic city.",
    "generation_config": {
        "image_config": {
            "aspect_ratio": "9:16",
            "image_size": "2k"
        }
    }
}'
```

### Speech generation

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.5-flash-preview-tts",
    "input": "Say the following: WOOHOO This is so much fun!.",
    "response_modalities": ["AUDIO"],
    "generation_config": {
        "speech_config": {
            "language": "en-us",
            "voice": "kore"
        }
    }
}' | jq -r '.outputs[] | select(.type == "audio") | .data' | base64 -d > generated_audio.pcm
# You may need to install ffmpeg.
ffmpeg -f s16le -ar 24000 -ac 1 -i generated_audio.pcm generated_audio.wav

```


## Function calling

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "What is the weather in Paris?",
    "tools": [{
        "type": "function",
        "name": "get_weather",
        "description": "Gets the weather for a given location.",
        "parameters": {
            "type": "object",
            "properties": {
                "location": {"type": "string", "description": "The city and state, e.g. San Francisco, CA"}
            },
            "required": ["location"]
        }
    }]
}'
```

## Agents with the Interaction API

### Deep Research Agent

```
# 1. Start the Deep Research Agent
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "input": "Research the history of the Google TPUs with a focus on 2025 and 2026.",
    "agent": "deep-research-pro-preview-12-2025",
    "background": true
}'

# 2. Poll for results (Replace INTERACTION_ID with the ID from the previous interaction)
# curl -X GET "https://generativelanguage.googleapis.com/v1beta/interactions/INTERACTION_ID" \
# -H "x-goog-api-key: $GEMINI_API_KEY"
```

## Built in tools

### Google Search Grounding

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Who won the last Super Bowl?",
    "tools": [{"type": "google_search"}]
}'
```


### Code execution

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Calculate the 50th Fibonacci number.",
    "tools": [{"type": "code_execution"}]
}'
```

### URL Context

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Summarize the content of https://www.wikipedia.org/",
    "tools": [{"type": "url_context"}]
}'
```

### Computer Use

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.5-computer-use-preview-10-2025",
    "input": "Search for highly rated smart fridges with touchscreen, 2 doors, around 25 cu ft, priced below 4000 dollars on Google Shopping. Create a bulleted list of the 3 cheapest options in the format of name, description, price in an easy-to-read layout.",
    "tools": [{
        "type": "computer_use",
        "environment": "browser",
        "excludedPredefinedFunctions": ["drag_and_drop"]
    }]
}'
```

## Remote MCP

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.5-flash",
    "input": "What is the weather like in New York today?",
    "tools": [{
        "type": "mcp_server",
        "name": "weather_service",
        "url": "https://gemini-api-demos.uc.r.appspot.com/mcp"
    }],
    "system_instruction": "Today is '"$(date +"%du%Bt%Y")"' YYYY-MM-DD>."
}'
```

## Structured output

```curl
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Moderate the following content: 'Congratulations! You've won a free cruise. Click here to claim your prize: www.definitely-not-a-scam.com'",
    "response_format": {
        "type": "object",
        "properties": {
            "decision": {
                "type": "object",
                "properties": {
                    "reason": {"type": "string", "description": "The reason why the content is considered spam."},
                    "spam_type": {"type": "string", "description": "The type of spam."}
                },
                "required": ["reason", "spam_type"]
            }
        },
        "required": ["decision"]
    }
}'
```

## Advanced

### Combining

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Who won the last euro?",
    "tools": [{"type": "google_search"}],
    "response_format": {
        "type": "object",
        "properties": {
            "winning_team": {"type": "string"},
            "score": {"type": "string"}
        }
    }
}'
```

### Streaming

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions?alt=sse" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Explain quantum entanglement in simple terms.",
    "stream": true
}'
```

### Configuration

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Tell me a story about a brave knight.",
    "generation_config": {
        "temperature": 0.7,
        "max_output_tokens": 500,
        "thinking_level": "low"
    }
}'
```

### Thinking Summaries

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": "Solve this step by step: What is 15% of 240?",
    "generation_config": {
        "thinking_level": "high",
        "thinking_summaries": "auto"
    }
}'
```

## Files

```
curl -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
-H "Content-Type: application/json" \
-H "x-goog-api-key: $GEMINI_API_KEY" \
-d '{
    "model": "gemini-3-flash-preview",
    "input": [
        {
            "type": "image",
            "uri": "https://github.com/<github-path>/cats-and-dogs.jpg"
        },
        {"type": "text", "text": "Describe what you see."}
    ]
}'
```