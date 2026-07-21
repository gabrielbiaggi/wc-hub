package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type RouteInfo struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	Module         string `json:"module"`
	OpenAPICovered bool   `json:"openapi_covered"`
}

var routePattern = regexp.MustCompile(`(?:HandleFunc|Handle)\(\s*"(?:(GET|POST|PUT|PATCH|DELETE) )(/api/[^" ]+)`)
var pathPattern = regexp.MustCompile(`(?m)^  (/[^:]+):\s*$`)
var operationPattern = regexp.MustCompile(`(?m)^    (get|post|put|patch|delete):\s*$`)

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fatal(err)
	}
	documented, err := openAPIOperations(filepath.Join(root, "openapi.yaml"))
	if err != nil {
		fatal(err)
	}
	routes := map[string]RouteInfo{}
	err = filepath.WalkDir(filepath.Join(root, "back", "internal"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, match := range routePattern.FindAllStringSubmatch(string(body), -1) {
			key := strings.ToLower(match[1]) + " " + match[2]
			parts := strings.Split(strings.TrimPrefix(match[2], "/api/v1/"), "/")
			module := parts[0]
			routes[key] = RouteInfo{Method: match[1], Path: match[2], Module: module, OpenAPICovered: documented[key]}
		}
		return nil
	})
	if err != nil {
		fatal(err)
	}
	items := make([]RouteInfo, 0, len(routes))
	for _, route := range routes {
		items = append(items, route)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Method+items[i].Path < items[j].Method+items[j].Path })
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"total_endpoints": len(items), "routes": items})
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "openapi.yaml")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("openapi.yaml not found from working directory")
		}
		dir = parent
	}
}

func openAPIOperations(path string) (map[string]bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	current := ""
	for _, line := range strings.Split(string(body), "\n") {
		if match := pathPattern.FindStringSubmatch(line); match != nil {
			current = match[1]
			continue
		}
		if match := operationPattern.FindStringSubmatch(line); match != nil && current != "" {
			result[match[1]+" "+current] = true
		}
	}
	return result, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
