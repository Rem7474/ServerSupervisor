package models

import (
	"errors"
	"strings"
	"time"
)

// ErrHostGroupInUse is returned when deleting a group an alert rule still
// targets (the foreign key refused the delete).
var ErrHostGroupInUse = errors.New("host group is targeted by alert rules")

// Host group match modes: a host must carry every tag of the group ("all")
// or at least one of them ("any").
const (
	HostGroupMatchAll = "all"
	HostGroupMatchAny = "any"
)

// HostGroup is a dynamic set of hosts defined by a matching rule over
// Host.Tags (and optionally Host.OS), never by a stored host list: who belongs
// to it is computed on every read and every alert evaluation, so tagging a
// new host is enough for it to join the group and every rule targeting it.
type HostGroup struct {
	ID          string `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	// MatchMode is HostGroupMatchAll or HostGroupMatchAny.
	MatchMode string   `json:"match_mode" db:"match_mode"`
	Tags      []string `json:"tags" db:"tags"`
	// OSContains, when set, additionally requires Host.OS to contain this
	// text (case-insensitive), e.g. "debian".
	OSContains string    `json:"os_contains" db:"os_contains"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	// MemberIDs is computed at read time from the current hosts, not stored.
	MemberIDs []string `json:"member_ids" db:"-"`
}

// HostGroupRequest is the body of POST/PUT /host-groups.
type HostGroupRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	MatchMode   string   `json:"match_mode"`
	Tags        []string `json:"tags"`
	OSContains  string   `json:"os_contains"`
}

// NormalizeTags trims every tag, drops empty ones and removes duplicates
// (compared case-insensitively, first spelling kept). Never returns nil.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

// HostHasTag reports whether the host carries tag (case-insensitive).
func HostHasTag(h Host, tag string) bool {
	for _, t := range h.Tags {
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(tag)) {
			return true
		}
	}
	return false
}

// HostHasAnyTag reports whether the host carries at least one of tags.
func HostHasAnyTag(h Host, tags []string) bool {
	for _, tag := range tags {
		if HostHasTag(h, tag) {
			return true
		}
	}
	return false
}

// Matches reports whether a host currently belongs to the group. A group
// with neither tags nor an OS filter matches nothing: the service refuses to
// save one, and an empty rule must never silently mean "every host".
func (g HostGroup) Matches(h Host) bool {
	tags := NormalizeTags(g.Tags)
	osFilter := strings.TrimSpace(g.OSContains)
	if len(tags) == 0 && osFilter == "" {
		return false
	}
	if osFilter != "" && !strings.Contains(strings.ToLower(h.OS), strings.ToLower(osFilter)) {
		return false
	}
	if len(tags) == 0 {
		return true
	}
	if g.MatchMode == HostGroupMatchAny {
		return HostHasAnyTag(h, tags)
	}
	for _, tag := range tags {
		if !HostHasTag(h, tag) {
			return false
		}
	}
	return true
}

// Members returns the hosts that currently belong to the group, in input order.
func (g HostGroup) Members(hosts []Host) []Host {
	out := make([]Host, 0)
	for _, h := range hosts {
		if g.Matches(h) {
			out = append(out, h)
		}
	}
	return out
}
