# Audit UI/UX — ServerSupervisor (vague 2)

**Date** : août 2026
**Stack auditée** : Vue 3 + TypeScript (`<script setup>`) + Tabler 1.x (dark-only, `data-bs-theme="dark"` figé dans `index.html`)
**Périmètre** : 28 vues (`frontend/src/views/`) + ~107 composants (`frontend/src/components/**`) — couverture exhaustive de tout le frontend.
**Contexte** : suite de [UIUX-AUDIT-2026.md](UIUX-AUDIT-2026.md) (31 juillet 2026). Cette vague fait le point après la fusion des pages de notifications (Centre + Historique) et l'introduction d'un outillage anti-dérive automatique (voir plus bas) — et étend l'audit à tout le reste de l'application, qui n'avait pas été repassé au crible depuis.

## Ce qui est désormais mécaniquement enforced (ne PAS re-corriger, c'est déjà bloquant en CI)

Deux catégories de la vague précédente sont maintenant vérifiées automatiquement, pas seulement documentées :

- **`npm run lint:css`** (stylelint, `.stylelintrc.js`) — interdit toute couleur hex en dur dans un bloc `<style>`. Balayage complet effectué : 40 violations trouvées, 31 corrigées (retour aux tokens `--tblr-*`/`--ss-*` existants), le reste grandfathered avec justification écrite (`NetworkGraph.vue`, palette dataviz catégorielle, pas un état d'UI).
- **`npm run lint`** (ESLint, règle locale `eslint-rules/design-system.js`) — interdit `btn-xs`, `btn-outline-light`/`btn-ghost-light`, `btn-outline-orange`/`btn-ghost-orange`, `btn-info`/`btn-outline-info`/`btn-ghost-info`, et `window.confirm()`/`window.alert()` natifs, y compris dans les bindings `:class`. Balayage complet effectué : 5 violations trouvées et corrigées, zéro allowlist nécessaire.

Ces deux garde-fous couvrent une partie de ce que l'audit de juillet avait relevé (§1 Boutons, `btn-xs`/`btn-outline-light`/`btn-outline-orange`) — **ces lignes-là sont closes et ne réapparaissent pas ci-dessous.**

## Note de méthode

Audit mené par 4 agents en parallèle (3 sur des découpages de fichiers, 1 dédié aux couleurs de survol sur les vues) contre le référentiel déjà documenté dans `frontend/CLAUDE.md`. Chaque agent avait pour consigne explicite de ne pas re-signaler ce qui est déjà enforced par lint (ci-dessus), et de vérifier ses affirmations contre le CSS réellement compilé de Tabler plutôt que de deviner à partir des noms de classe. Sections organisées par axe (comme la vague 1), pas par page — les incohérences sont transverses par nature.

---

## 0. Ce qui est déjà cohérent

| Axe | Constat |
|---|---|
| **Tri de tableau** | **100 % conforme.** Tous les tableaux triables (`AlertIncidentList`, `AlertRuleList`, `DockerContainersTab`, `NetworkPortList`, `MonitoringOverviewPanel`, `ProxmoxNodeDisksTab`, `ProxmoxNodeGuestsTab`, `ProxmoxNodeTasksTab`, `HostProcessesPanel`, `HostTasksTab`, `ThreatsPanel`, etc.) passent par `SortableHeader.vue`. Zéro tri fait main trouvé. |
| **Pagination** | **100 % conforme.** Tous les tableaux paginés utilisent `PaginationNav.vue`. Les listes "aperçu N + voir plus" (`HostCommandsTab`, `TrackerVersionHistoryCard`) sont une UX différente et légitime, pas une pagination réinventée. |
| **Formulaires** | Quasi propre : seul `NetworkTopologyConfig.vue` substitue `text-secondary small` à `form-hint` (7 fois dans ce seul fichier) — partout ailleurs la convention est respectée. |
| **Lignes cliquables** | Pas de vraie confusion `.clickable-row`/`table-hover` trouvée dans cette vague — les rares tables `table-hover` sur lignes non cliquables (voir §7) sont mineures. |

---

## 1. Couleurs d'état brutes au lieu des tokens sémantiques (`danger`/`success`/`warning`/`primary`/`secondary`)

**De loin la catégorie la plus large de tout l'audit.** Le pattern dominant — running/online/success = vert, offline/error = rouge, pending = jaune, souvent avec `orange` en substitut de `warning` et `azure`/`blue`/`info` en substitut de `primary` — se répète dans au moins 45 fichiers. `DashboardKPIs.vue` à lui seul en concentre 8 occurrences sur l'écran d'accueil de l'app.

### Tableau de bord

| Fichier | Constat |
|---|---|
| `views/DashboardView.vue:691-701` (`bannerBadgeClass`/`bannerIconClass`) | `danger`→rouge, `warning`→jaune, `info`→**azure** brut. C'est l'exemple qui avait motivé cette vague d'audit — le correctif n'a en fait jamais atterri, les 3 branches sont brutes, pas seulement `azure`. |
| `views/DashboardView.vue:552` | Badge "DANGER" (barre bulk-apt-upgrade) : `bg-red-lt text-red`. |
| `views/DashboardView.vue:311` | Badge "Installation en attente" : `bg-yellow-lt text-yellow` au lieu de `warning`. |
| `components/dashboard/DashboardKPIs.vue:13,16,30,48,53,69,88,106,118` | 8 bindings distincts (en ligne/hors ligne, versions obsolètes, CVE CRIT/HIGH, nœuds Proxmox down, jauge stockage 3 paliers) — tous en `text-green`/`text-red`/`text-yellow` bruts. |
| `components/dashboard/DashboardDockerVersions.vue:17,95-104,109-121` | Compteur obsolètes, badges à jour/dispo/suivi — verts/jaunes/secondary bruts. |

### Docker

| Fichier | Constat |
|---|---|
| `components/docker/DockerContainersTab.vue:936-947` (`stateClass`) | `running`→vert, `restarting/paused`→jaune, `created`→bleu, `dead`→rouge, **`removing`→orange** (orange utilisé comme palier warning de fait). |
| `components/docker/DockerContainersTab.vue:199-209` | Badges image à jour/MAJ dispo — vert/jaune bruts. |
| `components/docker/ComposeProjectsTab.vue:81-86` | Badge running/stopped (vert vs secondary), badge "MAJ" (jaune brut). |
| `components/docker/ComposeProjectsTab.vue:233-238`, `DockerContainersTab.vue:81-86`, `DashboardDockerVersions.vue:158-170` | `alert-info` utilisé indifféremment pour un message de succès **et** d'échec (catch → message d'erreur, mais toujours rendu en `alert-info`) — devrait brancher vers `alert-success`/`alert-danger`. |

### Proxmox / hôtes

| Fichier | Constat |
|---|---|
| `components/proxmox/ProxmoxClusterCard.vue:37,109,201-217` | Pastille online/offline + `pctColor`/`cpuBarColor`/`ramBarColor` (seuils CPU/RAM) — rouge/jaune/vert bruts. |
| `components/proxmox/ProxmoxNodeDisksTab.vue:79,83` | Badges santé SMART bruts — incohérent avec `wearoutColor` (157-161) du **même fichier**, qui utilise correctement `danger`/`warning`/`success`. |
| `components/proxmox/GuestLinkCell.vue:10,31` | Badges "Suggéré"/"Lié" — jaune/vert bruts. |
| `components/proxmox/ProxmoxNodeSecurityTab.vue:234-242` (`syslogLevelBadgeClass`) | critical/error→rouge, **warning→orange**, success→vert, **info→azure** — le cas d'école exact de l'exemple de calibrage. |
| `components/proxmox/ProxmoxNodeGuestsTab.vue:404-411`, `ProxmoxNodeServicesTab.vue:218-223`, `ProxmoxNodeStorageTab.vue:57-63`, `ProxmoxNodeTasksTab.vue:190-195` | États guest/service/stockage/tâche — vert/jaune/rouge/bleu bruts à chaque fois. |
| `components/host/HostSystemdPanel.vue:218-223` | Logique quasi identique à `ProxmoxNodeServicesTab.vue` ci-dessus (dupliquée), même défaut. |
| `components/host/HostMetricsPanel.vue:320-339` (`cpuColor`/`memColor`/`tempColor`) | Seuils CPU/RAM/température — rouge/jaune/vert bruts. |
| `components/host/HostProcessesPanel.vue:146` | Badge d'état process brut — alors que le CPU/MEM% du même fichier (131,136) utilise correctement `text-danger`/`text-warning`. |
| `components/host/HostAptTab.vue:63,76,169,173,344` | KPIs pending/security bruts ; `bg-orange-lt` (173, "Redémarrage requis") = orange-comme-warning. |
| `components/host/HostDockerTab.vue:40,44,60` | Badges MAJ/running — vert/jaune bruts. |
| `components/host/HostBackupTab.vue:257-269` (`runBadgeClass`) | vert/bleu/orange/rouge/secondary bruts. |
| `components/host/HostTimelineTab.vue:170-193` | rouge/jaune/bleu/orange/vert/secondary bruts sur icônes et badges. |
| `components/host/HostCommandsTab.vue:126-128`, `CommandLogPanel.vue:211-213` | Fallback de badge `bg-yellow-lt text-yellow` au lieu d'un fallback `warning` sémantique. |
| `components/apt/AptHostCard.vue:32,146,150,162,166,178-186,237,396` | Pastille online/offline (`status-lime`/`status-red`), KPIs pending/sécurité, et surtout la sévérité CVE avec **orange pour "medium"** — encore l'antipattern orange-comme-warning. |
| `components/network/NetworkNodeDetail.vue:456-459`, `NetworkPortList.vue:194-197` | Pastilles online/warning/offline en `status-lime`/`status-yellow`/`status-red` identiques dans les deux fichiers. |
| `components/network/NetworkNodeDetail.vue:445` | **`bg-amber-lt text-amber` — classe inexistante dans Tabler** (vérifié contre `tabler.css` : pas de variante `amber`). Rend sans aucun style, même symptôme que l'ancien bug `btn-xs`. |

### Sécurité / monitoring / CVE / alertes

| Fichier | Constat |
|---|---|
| `components/security/AuditSecurityPanel.vue:82,120`, `ExposureDomainsPanel.vue:131` | Badges "Bloquée"/échecs/désactivé — rouge brut. |
| `components/security/TrafficKpiCards.vue:45,117-129`, `TrafficOverviewPanel.vue:242,400-404` | KPI "Taux 5xx", deltas, badges IP suspecte/4xx/5xx — rouge/jaune/vert bruts. |
| `components/security/ThreatsPanel.vue:118,130,142,154,306,513` | 3 KPIs en `text-orange` (suspect) juste à côté d'un KPI correctement en `text-success` (154) sur le **même écran** — deux conventions cohabitent visuellement. |
| `components/security/DomainDetailsModal.vue:78,94,169,416,421,540-545` (`statusClass`) | KPI 4xx/5xx et badges statut/bloqué/suspect — tous bruts. |
| `components/security/IPTimelineModal.vue:61,142,208-209,532-554` | Badge "IP bloquée" (vert brut), KPI "Erreurs" (rouge brut), et classes `.is-calm/.is-warm/.is-hot`/`.active` bindées directement sur `--tblr-blue`/`--tblr-yellow`/`--tblr-red` plutôt que `--tblr-primary`/`--tblr-warning`/`--tblr-danger`. |
| `components/monitoring/MonitoringOverviewPanel.vue:189`, `UptimeDetailSection.vue:120,197` | Ticks/barres/badges OK/KO — `bg-green`/`bg-red` bruts. |
| `components/monitoring/SslDetailSection.vue` | (voir §3, pas de couleur ici mais chargement ad-hoc) |
| `components/alerts/AlertRuleStepConditions.vue:7,25,287,291,343,347` | Badges tag warn/crit et résultats de test — jaune/rouge bruts. |
| `components/alerts/AlertReleaseSummary.vue:211-216`, `AlertRuleList.vue:100` | États d'exécution (succeeded/failed/running/pending) et compteur d'incidents actifs — bruts. |
| `components/apt/CVEBadge.vue:49-58` **et** `apt/CVEList.vue:236-244` | Map CRITICAL/HIGH/MEDIUM/LOW/UNKNOWN → rouge/**orange**/jaune/bleu/secondary — **dupliquée à l'identique dans 2 fichiers**, en plus d'être toute en couleurs brutes. |
| `components/disk/DiskHistoryChart.vue:31` | Badge prédiction de remplissage — `bg-red text-white`/`bg-yellow text-white` **plein** (pas même `-lt`) au lieu de `bg-danger`/`bg-warning`. |
| `components/disk/DiskMetricsCard.vue:131` (`getProgressBarClass`) | Jauge à 4 paliers : `>=90 danger`, `>=80 warning`, **`>=70 info`**, sinon `success` — `info`/`azure` inventé comme 4ᵉ palier de sévérité entre warning et success. |
| `components/settings/SettingsDatabaseCard.vue:13`, `SettingsSystemInfoCard.vue:29`, `SettingsNPMCard.vue:150-166`, `SettingsProxmoxCard.vue:161-177` | Badges de connexion/TLS — vert/rouge/jaune bruts, logique dupliquée entre les deux cartes NPM/Proxmox. |
| `components/webhooks/WebhookExecutionList.vue:92`, `TrackerScriptHelpCard.vue:40` | Badge nb d'alertes, indicateur succès — rouge/vert bruts. |
| `views/NPMView.vue:321-331` (`uptimeBadge`/`sslBadge`) | up/down et échéance SSL (≤7j/≤30j/plus) — rouge/jaune/vert bruts. |
| `views/GitWebhooksView.vue:417,433,438` | "Cooldown actif"/"aucune release"/"erreur" — jaune/rouge bruts. |
| `views/ProxmoxGuestView.vue:357-364`, `ProxmoxNodeView.vue:655,662` | running/paused, badges d'onglet tâches échouées/MAJ en attente — bruts (655 est même `bg-yellow` pour un échec, alors que `danger` serait plus juste que `warning`). |
| `views/GlobalScheduledTasksView.vue:285` | Badge Oui/Non "Activée" : vert/secondary brut. |
| `views/RunbooksView.vue:554-559` (`executionBadgeClass`) | completed/failed/running — vert/rouge/bleu bruts. |
| `views/HostDetailView.vue:425,574,876,889,891` | Sévérité incident (rouge/jaune brut), badge "Admin only" en **`bg-red` plein** (pas `-lt`), compteurs d'onglet (incidents/sécurité/pending APT). |
| `views/ReleaseTrackerDetailView.vue:63` *(confiance moindre)* | `bg-info-lt border-info` — `info` n'est pas un des 5 tokens documentés, c'est une 6ᵉ variante Bootstrap non canonique. |
| `style.css:118` | `--ss-status-warning: #fb923c;` commenté `/* aligned to --tblr-orange */` — **le token lui-même** aligne délibérément "warning" sur orange plutôt que sur `--tblr-warning`/`#f59f00`, contredisant la charte documentée. Consommé par `NetworkGraph.vue:213`. À corriger à la source réglerait plusieurs symptômes d'un coup. |

---

## 2. État vide ad-hoc au lieu de `<EmptyState>`

| Fichier | Constat |
|---|---|
| `views/AptView.vue:71-80`, `HostDetailView.vue:407-412,580-585`, `NPMView.vue:73-82`, `GlobalScheduledTasksView.vue:646-651`, `RunbooksView.vue:312-317` | `<div class="text-center text-muted/secondary py-4/5">Aucun…</div>` classiques. |
| `components/docker/ComposeProjectsTab.vue:241-256` | Idem — fichier n'importe même pas `EmptyState`. |
| `components/network/NetworkPortList.vue:137-142,203-208,355-360,439-444` | **4 blocs ad-hoc distincts** dans le même fichier. |
| `components/NotificationBell.vue:57-68` | "Aucune notification" avec `IconBell` nu au lieu de `<EmptyState>`. |
| `components/monitoring/SslDetailSection.vue:183-188` | "Aucun renouvellement enregistré…". |
| `components/security/AuditSecurityPanel.vue:69-74,106-111`, `ExposureDomainsPanel.vue:73-78`, `TrafficOverviewPanel.vue:220-225,318-323` *(incohérent avec ses propres tableaux, à quelques lignes, qui utilisent bien `<EmptyState>`)*, `ThreatsPanel.vue:286-291`, `DomainDetailsModal.vue:110-115,146-151` | Même pattern répété dans quasi tout le domaine sécurité. |
| `components/webhooks/TrackableContainersModal.vue:41-51` | Markup Tabler brut `.empty`/`.empty-title` au lieu du composant partagé. |
| `components/webhooks/TrackerVersionHistoryCard.vue:19-24`, `WebhookExecutionList.vue:24-29` | "Aucune version"/"Aucune exécution". |
| `views/NetworkView.vue:271-286` *(confiance moindre)* | Overlay "aucun nœud" sur le canvas graphe — markup bespoke, cas limite car positionné en absolu sur un canvas, pas une carte/tableau classique. |
| `components/network/NetworkGraph.vue:130-140` *(confiance moindre)* | Idem, overlay `.graph-empty`. |

---

## 3. État de chargement ad-hoc au lieu de `<LoadingSkeleton>`

| Fichier | Constat |
|---|---|
| `views/AccountView.vue:337-343` | `<tr><td colspan="7">Chargement...</td></tr>` — alors que `AuditLogsView.vue` fait ça correctement (`LoadingSkeleton variant="table"`) pour le **même tableau** de commandes. |
| `views/AccountSecurityView.vue:259-263` | "Chargement…" texte brut pour la liste des passkeys — devrait être `variant="list"`. |
| `views/NetworkView.vue:264-270` | `spinner-border` pour le premier chargement de **toute la zone graphe** — `spinner-border` est fait pour une action ponctuelle, pas le premier rendu d'une zone entière ; `variant="chart"` est déjà utilisé ailleurs (Dashboard, ProxmoxGuestView). |
| `components/NotificationBell.vue:49-55` | "Chargement…" texte brut pour le premier chargement de la liste de notifications. |
| `components/monitoring/SslDetailSection.vue:176-181` | "Chargement…" texte brut pour l'historique de renouvellement. |
| `components/proxmox/ProxmoxNodeServicesTab.vue`, `components/host/HostSystemdPanel.vue` | Premier chargement du tableau de services représenté uniquement par un spinner sur le bouton "Actualiser" — aucun skeleton sur la zone tableau elle-même (contraste avec `HostProcessesPanel.vue:41-44`, qui fait ça bien pour un tableau similaire). |
| `components/settings/SettingsNPMCard.vue`, `SettingsProxmoxCard.vue`, `SettingsRegistryCredentialsCard.vue` | Aucun indicateur de chargement au premier `load()` — `EmptyState` ("Aucune connexion") peut clignoter avant la vraie réponse. |

---

## 4. Rôle/couleur de bouton incorrect

| Fichier | Constat |
|---|---|
| `views/AuditLogsView.vue:242-254` | "Annuler" une commande (table row) : `btn-sm btn-outline-danger` + texte, au lieu de `btn-icon btn-sm btn-ghost-danger`. |
| `views/GlobalScheduledTasksView.vue:310-322` | "Exécuter" (table row) : `btn-sm btn-ghost-primary` + texte, alors que les boutons voisins de la même ligne sont bien en `ghost-secondary`/`ghost-danger` icon-only — devrait être `btn-icon btn-sm btn-ghost-success`. |
| `views/GitWebhooksView.vue:218,478` | Boutons supprimer : `btn-icon btn-sm btn-outline-danger` au lieu de `btn-ghost-danger`. |
| `views/RunbooksView.vue:107` | "Lancer" par ligne : `btn-icon btn-sm btn-primary` **plein** — à la fois mauvaise couleur (devrait être `ghost-success`) et N+1 `btn-primary` sur l'écran (le header a déjà "Nouveau runbook" en primary plein). |
| `views/UsersView.vue:182-190` | "Supprimer" (table row) : `btn-sm btn-danger` **plein** avec texte — le plein rouge est réservé au pied de modale, pas à une ligne de tableau. |
| `components/host/HostTasksTab.vue:173-180` | "Supprimer" en ligne : `btn-sm btn-outline-danger` + texte au lieu d'icon-only ghost. |
| `components/settings/SettingsRegistryCredentialsCard.vue:126-141` | "Modifier"/"Supprimer" en texte, sans icône — incohérent avec les 2 cartes sœurs (NPM/Proxmox) qui sont icon-only. |
| `components/security/ThreatsPanel.vue:588-603` | "Débloquer" CrowdSec : bouton texte `outline-success`/`btn-danger` plein, alors que la **même action** dans `DomainDetailsModal.vue:191-209,380-398` est bien en `btn-icon btn-sm btn-ghost-danger/secondary`. |
| `components/webhooks/TrackableContainersModal.vue:145-152`, `WebhookModal.vue:477-484` | Bouton fermer/annuler en pied de modale : `btn-secondary` **plein** au lieu de `btn-outline-secondary` (toutes les autres modales du scope utilisent outline). |
| `components/webhooks/TrackerConfigCard.vue:20-32` | "Exécuter" en en-tête de carte : `btn-primary` plein — devrait être `btn-sm btn-outline-success` (action de déclenchement, pas l'action primaire de la page). |
| `apt/AptToolbar.vue:15,81` + `apt/AptHostCard.vue:78` | Filtre rapide actif + "apt upgrade" bulk + bouton "upgrade" par carte-hôte peuvent tous être `btn-primary` **simultanément** → N+2 boutons primary sur un même écran une fois des hôtes sélectionnés. |
| `components/docker/ComposeProjectsTab.vue:110`, `dashboard/DashboardDockerVersions.vue:136` | Bouton "Run"/"Déclencher" par ligne de tableau, en `btn-primary` — répété une fois par ligne obsolète. |
| `components/docker/DockerContainersTab.vue:345` | "Déclencher le tracker" : `btn-icon btn-sm btn-ghost-primary` — `primary` n'est pas dans la liste sanctionnée pour une action de ligne (`success/danger/warning` pour cycle de vie, `secondary` pour logs/statut/reload/déclenchement). |
| `components/monitoring/MonitoringOverviewPanel.vue:247-283` | Éditer/supprimer : `btn-icon btn-outline-secondary`/`btn-icon btn-outline-danger` (mauvaise famille de variante, `btn-sm` implicite via le `.btn-group-sm` parent plutôt qu'explicite). |
| `components/network/NetworkGraph.vue:63,79,95,112` | Contrôles zoom/fit/reset : `btn-sm btn-outline-secondary` au lieu de `btn-sm btn-ghost-secondary` (niveau "toolbar"). |
| `components/alerts/AlertRuleList.vue:194-204`, `monitoring/MonitoringOverviewPanel.vue:255-283` | `title` présent mais **`aria-label` absent** sur des boutons destructifs de ligne — la convention exige les deux. |

---

## 5. Ligne cliquable / `table-hover` mal posé

| Fichier | Constat |
|---|---|
| `components/proxmox/ProxmoxNodeServicesTab.vue:62`, `host/HostProcessesPanel.vue:67`, `host/HostSystemdPanel.vue:61`, `host/HostTasksTab.vue:47` | `table-hover` sur des tableaux dont les lignes ne sont pas individuellement cliquables (seuls des boutons de cellule le sont). |

Bonnes références (rien à corriger) : `TrafficOverviewPanel.vue`, `ThreatsPanel.vue`, `DomainDetailsModal.vue`, `DashboardView.vue`, `RunbooksView.vue`, `GlobalScheduledTasksView.vue` — tous corrects sur `.clickable-row` + `role="button"`/`tabindex`/clavier, ou réservent `table-hover` à juste titre.

---

## 6. Taille d'icône hors paliers (14/16/24/48 uniquement)

| Fichier | Constat |
|---|---|
| `views/AccountSecurityView.vue:238,361`, `AccountView.vue:136,175,313` | `:size="18"`/`"20"` en en-tête de carte au lieu de 24. |
| `views/DashboardView.vue:24,48,87` | 20 dans un `btn-sm` (→14), 18 en en-tête de carte (→24), 20 inline dans une alerte (→16). |
| `views/HostDetailView.vue:389`, `SettingsView.vue:33,53` | 18 en en-tête de carte / nav sidebar (→24/16). |
| `views/RunbooksView.vue:30,118,129,140,151` | 20 dans un `btn-primary` non-sm (→16) ; **4 occurrences** de 18 dans des `btn-icon btn-sm` de la même rangée d'actions (→14). |
| `views/NetworkView.vue:87,276` | 12 sur une icône info-bulle (→14) ; 40 sur l'icône d'état vide du graphe (→48). |
| `views/GlobalScheduledTasksView.vue:141` | `<EmptyState :icon-size="40">` (→48, ou omettre — 48 est déjà la valeur par défaut). |
| `components/CommandPalette.vue:17,58`, `NotificationBell.vue:16,63,136` | 18/20/32/12 à divers endroits. |
| `components/alerts/AlertRuleList.vue:190,201` | 20 dans des `btn-icon btn-sm` (→14). |
| `components/disk/DiskHealthCard.vue:23`, `DiskMetricsCard.vue:23` | `<EmptyState :icon-size="36">` (→48). |
| *(note mineure, pas un vrai défaut)* `MonitoringOverviewPanel.vue` utilise 14 dans ses `btn-icon btn-sm`, quand `DockerContainersTab`/`ComposeProjectsTab`/`AptHostCard`/`NetworkTopologyConfig` utilisent 16 au même endroit — à trancher une bonne fois (14 est la valeur documentée pour "dans un btn-sm/badge"). | |

---

## 7. Confirmation manquante sur une action destructrice

**La catégorie la plus importante de tout l'audit — ce sont de vrais trous UX/sécurité, pas du cosmétique.**

| Fichier | Action | Constat |
|---|---|---|
| `views/HostDetailView.vue:183-194` → `useHostDetail.ts` `deleteLink()` | Supprimer un lien Proxmox | Aucun `useConfirmDialog()` — alors que `deleteHost()`/`confirmLink()` du même composable en ont bien un. |
| `views/HostDetailView.vue:609-619` → `useHostDetail.ts` `revokePermission()` | Révoquer une permission d'hôte | Idem, aucune confirmation, action pourtant irréversible en un clic. |
| `components/security/DomainDetailsModal.vue:191-209,380-398` → `useDomainDetails.ts` | Bannir une IP (CrowdSec) | Aucun `useConfirmDialog()` dans tout le composable — alors que la **même action** dans `IPTimelineModal.vue:337-345` (`handleBanClick`) est bien protégée. |
| `components/security/ThreatsPanel.vue:588-603` → `useBot.ts` | Débloquer une IP CrowdSec | Idem, aucune confirmation dans le composable. |
| `components/network/NetworkTopologyConfig.vue:199-205,769-771` (`removeServiceRow`) | Supprimer une ligne de service réseau configurée manuellement | Suppression immédiate au clic, aucune étape de confirmation. |

---

## 8. Couleurs de hover

Le bug corrigé cette session (barre de recherche du navbar, `App.vue` — texte gris sur fond gris au survol) était **isolé** : aucune autre occurrence exacte du même pattern n'a été trouvée dans les 28 vues ni dans les 107 composants passés au crible. Un vrai trou distinct a émergé :

| Fichier | Constat |
|---|---|
| `views/ProxmoxView.vue:133-202` (4 cartes KPI "Nœuds hors ligne"/"Stockages >80%"/"Stockages inactifs"/"Tâches échouées") | Cartes réellement cliquables (`role="button"`, `tabindex="0"`, `@click`, `@keydown.enter`) mais **aucun `:hover` défini** — seul `.health-card-active` (anneau `box-shadow`) existe, pour la carte déjà sélectionnée. Survoler une carte inactive (le cas courant, 3 sur 4) ne produit **aucun** retour visuel au-delà du curseur. |
| `views/DashboardView.vue:309-320` *(confiance moindre)* | Deux badges "Installation en attente"/"Stats Proxmox" sont en fait des `<router-link>` avec `text-decoration-none`, posés à côté de badges inertes en `<span>` — aucun état hover propre à Tabler pour `a.badge`, et `text-decoration-none` retire même le seul indice natif (soulignement au survol). Au repos comme au survol, indiscernables des tags non cliquables voisins. |

---

## Priorisation suggérée

1. **§7 (confirmations manquantes)** — 5 corrections ciblées, chacune un ajout de `useConfirmDialog()` dans un composable existant. Risque quasi nul, gain réel.
2. **§8 (hover ProxmoxView)** — 1 règle CSS à ajouter, même ampleur que le correctif déjà fait sur `App.vue`.
3. **§1 (couleurs sémantiques)** — le plus gros volume, mais mécanique une fois qu'un petit nombre de mappings (running/success, offline/danger, pending/warning, catégoriel/secondary) sont extraits en fonctions/utilitaires partagés par domaine (à l'image de `notificationBadges.ts` fait pour les notifications) plutôt que corrigés fichier par fichier. Prioriser `DashboardKPIs.vue` (8 occurrences, écran d'accueil) et la paire `CVEBadge.vue`/`CVEList.vue` (logique dupliquée à fusionner en un seul utilitaire) en premier.
4. **§2/§3 (empty/loading ad-hoc)** — remplacement mécanique, faible risque, gain de cohérence large pour un faible coût (comme noté dans la vague 1).
5. **§4/§5/§6** — cosmétique, à traiter au fil de l'eau plutôt qu'en chantier dédié.
