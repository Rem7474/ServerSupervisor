import { computed, isRef, type ComputedRef, type Ref } from 'vue'
import { normalizeDockerPorts, type DockerPortMapping } from '../utils/dockerPorts'

type RefLike<T> = T | Ref<T> | ComputedRef<T>

export interface DockerContainerLike {
  id: string | number
  ports?: unknown
}

function resolveValue<T>(source: RefLike<T>): T {
  return isRef(source) ? source.value : source
}

export function useDockerContainerPorts(containers: RefLike<DockerContainerLike[]>) {
  const normalizedPortsByContainer = computed(() => {
    const map = new Map<string | number, DockerPortMapping[]>()

    for (const container of resolveValue(containers) || []) {
      if (container?.id === undefined || container?.id === null) {
        continue
      }
      map.set(container.id, normalizeDockerPorts(container.ports))
    }

    return map
  })

  function normalizedPortsForContainer(container: DockerContainerLike): DockerPortMapping[] {
    if (container?.id === undefined || container?.id === null) {
      return []
    }

    return normalizedPortsByContainer.value.get(container.id) || []
  }

  return {
    normalizedPortsByContainer,
    normalizedPortsForContainer,
  }
}