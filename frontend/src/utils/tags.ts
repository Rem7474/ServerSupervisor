/**
 * Shared helpers for the free-text "tags" field on hosts (comma-separated in
 * every form, string[] on the wire).
 */

/** Parse a comma-separated input string into a trimmed, non-empty tag list. */
export function parseTagsInput(raw: string): string[] {
  return raw
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)
}

/** Join a tag list back into the comma-separated string an input displays. */
export function formatTagsInput(tags: string[] | null | undefined): string {
  return (tags || []).join(', ')
}

interface TaggedHost { tags?: string[] | null; os?: string | null }

interface HostGroupRule {
  match_mode: string
  tags: string[]
  os_contains: string
}

const norm = (tag: string): string => tag.trim().toLowerCase()

/** Whether the host carries the tag, compared case-insensitively like the server. */
export function hostHasTag(host: TaggedHost, tag: string): boolean {
  const wanted = norm(tag)
  return (host.tags || []).some((t) => norm(t) === wanted)
}

/**
 * Client-side mirror of the server's HostGroup.Matches, used only to preview
 * membership while a group is being edited (the server stays the reference
 * for alert evaluation). No tag and no OS filter matches nothing.
 */
export function hostMatchesGroup(host: TaggedHost, group: HostGroupRule): boolean {
  const tags = group.tags.map((t) => t.trim()).filter(Boolean)
  const osFilter = group.os_contains.trim().toLowerCase()
  if (tags.length === 0 && !osFilter) return false
  if (osFilter && !(host.os || '').toLowerCase().includes(osFilter)) return false
  if (tags.length === 0) return true
  return group.match_mode === 'any'
    ? tags.some((tag) => hostHasTag(host, tag))
    : tags.every((tag) => hostHasTag(host, tag))
}

/** Sorted, case-insensitively de-duplicated union of the hosts' tags. */
export function collectHostTags(hosts: TaggedHost[]): string[] {
  const byKey = new Map<string, string>()
  for (const host of hosts) {
    for (const tag of host.tags || []) {
      const trimmed = tag.trim()
      if (trimmed && !byKey.has(norm(trimmed))) byKey.set(norm(trimmed), trimmed)
    }
  }
  return [...byKey.values()].sort((a, b) => a.localeCompare(b))
}
