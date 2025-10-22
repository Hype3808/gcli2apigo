package routes

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"gcli2apigo/internal/auth"
	"gcli2apigo/internal/client"
	"gcli2apigo/internal/config"
	"gcli2apigo/internal/models"
	"gcli2apigo/internal/transformers"

	"github.com/google/uuid"
)

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

	// Transform OpenAI request to Gemini format
	geminiRequestData := transformers.OpenAIRequestToGemini(&request)

	// Build the payload for Google API
	geminiPayload := client.BuildGeminiPayloadFromOpenAI(geminiRequestData)

	if request.Stream {
		handleStreamingChatCompletion(w, r, &request, geminiPayload)
	} else {
		handleNonStreamingChatCompletion(w, r, &request, geminiPayload)
	}
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
	// Authenticate user
	if _, err := auth.AuthenticateUser(r); err != nil {
		http.Error(w, `{"error":{"message":"Invalid authentication credentials","type":"invalid_request_error","code":401}}`, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, `{"error":{"message":"Method not allowed","type":"invalid_request_error","code":405}}`, http.StatusMethodNotAllowed)
		return
	}

	log.Println("OpenAI models list requested")

	// Convert Gemini models to OpenAI format
	openaiModels := make([]map[string]interface{}, 0)
	for _, model := range config.SupportedModels {
		modelID := strings.TrimPrefix(model.Name, "models/")
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
	}

	log.Printf("Returning %d models", len(openaiModels))

	response := map[string]interface{}{
		"object": "list",
		"data":   openaiModels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
