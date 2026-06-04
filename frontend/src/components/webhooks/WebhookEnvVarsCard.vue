<template>
  <div class="mt-3 pt-3 border-top">
    <div class="text-muted small mb-2">
      Variables injectees dans votre script :
    </div>
    <div class="table-responsive">
      <table class="table table-sm mb-0">
        <tbody>
          <tr
            v-for="variable in envVars"
            :key="variable.name"
          >
            <td class="py-1">
              <code class="small">{{ variable.name }}</code>
            </td>
            <td class="py-1 text-muted small">
              {{ variable.desc }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  envVars: { name: string, desc: string }[]
}>()
</script>
