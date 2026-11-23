import { describe, it, expect } from 'vitest'
import { gitRepoUrl, gitReleasesUrl, gitInstanceRoot, isSelfHostable, isValidGitBaseUrl } from './gitRepoLinks'

const ref = { repo_owner: 'grp', repo_name: 'app' }

describe('gitRepoLinks', () => {
  it('uses the public instance by default', () => {
    expect(gitRepoUrl({ ...ref, provider: 'github' })).toBe('https://github.com/grp/app')
    expect(gitRepoUrl({ ...ref, provider: 'gitlab' })).toBe('https://gitlab.com/grp/app')
    expect(gitRepoUrl({ ...ref, provider: 'gitea' })).toBe('https://codeberg.org/grp/app')
    expect(gitRepoUrl(ref)).toBe('https://github.com/grp/app')
  })

  it('uses the self-hosted base URL for gitlab and gitea, ignoring a trailing slash', () => {
    expect(gitRepoUrl({ ...ref, provider: 'gitlab', git_base_url: 'https://git.lan/gl/' })).toBe('https://git.lan/gl/grp/app')
    expect(gitInstanceRoot({ provider: 'gitea', git_base_url: 'http://10.0.0.5:3000' })).toBe('http://10.0.0.5:3000')
  })

  it('never applies a base URL to github', () => {
    expect(gitRepoUrl({ ...ref, provider: 'github', git_base_url: 'https://git.lan' })).toBe('https://github.com/grp/app')
  })

  it('builds the releases page per provider', () => {
    expect(gitReleasesUrl({ ...ref, provider: 'gitlab', git_base_url: 'https://git.lan' })).toBe('https://git.lan/grp/app/-/releases')
    expect(gitReleasesUrl({ ...ref, provider: 'gitea' })).toBe('https://codeberg.org/grp/app/releases')
  })

  it('returns # without a repository', () => {
    expect(gitRepoUrl({ provider: 'gitlab' })).toBe('#')
    expect(gitReleasesUrl({ provider: 'gitlab' })).toBe('#')
  })

  it('flags self-hostable providers', () => {
    expect(isSelfHostable('gitlab')).toBe(true)
    expect(isSelfHostable('gitea')).toBe(true)
    expect(isSelfHostable('github')).toBe(false)
    expect(isSelfHostable(undefined)).toBe(false)
  })

  it('validates base URLs', () => {
    for (const ok of ['', '  ', 'https://git.lan', 'http://10.0.0.5:3000/sub']) expect(isValidGitBaseUrl(ok)).toBe(true)
    for (const bad of ['git.lan', 'ftp://git.lan', 'https://' + 'u:p' + '@git.lan', 'https://git.lan?x=1', 'https://git.lan#a', 'javascript:alert(1)']) {
      expect(isValidGitBaseUrl(bad)).toBe(false)
    }
  })
})
