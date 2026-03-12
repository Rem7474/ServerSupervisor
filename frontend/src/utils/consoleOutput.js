function escapeHtml(value) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

function normalizeAnsiCodes(rawCodes) {
  return rawCodes
    .split(';')
    .map(code => Number.parseInt(code, 10))
    .filter(code => !Number.isNaN(code))
}

function statusLineColor(line) {
  const lower = line.toLowerCase()
  if (/\berror\b|err:|erreur|failed|echec|exception/i.test(lower)) return 'var(--tblr-danger)'
  if (/\bwarn(?:ing)?\b|attention|deprecated/i.test(lower)) return 'var(--tblr-warning)'
  if (/\bsuccess\b|done|ok\b|completed|✓/i.test(lower)) return 'var(--tblr-success)'
  return ''
}

function applyAnsiCode(style, code) {
  if (code === 0) return { color: '', backgroundColor: '', fontWeight: '' }
  if (code === 1) return { ...style, fontWeight: '700' }

  const foregroundColors = {
    30: '#94a3b8',
    31: 'var(--tblr-danger)',
    32: 'var(--tblr-success)',
    33: 'var(--tblr-warning)',
    34: '#60a5fa',
    35: '#f472b6',
    36: '#22d3ee',
    37: '#e2e8f0',
    90: '#94a3b8',
    91: '#f87171',
    92: '#4ade80',
    93: '#facc15',
    94: '#60a5fa',
    95: '#f472b6',
    96: '#22d3ee',
    97: '#f8fafc',
  }

  const backgroundColors = {
    40: '#0f172a',
    41: 'rgba(239, 68, 68, 0.18)',
    42: 'rgba(34, 197, 94, 0.18)',
    43: 'rgba(245, 158, 11, 0.18)',
    44: 'rgba(59, 130, 246, 0.18)',
    45: 'rgba(236, 72, 153, 0.18)',
    46: 'rgba(6, 182, 212, 0.18)',
    47: 'rgba(226, 232, 240, 0.18)',
    100: 'rgba(148, 163, 184, 0.22)',
    101: 'rgba(248, 113, 113, 0.22)',
    102: 'rgba(74, 222, 128, 0.22)',
    103: 'rgba(250, 204, 21, 0.22)',
    104: 'rgba(96, 165, 250, 0.22)',
    105: 'rgba(244, 114, 182, 0.22)',
    106: 'rgba(34, 211, 238, 0.22)',
    107: 'rgba(248, 250, 252, 0.22)',
  }

  if (foregroundColors[code]) return { ...style, color: foregroundColors[code] }
  if (backgroundColors[code]) return { ...style, backgroundColor: backgroundColors[code] }
  if (code === 22) return { ...style, fontWeight: '' }
  if (code === 39) return { ...style, color: '' }
  if (code === 49) return { ...style, backgroundColor: '' }
  return style
}

function styleToString(style) {
  return Object.entries(style)
    .filter(([, value]) => Boolean(value))
    .map(([key, value]) => `${key.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`)}:${value}`)
    .join(';')
}

function colorizeLine(line) {
  const ansiPattern = /(\u001b\[[0-9;]*m)/g
  const parts = line.split(ansiPattern)
  let style = { color: '', backgroundColor: '', fontWeight: '' }
  let html = ''
  let hasAnsi = false

  for (const part of parts) {
    if (!part) continue
    const ansiMatch = part.match(/^\u001b\[([0-9;]*)m$/)
    if (ansiMatch) {
      hasAnsi = true
      const codes = normalizeAnsiCodes(ansiMatch[1] || '0')
      style = codes.length ? codes.reduce(applyAnsiCode, style) : applyAnsiCode(style, 0)
      continue
    }

    const escaped = escapeHtml(part)
    const styleString = styleToString(style)
    html += styleString ? `<span style="${styleString}">${escaped}</span>` : escaped
  }

  if (hasAnsi) return html

  const escapedLine = escapeHtml(line)
  const keywordColor = statusLineColor(line)
  return keywordColor ? `<span style="color:${keywordColor}">${escapedLine}</span>` : escapedLine
}

export function normalizeConsoleOutput(raw) {
  if (!raw) return ''
  const lines = ['']
  let currentLine = ''

  for (let index = 0; index < raw.length; index += 1) {
    const character = raw[index]
    if (character === '\r') {
      currentLine = ''
      lines[lines.length - 1] = ''
      continue
    }
    if (character === '\n') {
      currentLine = ''
      lines.push('')
      continue
    }
    currentLine += character
    lines[lines.length - 1] = currentLine
  }

  return lines.join('\n')
}

export function colorizeConsoleOutput(raw) {
  const plain = normalizeConsoleOutput(raw)
  if (!plain) return ''
  return plain.split('\n').map(colorizeLine).join('\n')
}

export async function copyConsoleOutput(raw) {
  await navigator.clipboard.writeText(normalizeConsoleOutput(raw))
}

export function downloadConsoleOutput(raw, filename) {
  const blob = new Blob([normalizeConsoleOutput(raw)], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}