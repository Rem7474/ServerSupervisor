// Docker domain types — model shapes re-exported from generated.ts.
import type { DockerContainer } from './generated'

export type { DockerContainer, ComposeProject, DockerNetwork } from './generated'

/** Paginated envelope returned by GET /api/v1/docker/containers (not a model). */
export interface DockerContainersPage {
  containers: DockerContainer[]
  total: number
  limit: number
  offset: number
}
