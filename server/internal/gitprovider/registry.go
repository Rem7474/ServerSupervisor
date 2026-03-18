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
//
//	"nginx"                          → "registry-1.docker.io", "library/nginx"
//	"homeassistant/home-assistant"   → "registry-1.docker.io", "homeassistant/home-assistant"
//	"ghcr.io/org/app"               → "ghcr.io", "org/app"
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

// fetchDockerVersionForDigest finds a versioned tag matching targetDigest for the given image.
// For Docker Hub, it uses the hub.docker.com API (tags with digest in one call).
// For other registries, it enumerates tags and HEADs each semver-looking tag.
// Returns "" if resolution fails.
func fetchDockerVersionForDigest(client *http.Client, imageRef, targetDigest string) string {
	if targetDigest == "" {
		return ""
	}
	registry, image := parseDockerRegistry(imageRef)
	normTarget := strings.TrimPrefix(targetDigest, "sha256:")

	if registry == "registry-1.docker.io" {
		if v := dockerHubVersionForDigest(client, image, normTarget); v != "" {
			return v
		}
	}
	return registryVersionForDigest(client, registry, image, normTarget)
}

// dockerHubVersionForDigest queries hub.docker.com to find a versioned tag for a digest.
func dockerHubVersionForDigest(client *http.Client, image, normDigest string) string {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags?page_size=100&ordering=last_updated", image)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Results []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	for _, t := range result.Results {
		if !looksLikeVersion(t.Name) {
			continue
		}
		if strings.TrimPrefix(t.Digest, "sha256:") == normDigest {
			return t.Name
		}
	}
	return ""
}

// registryVersionForDigest uses the v2 tags/list API + HEAD manifest per tag
// to find a versioned tag matching normDigest. Checks up to 30 semver-looking tags.
func registryVersionForDigest(client *http.Client, registry, image, normDigest string) string {
	token, err := getRegistryToken(client, registry, image)
	if err != nil {
		return ""
	}

	tagsURL := fmt.Sprintf("https://%s/v2/%s/tags/list", registry, image)
	req, _ := http.NewRequest("GET", tagsURL, nil)
	req.Header.Set("User-Agent", "ServerSupervisor/1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	acceptHeader := strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", ")

	checked := 0
	for _, tag := range result.Tags {
		if !looksLikeVersion(tag) || checked >= 30 {
			continue
		}
		checked++

		hreq, _ := http.NewRequest("HEAD", fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, image, tag), nil)
		hreq.Header.Set("Accept", acceptHeader)
		hreq.Header.Set("User-Agent", "ServerSupervisor/1.0")
		if token != "" {
			hreq.Header.Set("Authorization", "Bearer "+token)
		}
		hresp, err := client.Do(hreq)
		if hresp != nil {
			_ = hresp.Body.Close()
		}
		if err != nil || hresp.StatusCode != http.StatusOK {
			continue
		}
		d := strings.TrimPrefix(hresp.Header.Get("Docker-Content-Digest"), "sha256:")
		if d == normDigest {
			return tag
		}
	}
	return ""
}

// looksLikeVersion returns true for tags that appear to be version numbers (e.g. "1.25.3", "v2.0.1").
// Rejects well-known non-version tags like "latest", "stable", "edge", "main".
func looksLikeVersion(tag string) bool {
	if tag == "" {
		return false
	}
	switch tag {
	case "latest", "stable", "edge", "nightly", "dev", "main", "master", "beta", "alpha", "rc":
		return false
	}
	t := tag
	if len(t) > 0 && t[0] == 'v' {
		t = t[1:]
	}
	if len(t) == 0 || t[0] < '0' || t[0] > '9' {
		return false
	}
	return strings.Contains(t, ".")
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
