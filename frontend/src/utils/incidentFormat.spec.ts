import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale } from '../i18n'
import {
  trackerStatusLabel,
  notificationTitle,
  formatIncidentValue,
  resolveHint,
} from './incidentFormat'

beforeEach(() => {
  setLocale('fr')
})

describe('trackerStatusLabel', () => {
  it('translates the known tracker statuses', () => {
    expect(trackerStatusLabel('pending')).toBe('Détection en cours')
    expect(trackerStatusLabel('success')).toBe('Exécution réussie')
    expect(trackerStatusLabel('error')).toBe('Exécution échouée')
  })

  it('falls back to a translated "unknown state" when the status is empty', () => {
    expect(trackerStatusLabel(undefined)).toBe('État inconnu')
  })

  it('falls back to the raw status string when it is set but unrecognized', () => {
    expect(trackerStatusLabel('weird_status')).toBe('weird_status')
  })
})

describe('notificationTitle', () => {
  it('prefers the rule name when present', () => {
    expect(notificationTitle({ rule_name: 'CPU high', type: 'alert_incident', metric: 'cpu' })).toBe('CPU high')
  })

  it('translates tracker-type titles', () => {
    expect(notificationTitle({ type: 'release_tracker_detected', rule_name: '', metric: '' })).toBe('Nouvelle version détectée')
    expect(notificationTitle({ type: 'release_tracker_execution', rule_name: '', metric: '' })).toBe('Exécution de tracker')
  })

  it('falls back to the metric label, then a generic "Notification"', () => {
    expect(notificationTitle({ type: 'alert_incident', rule_name: '', metric: 'memory' })).toBe('RAM')
    expect(notificationTitle({ type: 'alert_incident', rule_name: '', metric: '' })).toBe('Notification')
  })
})

describe('formatIncidentValue', () => {
  it('formats status_offline as translated online/offline', () => {
    expect(formatIncidentValue({ metric: 'status_offline', value: 1 })).toBe('offline')
    expect(formatIncidentValue({ metric: 'status_offline', value: 0 })).toBe('online')
  })

  it('formats disk_smart_status as translated OK/FAILED', () => {
    expect(formatIncidentValue({ metric: 'disk_smart_status', value: 1 })).toBe('FAILED')
    expect(formatIncidentValue({ metric: 'disk_smart_status', value: 0 })).toBe('OK')
  })

  it('formats docker_container_state across its three bands', () => {
    expect(formatIncidentValue({ metric: 'docker_container_state', value: 0 })).toBe('running')
    expect(formatIncidentValue({ metric: 'docker_container_state', value: 1 })).toBe('dégradé')
    expect(formatIncidentValue({ metric: 'docker_container_state', value: 2 })).toBe('critique')
  })

  it('prefers an explicit value_label for docker_container_state when present', () => {
    expect(formatIncidentValue({ metric: 'docker_container_state', value: 0, value_label: 'exited' })).toBe('exited')
  })

  it('pluralizes docker_compose_degraded_services by count', () => {
    expect(formatIncidentValue({ metric: 'docker_compose_degraded_services', value: 1 })).toBe('1 service dégradé')
    expect(formatIncidentValue({ metric: 'docker_compose_degraded_services', value: 3 })).toBe('3 services dégradés')
  })

  it('falls back to a generic value+unit format for other metrics', () => {
    expect(formatIncidentValue({ metric: 'cpu', value: 87.4 })).toBe('87.40%')
  })

  it('returns a dash when the value is null or undefined', () => {
    expect(formatIncidentValue({ metric: 'cpu', value: null })).toBe('-')
    expect(formatIncidentValue({ metric: 'cpu', value: undefined })).toBe('-')
  })
})

describe('resolveHint', () => {
  it('returns nothing when there is no clear threshold', () => {
    expect(resolveHint({ metric: 'cpu', clear_threshold: undefined, operator: '>' })).toBe('')
  })

  it('translates the ">"/">=" hint direction', () => {
    expect(resolveHint({ metric: 'cpu', clear_threshold: 70, operator: '>' })).toBe('repasse OK ≤ 70.00%')
  })

  it('translates the "<"/"<=" hint direction', () => {
    expect(resolveHint({ metric: 'ssl_min_days_remaining', clear_threshold: 30, operator: '<' })).toBe('repasse OK ≥ 30.00j')
  })

  it('falls back to a generic resolution-threshold phrase for an unrecognized operator', () => {
    expect(resolveHint({ metric: 'cpu', clear_threshold: 70, operator: '' })).toBe('seuil de résolution 70.00%')
  })
})
