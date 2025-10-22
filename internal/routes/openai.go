package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/client"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/models"
	"gcli2apigo/internal/transformers"

	"github.com/google/uuid"
)

// ChunkAccumulator accumulates streaming chunks with size checking
type ChunkAccumulator struct {
	chunks      []map[string]interface{}
	mu          sync.Mutex
	maxSize     int64
	currentSize int64
}

// NewChunkAccumulator creates a new ChunkAccumulator with the specified max size
func NewChunkAccumulator(maxSize int64) *ChunkAccumulator {
	return &ChunkAccumulator{
		chunks:      make([]map[string]interface{}, 0),
		maxSize:     maxSize,
		currentSize: 0,
	}
}

// Add adds a chunk to the accumulator with size checking
func (ca *ChunkAccumulator) Add(chunk map[string]interface{}) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	// Estimate chunk size by marshaling to JSON
	chunkBytes, err := json.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("failed to marshal chunk: %v", err)
	}

	chunkSize := int64(len(chunkBytes))

	// Check if adding this chunk would exceed the size limit
	if ca.currentSize+chunkSize > ca.maxSize {
		return fmt.Errorf("accumulated size would exceed limit: current=%d bytes, chunk=%d bytes, limit=%d bytes",
			ca.currentSize, chunkSize, ca.maxSize)
	}

	ca.chunks = append(ca.chunks, chunk)
	ca.currentSize += chunkSize

	return nil
}

// GetComplete merges all accumulated chunks into a complete response
func (ca *ChunkAccumulator) GetComplete() map[string]interface{} {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if len(ca.chunks) == 0 {
		return nil
	}

	// Merge all chunks - implementation in subtask 2.2
	return ca.mergeChunks()
}

// Size returns the current accumulated size in bytes
func (ca *ChunkAccumulator) Size() int64 {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	return ca.currentSize
}

// mergeChunks merges all accumulated chunks into a single response
func (ca *ChunkAccumulator) mergeChunks() map[string]interface{} {
	if len(ca.chunks) == 0 {
		return nil
	}

	// Start with the first chunk as the base
	merged := make(map[string]interface{})

	// Copy top-level fields from the first chunk
	firstChunk := ca.chunks[0]
	for key, value := range firstChunk {
		merged[key] = value
	}

	// Merge all candidates from all chunks
	allCandidates := make([]map[string]interface{}, 0)

	for _, chunk := range ca.chunks {
		if candidates, ok := chunk["candidates"].([]interface{}); ok {
			for _, candidate := range candidates {
				if candMap, ok := candidate.(map[string]interface{}); ok {
					allCandidates = append(allCandidates, candMap)
				}
			}
		}
	}

	// Group candidates by index and merge their content
	candidatesByIndex := make(map[int][]map[string]interface{})

	for _, candidate := range allCandidates {
		index := 0
		if idx, ok := candidate["index"].(float64); ok {
			index = int(idx)
		}
		candidatesByIndex[index] = append(candidatesByIndex[index], candidate)
	}

	// Merge each candidate group
	mergedCandidates := make([]interface{}, 0)

	for index := 0; index < len(candidatesByIndex); index++ {
		candidates := candidatesByIndex[index]
		if len(candidates) == 0 {
			continue
		}

		// Merge content parts from all chunks for this candidate
		var contentParts []interface{}
		var reasoningParts []string
		var finalFinishReason string

		for _, candidate := range candidates {
			// Extract content parts
			if content, ok := candidate["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok {
					for _, part := range parts {
						if partMap, ok := part.(map[string]interface{}); ok {
							// Check if this is a thinking token
							if thought, ok := partMap["thought"].(bool); ok && thought {
								if text, ok := partMap["text"].(string); ok {
									reasoningParts = append(reasoningParts, text)
								}
							} else {
								// Regular content part
								contentParts = append(contentParts, part)
							}
						}
					}
				}
			}

			// Use the last non-empty finish reason
			if finishReason, ok := candidate["finishReason"].(string); ok && finishReason != "" {
				finalFinishReason = finishReason
			}
		}

		// Build merged candidate
		mergedCandidate := map[string]interface{}{
			"index": index,
			"content": map[string]interface{}{
				"role":  "model",
				"parts": contentParts,
			},
		}

		// Add reasoning parts if present
		if len(reasoningParts) > 0 {
			// Add reasoning as a separate part with thought flag
			reasoningText := strings.Join(reasoningParts, "")
			if reasoningText != "" {
				// Add to content parts with thought flag
				existingParts := mergedCandidate["content"].(map[string]interface{})["parts"].([]interface{})
				reasoningPart := map[string]interface{}{
					"text":    reasoningText,
					"thought": true,
				}
				existingParts = append(existingParts, reasoningPart)
				mergedCandidate["content"].(map[string]interface{})["parts"] = existingParts
			}
		}

		// Add finish reason if present
		if finalFinishReason != "" {
			mergedCandidate["finishReason"] = finalFinishReason
		}

		mergedCandidates = append(mergedCandidates, mergedCandidate)
	}

	// Set the merged candidates
	merged["candidates"] = mergedCandidates

	return merged
}

// KeepAliveManager manages periodic keep-alive signals during fake stream collection
type KeepAliveManager struct {
	interval time.Duration
	stopChan chan struct{}
	writer   http.ResponseWriter
	flusher  http.Flusher
	mu       sync.Mutex
	stopOnce sync.Once
}

// NewKeepAliveManager creates a new KeepAliveManager
func NewKeepAliveManager(interval time.Duration, writer http.ResponseWriter, flusher http.Flusher) *KeepAliveManager {
	return &KeepAliveManager{
		interval: interval,
		stopChan: make(chan struct{}),
		writer:   writer,
		flusher:  flusher,
	}
}

// Start begins sending keep-alive signals at the configured interval
func (kam *KeepAliveManager) Start() {
	go func() {
		ticker := time.NewTicker(kam.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := kam.sendKeepAlive(); err != nil {
					log.Printf("Failed to send keep-alive signal: %v", err)
					return
				}
			case <-kam.stopChan:
				return
			}
		}
	}()
}

// Stop stops the keep-alive goroutine (safe to call multiple times)
func (kam *KeepAliveManager) Stop() {
	kam.stopOnce.Do(func() {
		close(kam.stopChan)
	})
}

// sendKeepAlive writes an SSE comment line as a keep-alive signal
func (kam *KeepAliveManager) sendKeepAlive() error {
	kam.mu.Lock()
	defer kam.mu.Unlock()

	// Write SSE comment line
	if _, err := fmt.Fprintf(kam.writer, ": keep-alive\n\n"); err != nil {
		return fmt.Errorf("failed to write keep-alive signal: %v", err)
	}

	// Flush to ensure the signal is sent immediately
	kam.flusher.Flush()

	return nil
}

// HandleChatCompletions handles OpenAI-compatible chat completions endpoint
func HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","type":"invalid_request_error","code":401}}`, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":{"message":"Method not allowed","type":"invalid_request_error","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":{"message":"Failed to read request body","type":"invalid_request_error","code":400}}`, http.StatusBadRequest)
		return
	}

	var request models.OpenAIChatCompletionRequest
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, `{"error":{"message":"Invalid JSON in request body","type":"invalid_request_error","code":400}}`, http.StatusBadRequest)
		return
	}

	log.Printf("OpenAI chat completion request: model=%s, stream=%v", request.Model, request.Stream)

	// Detect and handle -fake suffix for fake stream mode
	modelName := request.Model
	isFakeStream := strings.HasSuffix(modelName, "-fake")
	if isFakeStream {
		// Strip -fake suffix before forwarding to API
		modelName = strings.TrimSuffix(modelName, "-fake")
		request.Model = modelName
		log.Printf("Detected fake stream mode, stripped model name: %s", modelName)
	}

	// Transform OpenAI request to Gemini format
	geminiRequestData := transformers.OpenAIRequestToGemini(&request)

	// Build the payload for Google API
	geminiPayload := client.BuildGeminiPayloadFromOpenAI(geminiRequestData)

	// Route to appropriate handler
	if isFakeStream {
		// Force fake stream handler regardless of stream parameter
		handleFakeStreamChatCompletion(w, r, &request, geminiPayload)
	} else if request.Stream {
		handleStreamingChatCompletion(w, r, &request, geminiPayload)
	} else {
		handleNonStreamingChatCompletion(w, r, &request, geminiPayload)
	}
}

func handleFakeStreamChatCompletion(w http.ResponseWriter, r *http.Request, request *models.OpenAIChatCompletionRequest, geminiPayload map[string]interface{}) {
	// Set SSE headers since we'll return the response as streaming chunks
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":{"message":"Streaming not supported","type":"api_error","code":500}}`, http.StatusInternalServerError)
		return
	}

	// Create context with timeout for chunk collection (5 minutes)
	const collectionTimeout = 5 * time.Minute
	ctx, cancel := context.WithTimeout(r.Context(), collectionTimeout)
	defer cancel()

	// Force streaming mode for internal API request
	result, err := client.SendGeminiRequest(geminiPayload, true)
	if err != nil {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Request failed: %v", err),
				"type":    "api_error",
				"code":    500,
			},
		}
		jsonData, _ := json.Marshal(errorData)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	// Receive streaming channel from client layer
	streamChan, ok := result.(chan string)
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Streaming request failed",
				"type":    "api_error",
				"code":    500,
			},
		}
		jsonData, _ := json.Marshal(errorData)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	// Create chunk accumulator with 10 MB limit
	const maxSize = 10 * 1024 * 1024 // 10 MB
	accumulator := NewChunkAccumulator(maxSize)

	// Start heartbeat sender to keep connection alive during collection
	const heartbeatInterval = 3 * time.Second
	responseID := "chatcmpl-" + uuid.New().String()
	heartbeatDone := make(chan struct{})

	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send heartbeat chunk with empty content
				heartbeat := map[string]interface{}{
					"id":      responseID,
					"object":  "chat.completion.chunk",
					"created": time.Now().Unix(),
					"model":   request.Model,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"delta": map[string]interface{}{
								"role":    "assistant",
								"content": "",
							},
							"finish_reason": nil,
						},
					},
				}
				jsonData, _ := json.Marshal(heartbeat)
				fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
				flusher.Flush()
			case <-heartbeatDone:
				return
			}
		}
	}()
	defer close(heartbeatDone)

	log.Printf("Starting fake stream collection for model: %s", request.Model)

	// Loop through streaming channel and collect chunks
	for chunk := range streamChan {
		// Check for client disconnect or timeout
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				log.Printf("Timeout during fake stream collection after %v, cleaned up resources", collectionTimeout)
				errorData := map[string]interface{}{
					"error": map[string]interface{}{
						"message": fmt.Sprintf("Request timeout: chunk collection exceeded %v", collectionTimeout),
						"type":    "timeout_error",
						"code":    504,
					},
				}
				jsonData, _ := json.Marshal(errorData)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write(jsonData)
			} else {
				log.Printf("Client disconnected during fake stream collection, cleaned up resources")
			}
			return
		default:
		}

		var geminiChunk map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &geminiChunk); err != nil {
			log.Printf("Failed to unmarshal chunk: %v", err)
			continue
		}

		// Check for error chunks and abort if found
		if errObj, ok := geminiChunk["error"]; ok {
			errorData := map[string]interface{}{
				"error": errObj,
			}
			jsonData, _ := json.Marshal(errorData)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(jsonData)
			return
		}

		// Add chunk to accumulator
		if err := accumulator.Add(geminiChunk); err != nil {
			log.Printf("Failed to add chunk to accumulator: %v", err)
			errorData := map[string]interface{}{
				"error": map[string]interface{}{
					"message": fmt.Sprintf("Response too large: %v", err),
					"type":    "api_error",
					"code":    413,
				},
			}
			jsonData, _ := json.Marshal(errorData)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write(jsonData)
			return
		}
	}

	log.Printf("Completed fake stream collection, accumulated size: %d bytes", accumulator.Size())

	// Get complete response from accumulator
	completeResponse := accumulator.GetComplete()
	if completeResponse == nil {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "No response data collected",
				"type":    "api_error",
				"code":    500,
			},
		}
		jsonData, _ := json.Marshal(errorData)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(jsonData)
		return
	}

	// Transform to OpenAI non-streaming format first
	openaiResponse := transformers.GeminiResponseToOpenAI(completeResponse, request.Model)

	log.Printf("Successfully processed fake stream response for model: %s", request.Model)

	// Convert the complete response to a single streaming chunk and send via SSE
	// Use the same responseID that was used for heartbeats

	// Extract choices from the complete response
	// Note: choices might be []map[string]interface{} or []interface{}, handle both
	var choices []map[string]interface{}
	if choicesRaw, ok := openaiResponse["choices"].([]map[string]interface{}); ok {
		choices = choicesRaw
	} else if choicesInterface, ok := openaiResponse["choices"].([]interface{}); ok {
		// Convert []interface{} to []map[string]interface{}
		for _, c := range choicesInterface {
			if cMap, ok := c.(map[string]interface{}); ok {
				choices = append(choices, cMap)
			}
		}
	}

	// Build streaming choices with all content in deltas
	streamingChoices := make([]map[string]interface{}, 0)
	for _, choiceMap := range choices {
		message, _ := choiceMap["message"].(map[string]interface{})
		index, _ := choiceMap["index"].(int)
		finishReason := choiceMap["finish_reason"]

		// Build delta from message - this contains ALL the content
		delta := make(map[string]interface{})
		if content, ok := message["content"].(string); ok {
			delta["content"] = content
		}
		if reasoningContent, ok := message["reasoning_content"].(string); ok {
			delta["reasoning_content"] = reasoningContent
		}

		streamingChoices = append(streamingChoices, map[string]interface{}{
			"index":         index,
			"delta":         delta,
			"finish_reason": finishReason,
		})
	}

	// Create a single streaming chunk with all content
	streamChunk := map[string]interface{}{
		"id":      responseID,
		"object":  "chat.completion.chunk",
		"created": openaiResponse["created"],
		"model":   request.Model,
		"choices": streamingChoices,
	}

	// Send as single SSE event
	jsonData, _ := json.Marshal(streamChunk)
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	flusher.Flush()

	// Send the final [DONE] marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleStreamingChatCompletion(w http.ResponseWriter, r *http.Request, request *models.OpenAIChatCompletionRequest, geminiPayload map[string]interface{}) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":{"message":"Streaming not supported","type":"api_error","code":500}}`, http.StatusInternalServerError)
		return
	}

	// Send request to Gemini API
	result, err := client.SendGeminiRequest(geminiPayload, true)
	if err != nil {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Request failed: %v", err),
				"type":    "api_error",
				"code":    500,
			},
		}
		jsonData, _ := json.Marshal(errorData)
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	streamChan, ok := result.(chan string)
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Streaming request failed",
				"type":    "api_error",
				"code":    500,
			},
		}
		jsonData, _ := json.Marshal(errorData)
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	responseID := "chatcmpl-" + uuid.New().String()
	log.Printf("Starting streaming response: %s", responseID)

	for chunk := range streamChan {
		var geminiChunk map[string]interface{}
		if err := json.Unmarshal([]byte(chunk), &geminiChunk); err != nil {
			continue
		}

		// Check if this is an error chunk
		if errObj, ok := geminiChunk["error"]; ok {
			errorData := map[string]interface{}{
				"error": errObj,
			}
			jsonData, _ := json.Marshal(errorData)
			fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
			flusher.Flush()
			break
		}

		// Transform to OpenAI format
		openaiChunk := transformers.GeminiStreamChunkToOpenAI(geminiChunk, request.Model, responseID)
		jsonData, _ := json.Marshal(openaiChunk)
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		flusher.Flush()
	}

	// Send the final [DONE] marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	log.Printf("Completed streaming response: %s", responseID)
}

func handleNonStreamingChatCompletion(w http.ResponseWriter, r *http.Request, request *models.OpenAIChatCompletionRequest, geminiPayload map[string]interface{}) {
	// Send request to Gemini API
	result, err := client.SendGeminiRequest(geminiPayload, false)
	if err != nil {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Request failed: %v", err),
				"type":    "api_error",
				"code":    500,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorData)
		return
	}

	geminiResponse, ok := result.(map[string]interface{})
	if !ok {
		errorData := map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Invalid response from API",
				"type":    "api_error",
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

	// Transform to OpenAI format
	openaiResponse := transformers.GeminiResponseToOpenAI(geminiResponse, request.Model)

	log.Printf("Successfully processed non-streaming response for model: %s", request.Model)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openaiResponse)
}

// HandleListModels handles OpenAI-compatible models endpoint
func HandleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":{"message":"Method not allowed","type":"invalid_request_error","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	log.Println("OpenAI models list requested")

	// Convert Gemini models to OpenAI format
	openaiModels := make([]map[string]interface{}, 0)
	for _, model := range config.SupportedModels {
		modelID := strings.TrimPrefix(model.Name, "models/")

		// Add base model
		openaiModels = append(openaiModels, map[string]interface{}{
			"id":       modelID,
			"object":   "model",
			"created":  1677610602,
			"owned_by": "google",
			"permission": []map[string]interface{}{
				{
					"id":                   "modelperm-" + strings.ReplaceAll(modelID, "/", "-"),
					"object":               "model_permission",
					"created":              1677610602,
					"allow_create_engine":  false,
					"allow_sampling":       true,
					"allow_logprobs":       false,
					"allow_search_indices": false,
					"allow_view":           true,
					"allow_fine_tuning":    false,
					"organization":         "*",
					"group":                nil,
					"is_blocking":          false,
				},
			},
			"root":   modelID,
			"parent": nil,
		})

		// Add -fake variant with same metadata
		fakeModelID := modelID + "-fake"
		openaiModels = append(openaiModels, map[string]interface{}{
			"id":       fakeModelID,
			"object":   "model",
			"created":  1677610602,
			"owned_by": "google",
			"permission": []map[string]interface{}{
				{
					"id":                   "modelperm-" + strings.ReplaceAll(fakeModelID, "/", "-"),
					"object":               "model_permission",
					"created":              1677610602,
					"allow_create_engine":  false,
					"allow_sampling":       true,
					"allow_logprobs":       false,
					"allow_search_indices": false,
					"allow_view":           true,
					"allow_fine_tuning":    false,
					"organization":         "*",
					"group":                nil,
					"is_blocking":          false,
				},
			},
			"root":   fakeModelID,
			"parent": nil,
		})
	}

	log.Printf("Returning %d models (including -fake variants)", len(openaiModels))

	response := map[string]interface{}{
		"object": "list",
		"data":   openaiModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
