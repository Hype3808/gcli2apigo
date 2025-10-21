package banlist

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"gcli2apigo/internal/config"
)

// BanList manages banned credentials
type BanList struct {
	bannedProjects map[string]bool
	mu             sync.RWMutex
	storePath      string
}

var (
	globalBanList *BanList
	banListOnce   sync.Once
)

// GetBanList returns the global ban list instance
func GetBanList() *BanList {
	banListOnce.Do(func() {
		globalBanList = NewBanList()
		globalBanList.Load()
	})
	return globalBanList
}

// NewBanList creates a new ban list
func NewBanList() *BanList {
	storePath := filepath.Join(config.OAuthCredsFolder, "banlist.json")
	return &BanList{
		bannedProjects: make(map[string]bool),
		storePath:      storePath,
	}
}

// IsBanned checks if a project is banned
func (bl *BanList) IsBanned(projectID string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return bl.bannedProjects[projectID]
}

// Ban adds a project to the ban list
func (bl *BanList) Ban(projectID string) error {
	bl.mu.Lock()
	bl.bannedProjects[projectID] = true
	bl.mu.Unlock()

	log.Printf("[INFO] Banned credential: %s", projectID)
	return bl.Save()
}

// Unban removes a project from the ban list
func (bl *BanList) Unban(projectID string) error {
	bl.mu.Lock()
	delete(bl.bannedProjects, projectID)
	bl.mu.Unlock()

	log.Printf("[INFO] Unbanned credential: %s", projectID)
	return bl.Save()
}

// BanMultiple bans multiple projects
func (bl *BanList) BanMultiple(projectIDs []string) error {
	bl.mu.Lock()
	for _, projectID := range projectIDs {
		bl.bannedProjects[projectID] = true
	}
	bl.mu.Unlock()

	log.Printf("[INFO] Banned %d credentials", len(projectIDs))
	return bl.Save()
}

// UnbanMultiple unbans multiple projects
func (bl *BanList) UnbanMultiple(projectIDs []string) error {
	bl.mu.Lock()
	for _, projectID := range projectIDs {
		delete(bl.bannedProjects, projectID)
	}
	bl.mu.Unlock()

	log.Printf("[INFO] Unbanned %d credentials", len(projectIDs))
	return bl.Save()
}

// GetBannedProjects returns all banned project IDs
func (bl *BanList) GetBannedProjects() []string {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	projects := make([]string, 0, len(bl.bannedProjects))
	for projectID := range bl.bannedProjects {
		projects = append(projects, projectID)
	}
	return projects
}

// Save persists the ban list to disk
func (bl *BanList) Save() error {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(bl.storePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		log.Printf("[ERROR] Failed to create ban list directory: %v", err)
		return err
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(bl.bannedProjects, "", "  ")
	if err != nil {
		log.Printf("[ERROR] Failed to marshal ban list: %v", err)
		return err
	}

	// Write to file
	if err := os.WriteFile(bl.storePath, data, 0600); err != nil {
		log.Printf("[ERROR] Failed to write ban list: %v", err)
		return err
	}

	return nil
}

// Load reads the ban list from disk
func (bl *BanList) Load() error {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(bl.storePath); os.IsNotExist(err) {
		log.Printf("[INFO] Ban list file does not exist, starting fresh")
		return nil
	}

	// Read file
	data, err := os.ReadFile(bl.storePath)
	if err != nil {
		log.Printf("[ERROR] Failed to read ban list: %v", err)
		return err
	}

	// Unmarshal JSON
	if err := json.Unmarshal(data, &bl.bannedProjects); err != nil {
		log.Printf("[ERROR] Failed to unmarshal ban list: %v", err)
		return err
	}

	log.Printf("[INFO] Loaded ban list with %d banned credentials", len(bl.bannedProjects))
	return nil
}
