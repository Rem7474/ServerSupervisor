export interface GitRepoRef {
  provider?: string
  repo_owner?: string
  repo_name?: string
  git_base_url?: string
}

const PUBLIC_INSTANCES: Record<string, string> = {
  github: 'https://github.com',
  gitlab: 'https://gitlab.com',
  gitea: 'https://codeberg.org',
}

/** Web root of the tracker's git instance (self-hosted base URL, else the public one). */
export function gitInstanceRoot(ref: GitRepoRef): string {
  const provider = ref.provider || 'github'
  if (provider !== 'github' && ref.git_base_url) return ref.git_base_url.replace(/\/+$/, '')
  return PUBLIC_INSTANCES[provider] ?? PUBLIC_INSTANCES.github
}

/** Repository page URL, or '#' when no repository is configured. */
export function gitRepoUrl(ref: GitRepoRef): string {
  if (!ref.repo_owner || !ref.repo_name) return '#'
  return `${gitInstanceRoot(ref)}/${ref.repo_owner}/${ref.repo_name}`
}

/** Releases page URL (GitLab nests it under /-/). */
export function gitReleasesUrl(ref: GitRepoRef): string {
  const repo = gitRepoUrl(ref)
  if (repo === '#') return repo
  return ref.provider === 'gitlab' ? `${repo}/-/releases` : `${repo}/releases`
}

/** True for providers that can run on a self-hosted instance. */
export function isSelfHostable(provider: string | undefined): boolean {
  return provider === 'gitlab' || provider === 'gitea'
}

/** Client-side sanity check; the server stays authoritative (private ranges, loopback...). */
export function isValidGitBaseUrl(raw: string): boolean {
  const value = raw.trim()
  if (!value) return true
  try {
    const u = new URL(value)
    return (u.protocol === 'http:' || u.protocol === 'https:') && !u.username && !u.password && !u.search && !u.hash
  } catch {
    return false
  }
}
