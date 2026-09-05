import frAlerts from './fr/alerts.json'
import frApt from './fr/apt.json'
import frAuth from './fr/auth.json'
import frCommon from './fr/common.json'
import frDashboard from './fr/dashboard.json'
import frDocker from './fr/docker.json'
import frErrors from './fr/errors.json'
import frHost from './fr/host.json'
import frMonitoring from './fr/monitoring.json'
import frNav from './fr/nav.json'
import frNetwork from './fr/network.json'
import frProxmox from './fr/proxmox.json'
import frSecurity from './fr/security.json'
import frSettings from './fr/settings.json'
import frWebhooks from './fr/webhooks.json'

import enAlerts from './en/alerts.json'
import enApt from './en/apt.json'
import enAuth from './en/auth.json'
import enCommon from './en/common.json'
import enDashboard from './en/dashboard.json'
import enDocker from './en/docker.json'
import enErrors from './en/errors.json'
import enHost from './en/host.json'
import enMonitoring from './en/monitoring.json'
import enNav from './en/nav.json'
import enNetwork from './en/network.json'
import enProxmox from './en/proxmox.json'
import enSecurity from './en/security.json'
import enSettings from './en/settings.json'
import enWebhooks from './en/webhooks.json'

export const fr = {
  common: frCommon,
  nav: frNav,
  auth: frAuth,
  dashboard: frDashboard,
  docker: frDocker,
  proxmox: frProxmox,
  network: frNetwork,
  alerts: frAlerts,
  settings: frSettings,
  apt: frApt,
  host: frHost,
  monitoring: frMonitoring,
  security: frSecurity,
  webhooks: frWebhooks,
  errors: frErrors,
}

export const en = {
  common: enCommon,
  nav: enNav,
  auth: enAuth,
  dashboard: enDashboard,
  docker: enDocker,
  proxmox: enProxmox,
  network: enNetwork,
  alerts: enAlerts,
  settings: enSettings,
  apt: enApt,
  host: enHost,
  monitoring: enMonitoring,
  security: enSecurity,
  webhooks: enWebhooks,
  errors: enErrors,
}
