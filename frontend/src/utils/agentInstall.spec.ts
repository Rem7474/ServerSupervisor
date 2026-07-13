import { describe, it, expect } from 'vitest'
import { buildInstallCommand, buildAgentConfig } from './agentInstall'

describe('agentInstall', () => {
  it('buildInstallCommand embeds the api key and install script URL', () => {
    const cmd = buildInstallCommand('my-api-key')
    expect(cmd).toContain('curl -sSL https://raw.githubusercontent.com/Rem7474/ServerSupervisor/main/agent/install.sh')
    expect(cmd).toContain('--api-key "my-api-key"')
    expect(cmd).toContain('--server-url ')
  })

  it('buildAgentConfig embeds the api key and expected defaults', () => {
    const cfg = buildAgentConfig('my-api-key')
    expect(cfg).toContain('api_key: "my-api-key"')
    expect(cfg).toContain('report_interval: 30')
    expect(cfg).toContain('collect_docker: true')
    expect(cfg).toContain('collect_apt: true')
    expect(cfg).toMatch(/^server_url: "/)
  })

  it('resolves the same server URL in both outputs', () => {
    const cmd = buildInstallCommand('k')
    const cfg = buildAgentConfig('k')
    const urlFromCmd = cmd.match(/--server-url (\S+) --api-key/)?.[1]
    const urlFromCfg = cfg.match(/server_url: "([^"]+)"/)?.[1]
    expect(urlFromCmd).toBeTruthy()
    expect(urlFromCmd).toBe(urlFromCfg)
  })
})
