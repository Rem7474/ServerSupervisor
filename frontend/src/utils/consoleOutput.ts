function normalizeConsoleOutput(raw: string | null | undefined): string {
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


export async function copyConsoleOutput(raw: string | null | undefined): Promise<void> {
  await navigator.clipboard.writeText(normalizeConsoleOutput(raw))
}

export function downloadConsoleOutput(raw: string | null | undefined, filename: string): void {
  const blob = new Blob([normalizeConsoleOutput(raw)], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
