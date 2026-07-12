// Shared with AddHostView's post-registration screen and HostEditForm's key
// rotation panel — both need to hand the admin a working install command /
// agent config for the exact same api_key, so the server URL resolution and
// script location live here once instead of drifting between two copies.
const INSTALL_SCRIPT_URL =
  'https://raw.githubusercontent.com/Rem7474/ServerSupervisor/main/agent/install.sh'

function resolveServerUrl(): string {
  return typeof window !== 'undefined' && window.location?.origin
    ? window.location.origin
    : 'http://localhost:8080'
}

export function buildInstallCommand(apiKey: string): string {
  return `curl -sSL ${INSTALL_SCRIPT_URL} | sudo bash -s -- --server-url ${resolveServerUrl()} --api-key "${apiKey}"`
}

export function buildAgentConfig(apiKey: string): string {
  return `server_url: "${resolveServerUrl()}"\napi_key: "${apiKey}"\nreport_interval: 30\ncollect_docker: true\ncollect_apt: true`
}
