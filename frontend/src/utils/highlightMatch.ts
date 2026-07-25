export interface MatchPart {
  text: string
  matched: boolean
}

// Splits `text` into segments so the caller can render matched substrings
// distinctly (e.g. <mark>) without v-html — label/sublabel values here can
// come from agent-reported container/host names, so they're untrusted
// strings and must never be interpreted as HTML.
export function highlightParts(text: string, query: string): MatchPart[] {
  if (!text) return []
  const q = query.trim()
  if (!q) return [{ text, matched: false }]

  const lowerText = text.toLowerCase()
  const lowerQuery = q.toLowerCase()
  const parts: MatchPart[] = []
  let cursor = 0
  let idx = lowerText.indexOf(lowerQuery, cursor)
  if (idx === -1) return [{ text, matched: false }]

  while (idx !== -1) {
    if (idx > cursor) parts.push({ text: text.slice(cursor, idx), matched: false })
    parts.push({ text: text.slice(idx, idx + q.length), matched: true })
    cursor = idx + q.length
    idx = lowerText.indexOf(lowerQuery, cursor)
  }
  if (cursor < text.length) parts.push({ text: text.slice(cursor), matched: false })
  return parts
}
