<template>
  <div
    ref="worldMapWrap"
    class="world-map-wrap"
  >
    <svg
      ref="worldMapSvg"
      class="world-map"
      role="img"
      aria-label="Carte mondiale du trafic par pays"
    />
    <div
      v-if="tooltip"
      class="world-map-tooltip"
      :style="{ left: `${tooltip.x}px`, top: `${tooltip.y}px` }"
    >
      <div class="fw-bold">
        {{ tooltip.country }}
      </div>
      <div class="text-secondary small">
        {{ numberFormat(tooltip.hits) }} hits
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { max as d3Max } from 'd3-array'
import { geoNaturalEarth1, geoPath } from 'd3-geo'
import { scaleSequential } from 'd3-scale'
import { interpolateYlOrRd } from 'd3-scale-chromatic'
import { select } from 'd3-selection'
import { feature } from 'topojson-client'

type AnyRecord = Record<string, any>

const props = defineProps<{ countryDistribution: AnyRecord[] }>()

const worldMapSvg = ref<SVGSVGElement | null>(null)
const worldMapWrap = ref<HTMLDivElement | null>(null)
const tooltip = ref<{ x: number; y: number; country: string; hits: number } | null>(null)
let resizeHandler: (() => void) | null = null

function numberFormat(v: number): string {
  return new Intl.NumberFormat('fr-FR').format(Number(v) || 0)
}

function normalizeCountryName(name: string): string {
  return (name || '')
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]/g, '')
}

function mapCountryKey(name: string): string {
  const key = normalizeCountryName(name)
  const aliases: Record<string, string> = {
    usa: 'unitedstatesofamerica',
    unitedstates: 'unitedstatesofamerica',
    uk: 'unitedkingdom',
    greatbritain: 'unitedkingdom',
    russia: 'russianfederation',
    southkorea: 'southkorea',
    northkorea: 'northkorea',
    czechia: 'czechrepublic',
    ivorycoast: 'cotedivoire',
    uae: 'unitedarabemirates',
  }
  return aliases[key] || key
}

function updateTooltip(event: MouseEvent, country: string, hits: number): void {
  const wrap = worldMapWrap.value
  if (!wrap) return
  const rect = wrap.getBoundingClientRect()
  // Clamp so the tooltip never runs off the right/bottom edge of the card.
  const x = Math.min(event.clientX - rect.left + 14, rect.width - 160)
  const y = Math.max(event.clientY - rect.top - 12, 0)
  tooltip.value = { x, y, country, hits }
}

async function renderWorldMap() {
  if (!worldMapSvg.value) return

  const worldMod = await import('world-atlas/countries-110m.json')
  // The component may have unmounted during the async import — re-check the ref.
  if (!worldMapSvg.value) return
  const worldAtlas = (worldMod as any).default || worldMod
  const world = feature(worldAtlas, worldAtlas.objects.countries) as AnyRecord
  const features = world?.features || []
  if (!Array.isArray(features) || !features.length) return

  const width = Math.max(320, worldMapSvg.value.clientWidth || 320)
  const height = width < 540 ? 240 : 340
  const svg = select(worldMapSvg.value)
  svg.attr('viewBox', `0 0 ${width} ${height}`)

  const countryHits = new Map<string, number>()
  for (const row of props.countryDistribution) {
    const key = mapCountryKey(String(row?.country || ''))
    if (!key) continue
    countryHits.set(key, Number(row?.hits) || 0)
  }

  const maxHits = Math.max(1, d3Max(props.countryDistribution, (d: AnyRecord) => Number(d?.hits) || 0) || 1)
  const color = scaleSequential(interpolateYlOrRd).domain([0, maxHits])

  const projection = geoNaturalEarth1().fitSize([width, height], world as any)
  const path = geoPath(projection as any)

  const root = svg.selectAll<SVGGElement, null>('g.world-root').data([null]).join('g').attr('class', 'world-root')

  const countries = root
    .selectAll<SVGPathElement, any>('path.country')
    .data(features)
    .join('path')
    .attr('class', 'country')
    .attr('d', path as any)
    .attr('fill', (d: AnyRecord) => {
      const key = mapCountryKey(String(d?.properties?.name || ''))
      const hits = countryHits.get(key) || 0
      return hits > 0 ? color(hits) : '#e9edf2'
    })
    .attr('stroke', '#ffffff')
    .attr('stroke-width', 0.6)

  countries
    .selectAll('title')
    .data((d: any) => [d])
    .join('title')
    .text((d: AnyRecord) => {
      const country = String(d?.properties?.name || 'Unknown')
      const key = mapCountryKey(country)
      const hits = countryHits.get(key) || 0
      return `${country}: ${numberFormat(hits)} hits`
    })

  countries
    .on('mouseenter', function (this: SVGPathElement, event: MouseEvent, d: AnyRecord) {
      select(this).raise().classed('country-hover', true)
      const country = String(d?.properties?.name || 'Unknown')
      const key = mapCountryKey(country)
      updateTooltip(event, country, countryHits.get(key) || 0)
    })
    .on('mousemove', function (event: MouseEvent, d: AnyRecord) {
      const country = String(d?.properties?.name || 'Unknown')
      const key = mapCountryKey(country)
      updateTooltip(event, country, countryHits.get(key) || 0)
    })
    .on('mouseleave', function (this: SVGPathElement) {
      select(this).classed('country-hover', false)
      tooltip.value = null
    })
}

onMounted(() => {
  void renderWorldMap()
  resizeHandler = () => {
    void renderWorldMap()
  }
  window.addEventListener('resize', resizeHandler)
})

onBeforeUnmount(() => {
  if (resizeHandler) window.removeEventListener('resize', resizeHandler)
})

watch(
  () => props.countryDistribution,
  () => {
    void nextTick().then(renderWorldMap)
  },
  { deep: true }
)
</script>

<style scoped>
.world-map-wrap {
  position: relative;
}

.world-map {
  width: 100%;
  height: 340px;
  display: block;
}

.world-map :deep(.country) {
  transition: filter 0.1s ease-in-out, stroke-width 0.1s ease-in-out;
}

.world-map :deep(.country-hover) {
  cursor: pointer;
  filter: brightness(1.25);
  stroke: var(--tblr-body-color);
  stroke-width: 1.4;
}

.world-map-tooltip {
  position: absolute;
  z-index: 5;
  pointer-events: none;
  background: var(--tblr-bg-surface);
  border: 1px solid var(--tblr-border-color);
  border-radius: 0.35rem;
  padding: 0.35rem 0.6rem;
  box-shadow: var(--tblr-box-shadow-sm, 0 1px 3px rgba(0, 0, 0, 0.2));
  white-space: nowrap;
}

@media (max-width: 992px) {
  .world-map {
    height: 260px;
  }
}

@media (max-width: 768px) {
  .world-map {
    height: 220px;
  }
}
</style>
