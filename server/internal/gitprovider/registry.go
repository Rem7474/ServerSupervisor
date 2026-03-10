package gitprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// fetchDockerManifestDigest queries the Docker registry for the manifest digest
// of imageRef:tag (e.g. "nginx:v1.25.3" or "ghcr.io/org/app:v2.0.0").
// Returns the digest without "sha256:" prefix, e.g. "f88cbb90...".
func fetchDockerManifestDigest(client *http.Client, imageRef, tag string) (string, error) {
	registry, image := parseDockerRegistry(imageRef)

	token, err := getRegistryToken(client, registry, image)
	if err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}

	manifestURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, image, tag)
	req, _ := http.NewRequest("GET", manifestURL, nil)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry %s returned status %d for %s:%s", registry, resp.StatusCode, image, tag)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	// Strip "sha256:" prefix for storage
	if after, ok := strings.CutPrefix(digest, "sha256:"); ok {
		return after, nil
	}
	return digest, nil
}

// parseDockerRegistry splits a Docker image reference into registry and image name.
// Examples:
//   "nginx"                          → "registry-1.docker.io", "library/nginx"
//   "homeassistant/home-assistant"   → "registry-1.docker.io", "homeassistant/home-assistant"
//   "ghcr.io/org/app"               → "ghcr.io", "org/app"
func parseDockerRegistry(imageRef string) (registry, image string) {
	parts := strings.SplitN(imageRef, "/", 2)
	if len(parts) == 2 {
		first := parts[0]
		if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
			return first, parts[1]
		}
	}
	// Docker Hub
	if !strings.Contains(imageRef, "/") {
		return "registry-1.docker.io", "library/" + imageRef
	}
	return "registry-1.docker.io", imageRef
}

// getRegistryToken fetches an anonymous pull token for the given registry and image.
func getRegistryToken(client *http.Client, registry, image string) (string, error) {
	var authURL string
	switch registry {
	case "registry-1.docker.io":
		authURL = fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", image)
	case "ghcr.io":
		authURL = fmt.Sprintf("https://ghcr.io/token?scope=repository:%s:pull&service=ghcr.io", image)
	default:
		// For unknown registries, attempt unauthenticated access
		return "", nil
	}

	req, _ := http.NewRequest("GET", authURL, nil)
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}
