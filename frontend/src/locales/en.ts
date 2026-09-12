/**
 * All of the EN messages in one module.
 *
 * The per-locale barrel is what gives Rollup a single import boundary per
 * language: everything reachable only through `import('./en')` lands in one
 * chunk instead of one chunk per namespace. Adding a language means adding a
 * sibling file and one line in index.ts's LOADERS.
 */
import account from './en/account.json'
import alerts from './en/alerts.json'
import apt from './en/apt.json'
import auth from './en/auth.json'
import common from './en/common.json'
import dashboard from './en/dashboard.json'
import docker from './en/docker.json'
import errors from './en/errors.json'
import host from './en/host.json'
import monitoring from './en/monitoring.json'
import nav from './en/nav.json'
import network from './en/network.json'
import npm from './en/npm.json'
import proxmox from './en/proxmox.json'
import runbooks from './en/runbooks.json'
import scheduledTasks from './en/scheduledTasks.json'
import security from './en/security.json'
import settings from './en/settings.json'
import config from './en/config.json'
import webhooks from './en/webhooks.json'

export default {
  account,
  alerts,
  apt,
  auth,
  common,
  config,
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
