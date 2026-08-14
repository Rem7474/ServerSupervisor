// Docker domain types — model shapes re-exported from generated.ts.
import type { DockerContainer } from './generated'

export type { DockerContainer, ComposeProject, DockerNetwork, VersionComparison, DockerImageVersion } from './generated'

/**
 * Verdict of a VersionComparison row, computed server-side (see
 * models.VersionStatus* / ws.comparisonStatus) so the host Docker tab and the
 * global Docker page can't classify the same container differently.
 */
export type VersionComparisonStatus = 'up_to_date' | 'update_available' | 'unknown'

/** Paginated envelope returned by GET /api/v1/docker/containers (not a model). */
export interface DockerContainersPage {
  containers: DockerContainer[]
  total: number
  limit: number
  offset: number
}
