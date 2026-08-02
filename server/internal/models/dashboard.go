package models

// AttentionItem is one entry in the aggregated "needs attention" feed served
// by GET /v1/dashboard/attention — signals the backend already detects but
// that are otherwise only visible by landing on the exact right page (a
// suggested Proxmox link only shows on that host's detail page, an NPM host
// with monitoring off only shows in the NPM list, ...). Mirrors the shape
// frontend/src/composables/useAttentionCenter.ts previously computed
// client-side from five separate list endpoints.
type AttentionItem struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
	To       string `json:"to"`
	Severity string `json:"severity"` // "info" | "warning"
}
