// Command `formspec apply` registers YAML manifests to the Control Plane.
//
// It is the only way to submit YAML manifests into the two-stage pipeline.
// The Resource Plane never reads YAML from the filesystem — it pulls
// artifacts from the Control Plane.
//
// Usage:
//
//	formspec apply [flags] <spec-directory>
//
// Flags:
//
//	--control <url>   Control Plane URL (default: http://localhost:8443)
//	--app <name>      App name (default: "default")
//	--watch           Watch for file changes and auto-register
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func runApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	controlURL := fs.String("control", "http://localhost:8443", "Control Plane URL")
	appName := fs.String("app", "default", "App name")
	watchMode := fs.Bool("watch", false, "Watch for file changes and auto-register")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: formspec apply [flags] <spec-directory>\n\n")
		fmt.Fprintf(os.Stderr, "Register YAML manifests to the Control Plane.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	specArgs := fs.Args()
	if len(specArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: spec directory is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	specDir := specArgs[0]

	if err := registerDirectory(*controlURL, *appName, specDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *watchMode {
		watchForChanges(*controlURL, *appName, specDir)
	}
}

// registerDirectory discovers YAML files in a directory and registers them.
func registerDirectory(controlURL, appName, specDir string) error {
	// Validate directory
	info, err := os.Stat(specDir)
	if err != nil {
		return fmt.Errorf("spec directory %q: %w", specDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("spec path %q is not a directory", specDir)
	}

	// Discover YAML files
	var files []RegisterFile
	err = filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "impl" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".star" {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			relPath, _ := filepath.Rel(specDir, path)
			files = append(files, RegisterFile{Path: relPath, Content: data})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("discover files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .yaml or .yml files found in %s", specDir)
	}

	// Register via HTTP
	regReq := RegisterRequest{
		App:   appName,
		Files: make([]RegisterFileEntry, len(files)),
	}

	absPrefix, _ := filepath.Abs(specDir)
	for i, f := range files {
		fullPath := filepath.Join(absPrefix, f.Path)
		regReq.Files[i] = RegisterFileEntry{
			Path:    fullPath,
			Content: f.Content,
		}
	}

	body, err := json.Marshal(regReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/artifacts", controlURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var regResp RegisterResponse
	if err := json.Unmarshal(respBody, &regResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("✅ Registered %d files to Control Plane\n", len(files))
	fmt.Printf("   App:        %s\n", regResp.App)
	fmt.Printf("   Artifact:   %s\n", regResp.ArtifactID)
	fmt.Printf("   Version:    %d\n", regResp.Version)
	fmt.Printf("   SHA256:     %s\n", regResp.SHA256[:16])

	// Trigger dev poll for fast refresh
	triggerPoll(controlURL)

	return nil
}

// watchForChanges watches the spec directory for changes and re-registers.
func watchForChanges(controlURL, appName, specDir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error creating watcher: %v", err)
	}
	defer watcher.Close()

	// Add directory and subdirectories
	filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if !strings.HasPrefix(base, ".") && base != "node_modules" && base != "impl" {
				watcher.Add(path)
			}
		}
		return nil
	})

	fmt.Printf("\n👀 Watching %s for changes...\n", specDir)
	fmt.Println("   (Press Ctrl+C to stop)")

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				ext := strings.ToLower(filepath.Ext(event.Name))
				if ext == ".yaml" || ext == ".yml" || ext == ".star" {
					debounce.Reset(500 * time.Millisecond)
				}
			}
		case err := <-watcher.Errors:
			log.Printf("Watch error: %v", err)
		case <-debounce.C:
			fmt.Println("\n📝 Change detected, re-registering...")
			if err := registerDirectory(controlURL, appName, specDir); err != nil {
				log.Printf("Re-registration error: %v", err)
			}
		}
	}
}

// triggerPoll sends a POST /v1/poll to the Resource Plane for fast refresh.
func triggerPoll(controlURL string) {
	// Try the dev poll endpoint (port+1 convention)
	pollURL := strings.Replace(controlURL, ":8443", ":8081", 1)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(fmt.Sprintf("%s/v1/poll", pollURL), "application/json", nil)
	if err != nil {
		// Poll endpoint may not be available — that's fine
		return
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("   🔄 Resource Plane notified (fast refresh)")
	}
}

// Types matching the Control Plane API

type RegisterRequest struct {
	App   string              `json:"app"`
	Files []RegisterFileEntry `json:"files"`
}

type RegisterFileEntry struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}

type RegisterFile struct {
	Path    string
	Content []byte
}

type RegisterResponse struct {
	ArtifactID string `json:"artifact_id"`
	App        string `json:"app"`
	Version    int    `json:"version"`
	SHA256     string `json:"sha256"`
}
