package main

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/sunquakes/jsonrpc4go/v2/server"
)

// Fixture content used by the MCP conformance suite. The suite calls these
// exact tool names and checks the returned CallToolResult shapes.

const (
	fixturePNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	fixtureWAV = "UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA="
)

func textResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": false,
	}
}

func imageBlock() map[string]any {
	return map[string]any{"type": "image", "data": fixturePNG, "mimeType": "image/png"}
}

func audioBlock() map[string]any {
	return map[string]any{"type": "audio", "data": fixtureWAV, "mimeType": "audio/wav"}
}

func resourceBlock() map[string]any {
	return map[string]any{
		"type": "resource",
		"resource": map[string]any{
			"uri":      "test://embedded/text",
			"mimeType": "text/plain",
			"text":     "This is embedded resource content",
		},
	}
}

func registerFixtures(s *server.StreamableHttpServer) {
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_simple_text",
		Description: "Returns a simple text response",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return textResult("Hello, world!"), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_image_content",
		Description: "Returns an image content block",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return map[string]any{"content": []map[string]any{imageBlock()}, "isError": false}, nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_audio_content",
		Description: "Returns an audio content block",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return map[string]any{"content": []map[string]any{audioBlock()}, "isError": false}, nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_embedded_resource",
		Description: "Returns an embedded resource content block",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return map[string]any{"content": []map[string]any{resourceBlock()}, "isError": false}, nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_multiple_content_types",
		Description: "Returns text, image and embedded resource blocks",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return map[string]any{"content": []map[string]any{
				{"type": "text", "text": "This response contains multiple content types"},
				imageBlock(),
				resourceBlock(),
			}, "isError": false}, nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_tool_with_logging",
		Description: "Emits log notifications while running",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			_ = ctx.Log("debug", "test-logger", map[string]any{"message": "Debug: starting test_tool_with_logging", "timestamp": time.Now().Format(time.RFC3339)})
			_ = ctx.Log("info", "test-logger", map[string]any{"message": "Info: processing test_tool_with_logging request", "step": 1})
			_ = ctx.Log("warning", "test-logger", map[string]any{"message": "Warning: test_tool_with_logging completed with warnings"})
			return textResult("Tool executed with log notifications"), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_error_handling",
		Description: "Returns a tool-level error result",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": "Error: test tool failed as expected"}},
				"isError": true,
			}, nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_tool_with_progress",
		Description: "Emits progress notifications while running",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			for i := 1; i <= 5; i++ {
				_ = ctx.Progress(float64(i), 5, fmt.Sprintf("Step %d of 5", i))
				time.Sleep(20 * time.Millisecond)
			}
			return textResult("Tool with progress completed"), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_sampling",
		Description: "Requests sampling from the client",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			result, err := ctx.Request("sampling/createMessage", map[string]any{
				"messages": []map[string]any{
					{"role": "user", "content": map[string]any{"type": "text", "text": "Hello from server"}},
				},
				"maxTokens": 100,
			}, 10*time.Second)
			if err != nil {
				return map[string]any{
					"content": []map[string]any{{"type": "text", "text": "Sampling failed: " + err.Error()}},
					"isError": true,
				}, nil
			}
			return textResult(fmt.Sprintf("Sampling completed: %v", result)), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_elicitation",
		Description: "Requests user input from the client",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			result, err := ctx.Request("elicitation/create", map[string]any{
				"message": "Please provide your name",
				"requestedSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
			}, 10*time.Second)
			if err != nil {
				return map[string]any{
					"content": []map[string]any{{"type": "text", "text": "Elicitation failed: " + err.Error()}},
					"isError": true,
				}, nil
			}
			return textResult(fmt.Sprintf("Elicitation completed: %v", result)), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_elicitation_sep1034_defaults",
		Description: "Elicitation with schema default values",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			result, err := ctx.Request("elicitation/create", map[string]any{
				"message": "Please provide your information",
				"requestedSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":     map[string]any{"type": "string", "default": "John Doe"},
						"age":      map[string]any{"type": "integer", "default": 30},
						"score":    map[string]any{"type": "number", "default": 95.5},
						"status":   map[string]any{"type": "string", "enum": []string{"active", "inactive", "pending"}, "default": "active"},
						"verified": map[string]any{"type": "boolean", "default": true},
					},
				},
			}, 10*time.Second)
			if err != nil {
				return map[string]any{
					"content": []map[string]any{{"type": "text", "text": "Elicitation failed: " + err.Error()}},
					"isError": true,
				}, nil
			}
			return textResult(fmt.Sprintf("Defaults elicitation completed: %v", result)), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_elicitation_sep1330_enums",
		Description: "Elicitation with enum schemas",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			result, err := ctx.Request("elicitation/create", map[string]any{
				"message": "Please select options",
				"requestedSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"untitledSingle": map[string]any{
							"type": "string",
							"enum": []string{"option1", "option2", "option3"},
						},
						"titledSingle": map[string]any{
							"type": "string",
							"oneOf": []map[string]any{
								{"const": "value1", "title": "First Option"},
								{"const": "value2", "title": "Second Option"},
								{"const": "value3", "title": "Third Option"},
							},
						},
						"legacyEnum": map[string]any{
							"type":      "string",
							"enum":      []string{"opt1", "opt2", "opt3"},
							"enumNames": []string{"Option One", "Option Two", "Option Three"},
						},
						"untitledMulti": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "enum": []string{"option1", "option2", "option3"}},
						},
						"titledMulti": map[string]any{
							"type": "array",
							"items": map[string]any{
								"anyOf": []map[string]any{
									{"const": "value1", "title": "First Choice"},
									{"const": "value2", "title": "Second Choice"},
									{"const": "value3", "title": "Third Choice"},
								},
							},
						},
					},
				},
			}, 10*time.Second)
			if err != nil {
				return map[string]any{
					"content": []map[string]any{{"type": "text", "text": "Elicitation failed: " + err.Error()}},
					"isError": true,
				}, nil
			}
			return textResult(fmt.Sprintf("Enums elicitation completed: %v", result)), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "json_schema_2020_12_tool",
		Description: "Tool whose inputSchema uses the JSON Schema 2020-12 dialect",
		InputSchema: map[string]any{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type":    "object",
			"$defs": map[string]any{
				"address": map[string]any{
					"$anchor": "addressDef",
					"type":    "object",
					"properties": map[string]any{
						"street": map[string]any{"type": "string"},
						"city":   map[string]any{"type": "string"},
					},
				},
			},
			"properties": map[string]any{
				"name":          map[string]any{"type": "string"},
				"address":       map[string]any{"$ref": "#/$defs/address"},
				"contactMethod": map[string]any{"type": "string", "enum": []string{"phone", "email"}},
				"phone":         map[string]any{"type": "string"},
				"email":         map[string]any{"type": "string"},
			},
			"allOf":                []any{map[string]any{"anyOf": []any{map[string]any{"required": []string{"phone"}}, map[string]any{"required": []string{"email"}}}}},
			"if":                   map[string]any{"properties": map[string]any{"contactMethod": map[string]any{"const": "phone"}}, "required": []string{"contactMethod"}},
			"then":                 map[string]any{"required": []string{"phone"}},
			"else":                 map[string]any{"required": []string{"email"}},
			"additionalProperties": false,
		},
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			return textResult(fmt.Sprintf("Received: %v", args)), nil
		},
	})
	s.RegisterMcpTool(server.McpTool{
		Name:        "test_reconnection",
		Description: "Ends the stream mid-call and delivers the response via Last-Event-ID replay",
		Handler: func(ctx *server.McpContext, args map[string]any) (any, error) {
			ctx.EndStream()
			go func() {
				time.Sleep(200 * time.Millisecond)
				ctx.SendResult(textResult("Response after reconnection"))
			}()
			return nil, nil
		},
	})

	// Resources.
	s.RegisterMcpResource(server.McpResource{
		URI:         "test://static-text",
		Name:        "Static Text Resource",
		Description: "A static text resource",
		MimeType:    "text/plain",
		Text:        "This is static text content",
	})
	s.RegisterMcpResource(server.McpResource{
		URI:         "test://static-binary",
		Name:        "Static Binary Resource",
		Description: "A static binary resource",
		MimeType:    "application/octet-stream",
		Blob:        []byte{0x00, 0x01, 0x02, 0x03},
	})
	s.RegisterMcpResource(server.McpResource{
		URI:         "test://watched-resource",
		Name:        "Watched Resource",
		Description: "Resource used for subscribe/unsubscribe tests",
		MimeType:    "text/plain",
		Text:        "watched",
	})
	s.RegisterMcpResource(server.McpResource{
		URI:         "test://example-resource",
		Name:        "Example Resource",
		Description: "Resource referenced by the embedded-resource prompt",
		MimeType:    "text/plain",
		Text:        "This is example resource content",
	})
	s.RegisterMcpResourceTemplate(server.McpResourceTemplate{
		URITemplate: "test://template/{id}/data",
		Name:        "Template Resource",
		Description: "Resource with parameters",
		MimeType:    "text/plain",
		Read: func(uri string) (string, []byte, bool) {
			return "Template content for " + uri, nil, true
		},
	})

	// Prompts.
	s.RegisterMcpPrompt(server.McpPrompt{
		Name:        "test_simple_prompt",
		Description: "A simple prompt",
		Render: func(args map[string]any) (map[string]any, error) {
			return map[string]any{
				"description": "Simple prompt",
				"messages": []map[string]any{
					{"role": "user", "content": map[string]any{"type": "text", "text": "This is a simple prompt"}},
				},
			}, nil
		},
	})
	s.RegisterMcpPrompt(server.McpPrompt{
		Name:        "test_prompt_with_arguments",
		Description: "Prompt with arguments",
		Arguments: []map[string]any{
			{"name": "arg1", "description": "First argument", "required": true},
			{"name": "arg2", "description": "Second argument", "required": true},
		},
		Render: func(args map[string]any) (map[string]any, error) {
			return map[string]any{
				"description": "Prompt with arguments",
				"messages": []map[string]any{
					{"role": "user", "content": map[string]any{"type": "text", "text": fmt.Sprintf("Prompt with arguments: arg1='%v', arg2='%v'", args["arg1"], args["arg2"])}},
				},
			}, nil
		},
	})
	s.RegisterMcpPrompt(server.McpPrompt{
		Name:        "test_prompt_with_embedded_resource",
		Description: "Prompt with embedded resource",
		Arguments: []map[string]any{
			{"name": "resourceUri", "description": "URI of the resource to embed", "required": true},
		},
		Render: func(args map[string]any) (map[string]any, error) {
			uri, _ := args["resourceUri"].(string)
			return map[string]any{
				"description": "Prompt with embedded resource",
				"messages": []map[string]any{
					{"role": "user", "content": map[string]any{
						"type": "resource",
						"resource": map[string]any{
							"uri":      uri,
							"mimeType": "text/plain",
							"text":     "This is embedded resource content",
						},
					}},
				},
			}, nil
		},
	})
	s.RegisterMcpPrompt(server.McpPrompt{
		Name:        "test_prompt_with_image",
		Description: "Prompt with image content",
		Render: func(args map[string]any) (map[string]any, error) {
			png, _ := base64.StdEncoding.DecodeString(fixturePNG)
			return map[string]any{
				"description": "Prompt with image",
				"messages": []map[string]any{
					{"role": "user", "content": map[string]any{"type": "text", "text": "Prompt with image content"}},
					{"role": "user", "content": map[string]any{"type": "image", "data": fixturePNG, "mimeType": "image/png", "annotations": map[string]any{"size": len(png)}}},
				},
			}, nil
		},
	})
}
