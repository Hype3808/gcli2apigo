package transformers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gcli2apigo/internal/config"
	"gcli2apigo/internal/models"

	"github.com/google/uuid"
)

// OpenAIRequestToGemini transforms an OpenAI chat completion request to Gemini format
func OpenAIRequestToGemini(req *models.OpenAIChatCompletionRequest) map[string]interface{} {
	contents := make([]map[string]interface{}, 0)

	// Process each message in the conversation
	for _, message := range req.Messages {
		role := message.Role

		// Map OpenAI roles to Gemini roles
		if role == "assistant" {
			role = "model"
		} else if role == "system" {
			role = "user"
		}

		parts := make([]map[string]interface{}, 0)

		// Handle different content types
		switch content := message.Content.(type) {
		case string:
			// Simple text content; extract Markdown images
			parts = extractMarkdownImages(content)

		case []interface{}:
			// List of content parts
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partType, _ := partMap["type"].(string); partType == "text" {
						if text, ok := partMap["text"].(string); ok {
							parts = append(parts, extractMarkdownImages(text)...)
						}
					} else if partType == "image_url" {
						if imageURL, ok := partMap["image_url"].(map[string]interface{}); ok {
							if url, ok := imageURL["url"].(string); ok {
								imagePart := parseDataURI(url)
								if imagePart != nil {
									parts = append(parts, imagePart)
								}
							}
						}
					}
				}
			}
		}

		if len(parts) == 0 {
			parts = append(parts, map[string]interface{}{"text": ""})
		}

		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": parts,
		})
	}

	// Map OpenAI generation parameters to Gemini format
	generationConfig := make(map[string]interface{})
	if req.Temperature != nil {
		generationConfig["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		generationConfig["topP"] = *req.TopP
	}
	if req.MaxTokens != nil {
		generationConfig["maxOutputTokens"] = *req.MaxTokens
	}
	if req.Stop != nil {
		switch stop := req.Stop.(type) {
		case string:
			generationConfig["stopSequences"] = []string{stop}
		case []interface{}:
			stopSeqs := make([]string, 0)
			for _, s := range stop {
				if str, ok := s.(string); ok {
					stopSeqs = append(stopSeqs, str)
				}
			}
			generationConfig["stopSequences"] = stopSeqs
		}
	}
	if req.FrequencyPenalty != nil {
		generationConfig["frequencyPenalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		generationConfig["presencePenalty"] = *req.PresencePenalty
	}
	if req.N != nil {
		generationConfig["candidateCount"] = *req.N
	}
	if req.Seed != nil {
		generationConfig["seed"] = *req.Seed
	}
	if req.ResponseFormat != nil {
		if respType, ok := req.ResponseFormat["type"].(string); ok && respType == "json_object" {
			generationConfig["responseMimeType"] = "application/json"
		}
	}

	// Build the request payload
	requestPayload := map[string]interface{}{
		"contents":         contents,
		"generationConfig": generationConfig,
		"safetySettings":   config.DefaultSafetySettings,
		"model":            config.GetBaseModelName(req.Model),
	}

	return requestPayload
}

// GeminiResponseToOpenAI transforms a Gemini API response to OpenAI chat completion format
func GeminiResponseToOpenAI(geminiResp map[string]interface{}, model string) map[string]interface{} {
	choices := make([]map[string]interface{}, 0)

	candidates, _ := geminiResp["candidates"].([]interface{})
	for _, candidate := range candidates {
		candMap, _ := candidate.(map[string]interface{})
		content, _ := candMap["content"].(map[string]interface{})
		role, _ := content["role"].(string)

		// Map Gemini roles back to OpenAI roles
		if role == "model" {
			role = "assistant"
		}

		// Extract and separate thinking tokens from regular content
		parts, _ := content["parts"].([]interface{})
		contentParts := make([]string, 0)
		reasoningContent := ""

		for _, part := range parts {
			partMap, _ := part.(map[string]interface{})

			// Text parts (may include thinking tokens)
			if text, ok := partMap["text"].(string); ok {
				if thought, _ := partMap["thought"].(bool); thought {
					reasoningContent += text
				} else {
					contentParts = append(contentParts, text)
				}
				continue
			}

			// Inline image data -> embed as Markdown data URI
			if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
				if data, ok := inlineData["data"].(string); ok {
					mimeType, _ := inlineData["mimeType"].(string)
					if mimeType == "" {
						mimeType = "image/png"
					}
					if strings.HasPrefix(mimeType, "image/") {
						contentParts = append(contentParts, fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data))
					}
				}
			}
		}

		contentStr := strings.Join(contentParts, "\n\n")

		// Build message object
		message := map[string]interface{}{
			"role":    role,
			"content": contentStr,
		}

		// Add reasoning_content if there are thinking tokens
		if reasoningContent != "" {
			message["reasoning_content"] = reasoningContent
		}

		index, _ := candMap["index"].(float64)
		finishReason, _ := candMap["finishReason"].(string)

		choices = append(choices, map[string]interface{}{
			"index":         int(index),
			"message":       message,
			"finish_reason": mapFinishReason(finishReason),
		})
	}

	return map[string]interface{}{
		"id":      uuid.New().String(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": choices,
	}
}

// GeminiStreamChunkToOpenAI transforms a Gemini streaming response chunk to OpenAI streaming format
func GeminiStreamChunkToOpenAI(geminiChunk map[string]interface{}, model string, responseID string) map[string]interface{} {
	choices := make([]map[string]interface{}, 0)

	candidates, _ := geminiChunk["candidates"].([]interface{})
	for _, candidate := range candidates {
		candMap, _ := candidate.(map[string]interface{})
		content, _ := candMap["content"].(map[string]interface{})
		role, _ := content["role"].(string)

		// Map Gemini roles back to OpenAI roles
		if role == "model" {
			role = "assistant"
		}

		// Extract and separate thinking tokens from regular content
		parts, _ := content["parts"].([]interface{})
		contentParts := make([]string, 0)
		reasoningContent := ""

		for _, part := range parts {
			partMap, _ := part.(map[string]interface{})

			// Text parts (may include thinking tokens)
			if text, ok := partMap["text"].(string); ok {
				if thought, _ := partMap["thought"].(bool); thought {
					reasoningContent += text
				} else {
					contentParts = append(contentParts, text)
				}
				continue
			}

			// Inline image data -> embed as Markdown data URI
			if inlineData, ok := partMap["inlineData"].(map[string]interface{}); ok {
				if data, ok := inlineData["data"].(string); ok {
					mimeType, _ := inlineData["mimeType"].(string)
					if mimeType == "" {
						mimeType = "image/png"
					}
					if strings.HasPrefix(mimeType, "image/") {
						contentParts = append(contentParts, fmt.Sprintf("![image](data:%s;base64,%s)", mimeType, data))
					}
				}
			}
		}

		contentStr := strings.Join(contentParts, "\n\n")

		// Build delta object
		delta := make(map[string]interface{})
		if contentStr != "" {
			delta["content"] = contentStr
		}
		if reasoningContent != "" {
			delta["reasoning_content"] = reasoningContent
		}

		index, _ := candMap["index"].(float64)
		finishReason, _ := candMap["finishReason"].(string)

		choices = append(choices, map[string]interface{}{
			"index":         int(index),
			"delta":         delta,
			"finish_reason": mapFinishReason(finishReason),
		})
	}

	return map[string]interface{}{
		"id":      responseID,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": choices,
	}
}

func mapFinishReason(geminiReason string) interface{} {
	switch geminiReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	default:
		return nil
	}
}

func extractMarkdownImages(text string) []map[string]interface{} {
	parts := make([]map[string]interface{}, 0)
	pattern := regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatchIndex(text, -1)

	if len(matches) == 0 {
		parts = append(parts, map[string]interface{}{"text": text})
		return parts
	}

	lastIdx := 0
	for _, match := range matches {
		// match[0] = start of full match, match[1] = end of full match
		// match[2] = start of URL group, match[3] = end of URL group
		start, end := match[0], match[1]
		urlStart, urlEnd := match[2], match[3]

		// Emit text before the image
		if start > lastIdx {
			before := text[lastIdx:start]
			if before != "" {
				parts = append(parts, map[string]interface{}{"text": before})
			}
		}

		// Handle data URI images
		url := strings.TrimSpace(text[urlStart:urlEnd])
		url = strings.Trim(url, `"'`)

		if strings.HasPrefix(url, "data:") {
			imagePart := parseDataURI(url)
			if imagePart != nil {
				parts = append(parts, imagePart)
			} else {
				// Fallback: keep original markdown as text
				parts = append(parts, map[string]interface{}{"text": text[start:end]})
			}
		} else {
			// Non-data URIs: keep markdown as text
			parts = append(parts, map[string]interface{}{"text": text[start:end]})
		}

		lastIdx = end
	}

	// Tail text after last image
	if lastIdx < len(text) {
		tail := text[lastIdx:]
		if tail != "" {
			parts = append(parts, map[string]interface{}{"text": tail})
		}
	}

	return parts
}

func parseDataURI(url string) map[string]interface{} {
	if !strings.HasPrefix(url, "data:") {
		return nil
	}

	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return nil
	}

	header := parts[0]
	base64Data := parts[1]

	// Extract MIME type from header (e.g., "data:image/png;base64")
	mimeType := "image/png"
	if strings.Contains(header, ":") {
		headerParts := strings.SplitN(header, ":", 2)
		if len(headerParts) == 2 {
			mimeTypePart := strings.Split(headerParts[1], ";")[0]
			if mimeTypePart != "" {
				mimeType = mimeTypePart
			}
		}
	}

	return map[string]interface{}{
		"inlineData": map[string]interface{}{
			"mimeType": mimeType,
			"data":     base64Data,
		},
	}
}
