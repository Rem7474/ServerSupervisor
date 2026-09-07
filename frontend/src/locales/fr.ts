/**
 * All of the FR messages in one module.
 *
 * The per-locale barrel is what gives Rollup a single import boundary per
 * language: everything reachable only through `import('./fr')` lands in one
 * chunk instead of one chunk per namespace. Adding a language means adding a
 * sibling file and one line in index.ts's LOADERS.
 */
import account from './fr/account.json'
import alerts from './fr/alerts.json'
import apt from './fr/apt.json'
import auth from './fr/auth.json'
import common from './fr/common.json'
import dashboard from './fr/dashboard.json'
import docker from './fr/docker.json'
import errors from './fr/errors.json'
import host from './fr/host.json'
import monitoring from './fr/monitoring.json'
import nav from './fr/nav.json'
import network from './fr/network.json'
import npm from './fr/npm.json'
import proxmox from './fr/proxmox.json'
import runbooks from './fr/runbooks.json'
import scheduledTasks from './fr/scheduledTasks.json'
import security from './fr/security.json'
import settings from './fr/settings.json'
import webhooks from './fr/webhooks.json'

export default {
  account,
  alerts,
  apt,
  auth,
  common,
  dashboard,
  docker,
  errors,
  host,
  monitoring,
  nav,
  network,
  npm,
  proxmox,
  runbooks,
  scheduledTasks,
  security,
  settings,
  webhooks,
}
