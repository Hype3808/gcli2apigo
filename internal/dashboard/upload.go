package dashboard

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"gcli2apigo/internal/config"
)

// HandleJSONUpload processes a single JSON credential file
func HandleJSONUpload(file multipart.File, filename string) (int, error) {
	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read file: %v", err)
	}

	// Validate JSON structure
	var credData map[string]interface{}
	if err := json.Unmarshal(content, &credData); err != nil {
		return 0, fmt.Errorf("invalid JSON format: %v", err)
	}

	// Extract project_id
	projectID, ok := credData["project_id"].(string)
	if !ok || projectID == "" {
		return 0, fmt.Errorf("missing or invalid project_id in credential file")
	}

	// Validate project_id
	if err := ValidateProjectID(projectID); err != nil {
		return 0, fmt.Errorf("invalid project_id: %v", err)
	}

	// Ensure oauth_creds directory exists
	credsDir := config.OAuthCredsFolder
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to create credentials directory: %v", err)
	}

	// Save credential file
	filePath := filepath.Join(credsDir, projectID+".json")

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		log.Printf("[WARN] Credential file already exists for project %s, overwriting", projectID)
	}

	// Write file with proper permissions
	if err := os.WriteFile(filePath, content, 0600); err != nil {
		return 0, fmt.Errorf("failed to save credential file: %v", err)
	}

	log.Printf("[INFO] Successfully saved credential for project: %s (from %s)", projectID, filename)
	return 1, nil
}

// HandleZIPUpload processes a ZIP file containing multiple JSON credentials
func HandleZIPUpload(file multipart.File, size int64) (int, error) {
	// Read ZIP file into memory
	content, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read ZIP file: %v", err)
	}

	// Create ZIP reader
	zipReader, err := zip.NewReader(bytes.NewReader(content), size)
	if err != nil {
		return 0, fmt.Errorf("failed to open ZIP file: %v", err)
	}

	// Ensure oauth_creds directory exists
	credsDir := config.OAuthCredsFolder
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		return 0, fmt.Errorf("failed to create credentials directory: %v", err)
	}

	count := 0
	errors := []string{}

	// Process each file in the ZIP
	for _, zipFile := range zipReader.File {
		// Skip directories
		if zipFile.FileInfo().IsDir() {
			continue
		}

		// Only process JSON files
		if !strings.HasSuffix(strings.ToLower(zipFile.Name), ".json") {
			log.Printf("[WARN] Skipping non-JSON file in ZIP: %s", zipFile.Name)
			continue
		}

		// Skip hidden files and __MACOSX folder
		if strings.HasPrefix(filepath.Base(zipFile.Name), ".") || strings.Contains(zipFile.Name, "__MACOSX") {
			continue
		}

		// Open file in ZIP
		rc, err := zipFile.Open()
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to open %s: %v", zipFile.Name, err))
			continue
		}

		// Read file content
		fileContent, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to read %s: %v", zipFile.Name, err))
			continue
		}

		// Validate JSON structure
		var credData map[string]interface{}
		if err := json.Unmarshal(fileContent, &credData); err != nil {
			errors = append(errors, fmt.Sprintf("invalid JSON in %s: %v", zipFile.Name, err))
			continue
		}

		// Extract project_id
		projectID, ok := credData["project_id"].(string)
		if !ok || projectID == "" {
			errors = append(errors, fmt.Sprintf("missing project_id in %s", zipFile.Name))
			continue
		}

		// Validate project_id
		if err := ValidateProjectID(projectID); err != nil {
			errors = append(errors, fmt.Sprintf("invalid project_id in %s: %v", zipFile.Name, err))
			continue
		}

		// Save credential file
		filePath := filepath.Join(credsDir, projectID+".json")

		// Check if file already exists
		if _, err := os.Stat(filePath); err == nil {
			log.Printf("[WARN] Credential file already exists for project %s, overwriting", projectID)
		}

		// Write file with proper permissions
		if err := os.WriteFile(filePath, fileContent, 0600); err != nil {
			errors = append(errors, fmt.Sprintf("failed to save %s: %v", zipFile.Name, err))
			continue
		}

		log.Printf("[INFO] Successfully saved credential for project: %s (from %s)", projectID, zipFile.Name)
		count++
	}

	if count == 0 {
		if len(errors) > 0 {
			return 0, fmt.Errorf("no credentials saved. Errors: %s", strings.Join(errors, "; "))
		}
		return 0, fmt.Errorf("no valid JSON credential files found in ZIP")
	}

	if len(errors) > 0 {
		log.Printf("[WARN] Uploaded %d credentials with %d errors: %s", count, len(errors), strings.Join(errors, "; "))
	}

	return count, nil
}
