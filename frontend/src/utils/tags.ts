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
