package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/client"
	"gcli2apigo/internal/config"
)

// HandleGeminiListModels handles native Gemini models endpoint
func HandleGeminiListModels(w http.ResponseWriter, r *http.Request) {
	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","code":401}}`, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":{"message":"Method not allowed","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	log.Println("Gemini models list requested")

	// Build models list including fake streaming variants
	allModels := make([]config.Model, 0, len(config.SupportedModels)*2)

	// Add base models
	allModels = append(allModels, config.SupportedModels...)

	// Add fake streaming variants for supported models
	for _, model := range config.SupportedModels {
		modelID := strings.TrimPrefix(model.Name, "models/")
		if isFakeStreamingAllowed(modelID) {
			fakeModelName := config.GetFakeModelName(modelID)
			fakeModel := model
			fakeModel.Name = "models/" + fakeModelName
			fakeModel.DisplayName = fakeModel.DisplayName + " (Fake Streaming)"
			fakeModel.Description = fakeModel.Description + " - Fake streaming variant"
			allModels = append(allModels, fakeModel)
		}
	}

	modelsResponse := map[string]interface{}{
		"models": allModels,
	}

	log.Printf("Returning %d Gemini models (including fake streaming variants)", len(allModels))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(modelsResponse)
}

// HandleGeminiListModelsV1 handles alternative models endpoint for v1 API version
func HandleGeminiListModelsV1(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/models" {
		HandleGeminiListModels(w, r)
	} else {
		http.NotFound(w, r)
	}
}

// HandleGeminiProxy handles native Gemini API proxy endpoint
func HandleGeminiProxy(w http.ResponseWriter, r *http.Request) {
	// Skip if this is a known route
	if r.URL.Path == "/" || r.URL.Path == "/health" ||
		strings.HasPrefix(r.URL.Path, "/v1/chat/completions") ||
		(r.URL.Path == "/v1/models" && r.Method == http.MethodGet) ||
		(r.URL.Path == "/v1beta/models" && r.Method == http.MethodGet) {
		http.NotFound(w, r)
		return
	}

	// Only handle Gemini API paths
	if !strings.HasPrefix(r.URL.Path, "/v1beta/") && !strings.HasPrefix(r.URL.Path, "/v1/") {
		http.NotFound(w, r)
		return
	}

	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","code":401}}`, http.StatusUnauthorized)
		return
	}

	// Get the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":{"message":"Failed to read request body","code":400}}`, http.StatusBadRequest)
		return
	}

	// Determine if this is a streaming request
	isStreaming := strings.Contains(strings.ToLower(r.URL.Path), "stream")

	// Extract model name from the path
	modelName := extractModelFromPath(r.URL.Path)

	log.Printf("Gemini proxy request: path=%s, model=%s, stream=%v", r.URL.Path, modelName, isStreaming)

	if modelName == "" {
		log.Printf("Could not extract model name from path: %s", r.URL.Path)
		http.Error(w, fmt.Sprintf(`{"error":{"message":"Could not extract model name from path: %s","code":400}}`, r.URL.Path), http.StatusBadRequest)
		return
	}

	// Detect and handle fake stream mode based on language setting
	isFakeStream := false

	// Check for English format: modelID-fake
	if strings.HasSuffix(modelName, "-fake") {
		isFakeStream = true
		modelName = strings.TrimSuffix(modelName, "-fake")
	} else if strings.HasPrefix(modelName, "假流式/") {
		// Check for Chinese format: 假流式/modelID
		isFakeStream = true
		modelName = strings.TrimPrefix(modelName, "假流式/")
	}

	if isFakeStream {
		log.Printf("Detected fake stream mode in Gemini proxy, stripped model name: %s", modelName)

		// Validate that fake streaming is only allowed for specific models
		if !isFakeStreamingAllowed(modelName) {
			errorData := map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Fake streaming is not supported for model: %s. Only gemini-2.5-pro (and preview models) and gemini flash models (excluding gemini-flash-image) support fake streaming.", modelName),
					"code":    400,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorData)
			return
		}

		// Force streaming mode for fake stream
		isStreaming = true
	}

	// Parse the incoming request
	var incomingRequest map[string]interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &incomingRequest); err != nil {
			log.Printf("Invalid JSON in request body: %v", err)
			http.Error(w, `{"error":{"message":"Invalid JSON in request body","code":400}}`, http.StatusBadRequest)
			return
		}
	} else {
		incomingRequest = make(map[string]interface{})
	}

	// Build the payload for Google API
	geminiPayload := client.BuildGeminiPayloadFromNative(incomingRequest, modelName)

	// Send the request to Google API
	result, err := client.SendGeminiRequest(geminiPayload, isStreaming)
	if err != nil {
		log.Printf("Gemini proxy error: %v", err)
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Proxy error: %v", err),
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	if isStreaming {
		handleGeminiStreamingResponse(w, result)
	} else {
		handleGeminiNonStreamingResponse(w, result, modelName)
	}
}

func handleGeminiStreamingResponse(w http.ResponseWriter, result interface{}) {
	streamChan, ok := result.(chan string)
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Streaming request failed",
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("Vary", "Origin, X-Origin, Referer")
	w.Header().Set("X-XSS-Protection", "0")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Server", "ESF")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":{"message":"Streaming not supported","code":500}}`, http.StatusInternalServerError)
		return
	}

	// Smart buffering: accumulate chunks and flush on sentence boundaries or time
	var chunkAccumulator strings.Builder
	chunkAccumulator.Grow(8 * 1024) // Pre-allocate 8KB

	lastFlushTime := time.Now()
	flushInterval := 50 * time.Millisecond

	// Sentence boundary detection
	isSentenceBoundary := func(text string) bool {
		if len(text) == 0 {
			return false
		}
		// Check last rune for sentence boundaries (supports Unicode)
		runes := []rune(text)
		lastRune := runes[len(runes)-1]
		return lastRune == '.' || lastRune == '!' || lastRune == '?' ||
			lastRune == '。' || lastRune == '！' || lastRune == '？' ||
			lastRune == '\n'
	}

	sendAccumulatedChunk := func() {
		if chunkAccumulator.Len() == 0 {
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", chunkAccumulator.String())
		flusher.Flush()

		chunkAccumulator.Reset()
		lastFlushTime = time.Now()
	}

	for chunk := range streamChan {
		chunkAccumulator.WriteString(chunk)

		// Extract text to check for sentence boundaries
		var geminiChunk map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &geminiChunk); err == nil {
			candidates, _ := geminiChunk["candidates"].([]interface{})
			for _, candidate := range candidates {
				candMap, _ := candidate.(map[string]interface{})
				content, _ := candMap["content"].(map[string]interface{})
				parts, _ := content["parts"].([]interface{})

				var textContent string
				for _, part := range parts {
					partMap, _ := part.(map[string]interface{})
					if text, ok := partMap["text"].(string); ok {
						textContent += text
					}
				}

				// Flush conditions:
				// 1. Sentence boundary detected
				// 2. Time interval exceeded (50ms)
				// 3. Buffer size exceeded (8KB safety limit)
				timeSinceFlush := time.Since(lastFlushTime)

				if isSentenceBoundary(textContent) ||
					timeSinceFlush >= flushInterval ||
					chunkAccumulator.Len() >= 8*1024 {
					sendAccumulatedChunk()
					return
				}
			}
		}
	}

	sendAccumulatedChunk() // Final flush
}

func handleGeminiNonStreamingResponse(w http.ResponseWriter, result interface{}, modelName string) {
	geminiResponse, ok := result.(map[string]interface{})
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid response from API",
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	// Check for error in response
	if errObj, ok := geminiResponse["error"]; ok {
		w.Header().Set("Content-Type", "application/json")
		if errMap, ok := errObj.(map[string]interface{}); ok {
			if code, ok := errMap["code"].(float64); ok {
				w.WriteHeader(int(code))
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"error": errObj})
		return
	}

	log.Printf("Successfully processed Gemini request for model: %s", modelName)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(geminiResponse)
}

func extractModelFromPath(path string) string {
	parts := strings.Split(path, "/")

	// Look for the pattern: .../models/{model_name}/...
	for i, part := range parts {
		if part == "models" && i+1 < len(parts) {
			modelName := parts[i+1]
			// Remove any action suffix like ":streamGenerateContent" or ":generateContent"
			if idx := strings.Index(modelName, ":"); idx != -1 {
				modelName = modelName[:idx]
			}
			return modelName
		}
	}

	return ""
}
