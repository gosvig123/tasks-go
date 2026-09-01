package gist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	githubAPIURL = "https://api.github.com"
	configFile   = ".tasks-gist-config"
)

// Config holds gist sync configuration
type Config struct {
	Token  string `json:"token"`
	GistID string `json:"gist_id"`
}

// Client handles GitHub Gist API operations
type Client struct {
	config     *Config
	configPath string
	httpClient *http.Client
}

// GistFile represents a file in a gist
type GistFile struct {
	Filename string `json:"filename,omitempty"`
	Content  string `json:"content"`
}

// Gist represents a GitHub Gist
type Gist struct {
	ID          string              `json:"id,omitempty"`
	Description string              `json:"description"`
	Public      bool                `json:"public"`
	Files       map[string]GistFile `json:"files"`
	HTMLURL     string              `json:"html_url,omitempty"`
	CreatedAt   string              `json:"created_at,omitempty"`
	UpdatedAt   string              `json:"updated_at,omitempty"`
}

// NewClient creates a new gist client
func NewClient() *Client {
	home, _ := os.UserHomeDir()
	return &Client{
		configPath: filepath.Join(home, configFile),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// LoadConfig loads the gist configuration
func (c *Client) LoadConfig() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("gist not configured. Run 'tasks sync init' first")
		}
		return err
	}

	c.config = &Config{}
	return json.Unmarshal(data, c.config)
}

// SaveConfig saves the gist configuration
func (c *Client) SaveConfig(cfg *Config) error {
	c.config = cfg
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configPath, data, 0600)
}

// IsConfigured checks if gist sync is configured
func (c *Client) IsConfigured() bool {
	if err := c.LoadConfig(); err != nil {
		return false
	}
	return c.config.Token != "" && c.config.GistID != ""
}

// Init initializes gist sync with a GitHub token
func (c *Client) Init(token string, tasksDir string) (*Gist, error) {
	c.config = &Config{Token: token}

	// Create initial gist with all task files
	files, err := c.readTaskFiles(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read task files: %w", err)
	}

	if len(files) == 0 {
		// Create a placeholder file if no tasks exist
		files["_tasks-backup-info.md"] = GistFile{
			Content: fmt.Sprintf("# Tasks Backup\n\nInitialized: %s\n\nThis gist is automatically synced from tasks-go.\n", time.Now().Format("2006-01-02 15:04:05")),
		}
	}

	gist := &Gist{
		Description: "Tasks Backup - tasks-go",
		Public:      false, // Secret gist
		Files:       files,
	}

	created, err := c.createGist(gist)
	if err != nil {
		return nil, err
	}

	c.config.GistID = created.ID
	if err := c.SaveConfig(c.config); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return created, nil
}

// Sync syncs all task files to the gist
func (c *Client) Sync(tasksDir string) (*Gist, error) {
	if err := c.LoadConfig(); err != nil {
		return nil, err
	}

	files, err := c.syncedFiles(tasksDir)
	if err != nil {
		return nil, err
	}
	gist := &Gist{Description: "Tasks Backup - tasks-go", Files: files}
	return c.updateOrGitSync(gist, files)
}

func (c *Client) syncedFiles(tasksDir string) (map[string]GistFile, error) {
	files, err := c.readTaskFiles(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read task files: %w", err)
	}
	files["_last-sync.md"] = GistFile{
		Content: fmt.Sprintf("Last synced: %s\n", time.Now().Format("2006-01-02 15:04:05")),
	}
	return files, nil
}

func (c *Client) updateOrGitSync(gist *Gist, files map[string]GistFile) (*Gist, error) {
	updated, err := c.updateGist(gist)
	if err == nil {
		return updated, nil
	}
	if syncErr := c.syncWithGit(files); syncErr != nil {
		return nil, fmt.Errorf("gist API sync failed: %w; git fallback failed: %v", err, syncErr)
	}
	return c.getGist()
}

// UpdateToken replaces the stored GitHub token without changing the gist.
func (c *Client) UpdateToken(token string) error {
	if err := c.LoadConfig(); err != nil {
		return err
	}
	c.config.Token = token
	return c.SaveConfig(c.config)
}

// Status returns the current sync status
func (c *Client) Status() (*Gist, error) {
	if err := c.LoadConfig(); err != nil {
		return nil, err
	}
	return c.getGist()
}

// readTaskFiles reads all .md files from the tasks directory
func (c *Client) readTaskFiles(tasksDir string) (map[string]GistFile, error) {
	files := make(map[string]GistFile)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		files[entry.Name()] = GistFile{
			Content: string(content),
		}
	}

	return files, nil
}

// doGistRequest performs a gist API request and returns the response
func (c *Client) doGistRequest(method, url string, body interface{}, expectedStatus int) (*Gist, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gist API error: %s - %s", resp.Status, string(respBody))
	}

	var g Gist
	if err := json.NewDecoder(resp.Body).Decode(&g); err != nil {
		return nil, err
	}

	return &g, nil
}

// createGist creates a new gist
func (c *Client) createGist(gist *Gist) (*Gist, error) {
	return c.doGistRequest("POST", githubAPIURL+"/gists", gist, http.StatusCreated)
}

// updateGist updates an existing gist
func (c *Client) updateGist(gist *Gist) (*Gist, error) {
	url := fmt.Sprintf("%s/gists/%s", githubAPIURL, c.config.GistID)
	return c.doGistRequest("PATCH", url, gist, http.StatusOK)
}

// getGist retrieves the current gist
func (c *Client) getGist() (*Gist, error) {
	url := fmt.Sprintf("%s/gists/%s", githubAPIURL, c.config.GistID)
	return c.doGistRequest("GET", url, nil, http.StatusOK)
}
