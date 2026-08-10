// Docker Compose metadata derived from a container's own labels — shared by
// DockerContainersTab.vue (the /docker global page) and HostDockerTab.vue
// (the host detail Docker tab), which used to compute this independently and
// only the global page actually rendered it, leaving the host tab with no
// way to tell a compose-managed container from a standalone one.

export interface ComposeInfo {
  project: string
  service: string
  workingDir: string
  configFiles: string
}

export function getComposeInfo(labels: Record<string, string> | undefined): ComposeInfo {
  return {
    project: labels?.['com.docker.compose.project'] || '',
    service: labels?.['com.docker.compose.service'] || '',
    workingDir: labels?.['com.docker.compose.project.working_dir'] || '',
    configFiles: labels?.['com.docker.compose.project.config_files'] || '',
  }
}

function normalizeComposeName(value: string | undefined): string {
  return String(value || '').toLowerCase().replace(/[^a-z0-9]/g, '')
}

// A compose service name that's just a normalized copy of the project name
// (the common single-service-project case) is redundant to show twice.
export function isComposeServiceRedundant(labels: Record<string, string> | undefined): boolean {
  const info = getComposeInfo(labels)
  if (!info.project || !info.service) return true
  return normalizeComposeName(info.project) === normalizeComposeName(info.service)
}

export function isComposeContainer(labels: Record<string, string> | undefined): boolean {
  return !!labels?.['com.docker.compose.project']
}
