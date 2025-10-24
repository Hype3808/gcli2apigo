package usage

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Limits for API usage
const (
	ProModelDailyLimit = 100  // gemini-2.5-pro models limit
	OverallDailyLimit  = 1000 // all models limit
	ResetHour          = 15   // 3:00 PM
	ResetMinute        = 0
	ResetTimezone      = 8 * 60 * 60 // GMT+8 in seconds
)

// ProjectUsage tracks usage statistics for a single project
type ProjectUsage struct {
	ProjectID      string    `json:"project_id"`
	ProModelCount  int       `json:"pro_model_count"`
	OverallCount   int       `json:"overall_count"`
	LastResetTime  time.Time `json:"last_reset_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
	LastErrorCode  int       `json:"last_error_code"` // HTTP error code from last API request, 0 if successful
}

// UsageTracker manages usage statistics for all projects
type UsageTracker struct {
	usageMap     map[string]*ProjectUsage
	mu           sync.RWMutex
	storePath    string
	dirty        bool // Tracks if data needs to be saved
	dirtyMu      sync.Mutex
	lastSaveTime time.Time
}

var (
	globalTracker *UsageTracker
	trackerOnce   sync.Once
)

// GetTracker returns the global usage tracker instance
func GetTracker() *UsageTracker {
	trackerOnce.Do(func() {
		globalTracker = NewUsageTracker()
		globalTracker.Load()
		go globalTracker.autoResetLoop()
		go globalTracker.autoSaveLoop() // Batch save every 5 seconds
	})
	return globalTracker
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker() *UsageTracker {
	storePath := filepath.Join("oauth_creds", "usage_stats.json")
	return &UsageTracker{
		usageMap:  make(map[string]*ProjectUsage),
		storePath: storePath,
	}
}

// IncrementUsage increments usage counters for a project
func (ut *UsageTracker) IncrementUsage(projectID string, isProModel bool) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	usage, exists := ut.usageMap[projectID]
	if !exists {
		usage = &ProjectUsage{
			ProjectID:     projectID,
			LastResetTime: ut.getNextResetTime().Add(-24 * time.Hour), // Set to previous reset
		}
		ut.usageMap[projectID] = usage
	}

	// Check if reset is needed
	if ut.shouldReset(usage.LastResetTime) {
		usage.ProModelCount = 0
		usage.OverallCount = 0
		usage.LastResetTime = ut.getLastResetTime()
	}

	// Increment counters
	usage.OverallCount++
	if isProModel {
		usage.ProModelCount++
	}
	usage.LastUpdateTime = time.Now()

	// Clear error code on successful request
	usage.LastErrorCode = 0

	// Mark as dirty for batch save
	ut.markDirty()
}

// SetErrorCode sets the error code for a project
func (ut *UsageTracker) SetErrorCode(projectID string, errorCode int) {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	usage, exists := ut.usageMap[projectID]
	if !exists {
		usage = &ProjectUsage{
			ProjectID:     projectID,
			LastResetTime: ut.getNextResetTime().Add(-24 * time.Hour),
		}
		ut.usageMap[projectID] = usage
	}

	usage.LastErrorCode = errorCode
	usage.LastUpdateTime = time.Now()

	// Mark as dirty for batch save
	ut.markDirty()
}

// GetLastErrorCode returns the last error code for a project
func (ut *UsageTracker) GetLastErrorCode(projectID string) int {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	usage, exists := ut.usageMap[projectID]
	if !exists {
		return 0
	}

	return usage.LastErrorCode
}

// GetUsage returns usage statistics for a project
func (ut *UsageTracker) GetUsage(projectID string) *ProjectUsage {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	usage, exists := ut.usageMap[projectID]
	if !exists {
		return &ProjectUsage{
			ProjectID:     projectID,
			LastResetTime: ut.getLastResetTime(),
		}
	}

	// Check if reset is needed
	if ut.shouldReset(usage.LastResetTime) {
		return &ProjectUsage{
			ProjectID:     projectID,
			LastResetTime: ut.getNextResetTime().Add(-24 * time.Hour),
		}
	}

	// Return a copy
	return &ProjectUsage{
		ProjectID:      usage.ProjectID,
		ProModelCount:  usage.ProModelCount,
		OverallCount:   usage.OverallCount,
		LastResetTime:  usage.LastResetTime,
		LastUpdateTime: usage.LastUpdateTime,
	}
}

// GetAllUsage returns usage statistics for all projects
func (ut *UsageTracker) GetAllUsage() map[string]*ProjectUsage {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	result := make(map[string]*ProjectUsage)

	for projectID, usage := range ut.usageMap {
		if ut.shouldReset(usage.LastResetTime) {
			result[projectID] = &ProjectUsage{
				ProjectID:     projectID,
				LastResetTime: ut.getLastResetTime(),
			}
		} else {
			result[projectID] = &ProjectUsage{
				ProjectID:      usage.ProjectID,
				ProModelCount:  usage.ProModelCount,
				OverallCount:   usage.OverallCount,
				LastResetTime:  usage.LastResetTime,
				LastUpdateTime: usage.LastUpdateTime,
			}
		}
	}

	return result
}

// shouldReset checks if usage should be reset based on last reset time
func (ut *UsageTracker) shouldReset(lastResetTime time.Time) bool {
	// If lastResetTime is zero, it needs reset
	if lastResetTime.IsZero() {
		return true
	}

	// Get the most recent reset time (today or yesterday at 3 PM)
	lastResetPoint := ut.getLastResetTime()

	// If the last reset was before the most recent reset point, we need to reset
	return lastResetTime.Before(lastResetPoint)
}

// getLastResetTime returns the most recent reset time (today or yesterday at 3 PM GMT+8)
func (ut *UsageTracker) getLastResetTime() time.Time {
	location := time.FixedZone("GMT+8", ResetTimezone)
	now := time.Now().In(location)

	resetTime := time.Date(now.Year(), now.Month(), now.Day(), ResetHour, ResetMinute, 0, 0, location)

	if now.Before(resetTime) {
		// If current time is before today's reset, return yesterday's reset
		resetTime = resetTime.Add(-24 * time.Hour)
	}

	return resetTime
}

// GetNextResetTime returns the next reset time (today or tomorrow at 3 PM GMT+8)
func (ut *UsageTracker) GetNextResetTime() time.Time {
	return ut.getNextResetTime()
}

// getNextResetTime returns the next reset time (today or tomorrow at 3 PM GMT+8)
func (ut *UsageTracker) getNextResetTime() time.Time {
	location := time.FixedZone("GMT+8", ResetTimezone)
	now := time.Now().In(location)

	resetTime := time.Date(now.Year(), now.Month(), now.Day(), ResetHour, ResetMinute, 0, 0, location)

	if now.After(resetTime) || now.Equal(resetTime) {
		// If current time is after today's reset, return tomorrow's reset
		resetTime = resetTime.Add(24 * time.Hour)
	}

	return resetTime
}

// autoResetLoop runs a background goroutine that resets usage at 3 PM GMT+8 daily
func (ut *UsageTracker) autoResetLoop() {
	for {
		nextReset := ut.getNextResetTime()
		duration := time.Until(nextReset)

		log.Printf("[INFO] Next usage reset scheduled at: %v (in %v)", nextReset, duration)

		time.Sleep(duration)

		// Perform reset
		ut.mu.Lock()
		for _, usage := range ut.usageMap {
			usage.ProModelCount = 0
			usage.OverallCount = 0
			usage.LastResetTime = time.Now()
		}
		ut.mu.Unlock()

		log.Printf("[INFO] Usage statistics reset completed at %v", time.Now())

		// Save to disk
		ut.Save()

		// Sleep a bit to avoid immediate re-trigger
		time.Sleep(1 * time.Minute)
	}
}

// Save persists usage statistics to disk
func (ut *UsageTracker) Save() error {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(ut.storePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("[ERROR] Failed to create usage stats directory: %v", err)
		return err
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(ut.usageMap, "", "  ")
	if err != nil {
		log.Printf("[ERROR] Failed to marshal usage stats: %v", err)
		return err
	}

	// Write to file
	if err := os.WriteFile(ut.storePath, data, 0600); err != nil {
		log.Printf("[ERROR] Failed to write usage stats: %v", err)
		return err
	}

	return nil
}

// Load reads usage statistics from disk
func (ut *UsageTracker) Load() error {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(ut.storePath); os.IsNotExist(err) {
		log.Printf("[INFO] Usage stats file does not exist, starting fresh")
		return nil
	}

	// Read file
	data, err := os.ReadFile(ut.storePath)
	if err != nil {
		log.Printf("[ERROR] Failed to read usage stats: %v", err)
		return err
	}

	// Unmarshal JSON
	if err := json.Unmarshal(data, &ut.usageMap); err != nil {
		log.Printf("[ERROR] Failed to unmarshal usage stats: %v", err)
		return err
	}

	log.Printf("[INFO] Loaded usage statistics for %d projects", len(ut.usageMap))
	return nil
}

// CheckAndResetIfNeeded checks all projects and resets usage if the reset time has passed
// This should be called on startup to handle cases where the program was not running during reset time
func (ut *UsageTracker) CheckAndResetIfNeeded() {
	ut.mu.Lock()
	defer ut.mu.Unlock()

	resetCount := 0
	nextReset := ut.getNextResetTime()

	for projectID, usage := range ut.usageMap {
		if ut.shouldReset(usage.LastResetTime) {
			log.Printf("[INFO] Resetting usage for project %s (last reset: %v, next reset: %v)",
				projectID, usage.LastResetTime, nextReset)
			usage.ProModelCount = 0
			usage.OverallCount = 0
			usage.LastResetTime = ut.getLastResetTime()
			resetCount++
		}
	}

	if resetCount > 0 {
		log.Printf("[INFO] Reset usage statistics for %d projects on startup", resetCount)
		// Save the reset state to disk
		ut.mu.Unlock()
		ut.Save()
		ut.mu.Lock()
	} else {
		log.Printf("[INFO] No usage reset needed on startup")
	}
}

// IsProModel checks if a model name is a Pro model (gemini-2.5-pro variants)
func IsProModel(modelName string) bool {
	// Check if model name contains "pro" (case-insensitive)
	// This covers: gemini-2.5-pro, gemini-2.5-pro-preview-*, etc.
	return len(modelName) >= 3 &&
		(modelName[len(modelName)-3:] == "pro" ||
			len(modelName) >= 4 && modelName[len(modelName)-4:len(modelName)-1] == "pro-")
}

// markDirty marks the tracker as having unsaved changes
func (ut *UsageTracker) markDirty() {
	ut.dirtyMu.Lock()
	ut.dirty = true
	ut.dirtyMu.Unlock()
}

// autoSaveLoop runs a background goroutine that saves usage stats every 5 seconds if dirty
// This batches disk writes to improve performance under high load
func (ut *UsageTracker) autoSaveLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ut.dirtyMu.Lock()
		isDirty := ut.dirty
		ut.dirtyMu.Unlock()

		if isDirty {
			if err := ut.Save(); err != nil {
				log.Printf("[ERROR] Auto-save failed: %v", err)
			} else {
				ut.dirtyMu.Lock()
				ut.dirty = false
				ut.lastSaveTime = time.Now()
				ut.dirtyMu.Unlock()
			}
		}
	}
}
