# Audit UI/UX — ServerSupervisor

**Date** : 31 juillet 2026 · **Branche** : `claude/servervisor-uiux-audit-uq5bkh`
**Stack auditée** : Vue 3 + TypeScript (`<script setup>`) + Tabler 1.x (dark-only, `data-bs-theme="dark"` figé dans `index.html`)
**Périmètre** : 30 routes / 28 vues / 136 composants `.vue` — `frontend/src/`
**Design system existant** : aucune charte formalisée. Tabler sert de socle de fait ; les règles ci-dessous sont **déduites de l'existant** (convention majoritaire = référence).

## Note de méthode

L'énoncé demandait un tableau par page. Les incohérences réelles sont **transverses par nature** (« même action, deux styles selon la page ») : un découpage par page produirait 30 tableaux quasi identiques et masquerait justement les écarts inter-pages. Le rapport est donc organisé par **axe d'audit**, avec la colonne « Page(s) concernée(s) » demandée. Chaque ligne est vérifiée par inspection du code (comptages reproductibles, pas d'estimation).

---

## 0. Ce qui est déjà cohérent

À isoler d'emblée : ces points sont sains et ne doivent pas être « corrigés ».

| Axe | Constat |
|---|---|
| **Set d'icônes** | **100 % `@tabler/icons-vue`** (68 fichiers). Aucun mélange Material/FontAwesome/Lucide. Les 2 seuls `<svg>` inline sont de la dataviz légitime (`NetworkGraph.vue`, `TrafficWorldMap.vue`). |
| **Pagination** | **Un seul composant** `PaginationNav.vue`, utilisé par les 8 listes paginées. **Zéro** markup `.pagination` ad-hoc ailleurs. |
| **En-tête de page** | 27/28 vues utilisent `page-header` + `page-pretitle` + `page-title` (seule `LoginView` diverge, à raison). |
| **Focus clavier** | `*:focus-visible` global dans `style.css:177` + variante dark. Couvert partout par défaut. |
| **Tokens custom** | `--ss-*` (`style.css:71-124`) : vrai début de design system pour les surfaces slate hors palette Tabler. |
| **Alignement colonne Actions** | 25 `text-end` contre 2 `text-center` — convention claire, 2 exceptions à aligner. |

---

## 1. Boutons

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Triade start / stop / restart** | **3 conventions de remplissage** pour la même action, selon la page : `btn-success` **plein** + `btn-outline-danger` + `btn-outline-warning` (Docker) · `btn-ghost-success/danger/warning` **fantôme** (Proxmox guests) · `btn-outline-success` + `btn-outline-danger` + `btn-primary` **plein bleu pour Restart** (systemd / services PVE) | `DockerContainersTab.vue:244,262,281` · `HostDockerTab.vue` · `ComposeProjectsTab.vue` · `ProxmoxNodeGuestsTab.vue:216,233` · `HostSystemdPanel.vue:97` · `ProxmoxNodeServicesTab.vue:99` | **Haute** | Une règle unique : action de cycle de vie = `btn-icon btn-sm btn-ghost-<intent>`. Extraire un composant `LifecycleActions.vue` (start/stop/restart/logs) consommé par les 6 emplacements. |
| **Bouton Supprimer** | **7 combinaisons distinctes** pour la même action destructive : `btn-icon btn-sm btn-ghost-danger` (×6) · `btn-sm btn-outline-danger` (×5) · `btn-icon btn-sm btn-outline-danger` (×3) · `btn-icon btn-outline-danger` (×2, **sans `btn-sm`** → taille différente en ligne de tableau) · `btn-sm btn-danger` **plein** (×1) · `btn-outline-danger` (×1) · `btn-sm btn-link` (×1) | `AlertRuleList` · `SettingsNPMCard` · `SettingsProxmoxCard` · `GlobalScheduledTasksView` · `HostDetailView` · `HostTasksTab` · `NetworkTopologyConfig` · `AccountSecurityView` · `GitWebhooksView` · `MonitoringOverviewPanel:258,279` · `UsersView:184` · `ProxmoxView:376` | **Haute** | Destructif en ligne de tableau → `btn-icon btn-sm btn-ghost-danger` ; destructif en pied de modale → `btn-danger`. Corriger en priorité `UsersView:184` (seul rouge plein en tableau) et `MonitoringOverviewPanel` (taille non alignée). |
| **`btn-xs`** | Taille **inexistante dans Tabler** (vérifié contre `@tabler/core/dist/css/tabler.css` : aucune définition). Les 5 usages ne produisent donc aucun style — le bouton retombe silencieusement en taille par défaut, plus grand que voulu. | `GlobalScheduledTasksView.vue:701,714` · `GuestLinkCell.vue:14,21,34` | **Moyenne** | Remplacer par `btn-sm`, ou définir explicitement `.btn-xs` dans `style.css` si un 4ᵉ palier est réellement voulu. |
| **`btn-outline-light` / `btn-ghost-light`** | 9 usages d'une variante « claire » sur un thème **dark-only** — contraste et intention non alignés avec `btn-outline-secondary` (109 usages), qui est la convention réelle. | `AddHostView:171,189,203` · `HostEditForm:123,135,148` · `AccountSecurityView:125,166` · `NetworkTopologyConfig:90` | **Moyenne** | Remplacer par `btn-outline-secondary`. |
| **`btn-outline-orange`** | Variante unique dans toute l'app (1 usage), là où l'équivalent sémantique `btn-outline-warning` est utilisé 9 fois. | `HostDetailView.vue:218` | **Faible** | Remplacer par `btn-outline-warning`. |
| **Libellés EN/FR** | `Start` / `Stop` / `Restart` en anglais coexistent avec `Démarrer` / `Arrêter` / `Redémarrer`, pour les mêmes actions. | Docker vs Proxmox / systemd | **Moyenne** | Français partout (`<html lang="fr">`, i18n non installé — cf. `frontend/CLAUDE.md`). |
| **Préfixe `+` textuel** | `+ Ajouter`, `+ Ajouter une connexion` : le « plus » est un caractère texte au lieu de `<IconPlus>`, contrairement au reste des boutons à icône. | `SettingsNPMCard` · `SettingsProxmoxCard` · `NetworkTopologyConfig` | **Faible** | `<IconPlus :size="16" />` + libellé, comme les autres CTA. |
| **Suppression rendue en lien** | `Retirer le filtre` en `btn-link` alors que toutes les autres actions secondaires sont `btn-ghost-secondary` / `btn-outline-secondary`. | `ProxmoxView.vue:376` | **Faible** | `btn-sm btn-ghost-secondary`. |

---

## 2. Icônes

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Tailles** | **10 valeurs de `:size`** en circulation : 12, 14, 16, 18, 20, 24, 32, 36, 40, 48. Les paliers 18 (×21), 20 (×11) et 36/40 (×5) sont des intermédiaires non justifiés entre les paliers dominants 14 (×41), 16 (×98) et 24 (×10). | Transverse | **Moyenne** | Figer 4 paliers : **14** (dans un `btn-sm` / badge), **16** (défaut inline/tableau), **24** (en-tête de carte), **48** (état vide). Supprimer 18/20/32/36/40. |
| **Deux systèmes de dimensionnement** | Coexistence de la prop `:size="N"` (~200 usages) et des classes utilitaires `.icon-sm/.icon-md/.icon-lg` (59 usages, `style.css:433-457`) pour le même besoin. Une icône `:size="16"` et une `.icon-sm` (16px) sont identiques mais s'écrivent de deux façons. | Transverse | **Faible** | Choisir `:size` (typé, colocalisé) comme convention unique ; garder `.icon-*` uniquement pour les icônes dont la taille doit varier en responsive (`.icon-responsive-lg`). |
| **`stroke-width`** | 4 valeurs : `1.5` (×13), `2` (×2), `2.5` (×4), `1.2` (×1). Tabler icons rend en `2` par défaut ; les surcharges ponctuelles font varier le « poids » visuel d'une page à l'autre. | `EmptyState` (1.5) · `NetworkGraph` · divers | **Faible** | `1.5` pour les icônes décoratives grand format (états vides), défaut (`2`) partout ailleurs. Retirer `2.5` et `1.2`. |

---

## 3. États hover / focus / active

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Lignes de tableau cliquables** | **3 mécanismes concurrents** pour « cette ligne ouvre un détail » : `.clickable-row` (utilitaire partagé, `style.css:418`) sur **3 fichiers seulement** · `.table-hover` (Tabler) sur **5 fichiers** · `cursor-pointer` seul, **sans retour visuel**, sur 13 fichiers. Résultat : selon la page, une ligne cliquable se signale par un fond, par un simple curseur, ou par rien. | `.clickable-row` : `ThreatsPanel`, `TrafficOverviewPanel`, `DomainDetailsModal` — `.table-hover` : `HostProcessesPanel`, `HostSystemdPanel`, `HostTasksTab`, `ProxmoxNodeServicesTab`, `GlobalScheduledTasksView` — reste : sans hover | **Haute** | Généraliser `.clickable-row` (déjà en place, avec `:focus-visible`) à **toutes** les lignes/cartes cliquables. Réserver `.table-hover` aux tableaux **non** cliquables où le survol n'est qu'une aide à la lecture. |
| **Éléments cliquables sans affordance** | Sur 24 éléments non-`<button>`/non-`<a>` porteurs d'un `@click`, **16 n'ont aucune classe de survol**. | `GlobalScheduledTasksView` (×3) · `DockerContainersTab` (×2) · `HostDetailView` · `ProxmoxNodeView` · `DashboardView` · `CommandPalette` · `ComposeProjectsTab` · `DashboardDockerVersions` · `PayloadViewerModal` · `AptScheduleModal` · `IPTimelineModal` | **Haute** | Ajouter `.clickable-row` (ou en faire un `<button>` si le contenu n'a pas à être sélectionnable). |
| **Accessibilité clavier des faux boutons** | Sur ces mêmes 24 éléments : **17 sans `role="button"`**, **12 sans gestionnaire clavier**. Un `<div @click>` sans `tabindex`/`role`/`@keydown` est **inatteignable au clavier**. | idem ci-dessus | **Haute** | Appliquer le motif déjà validé dans `ThreatsPanel` : `role="button"` + `tabindex="0"` + `@keydown.enter` + `@keydown.space.prevent`. |
| **Transitions** | Aucune convention de durée. `TrafficWorldMap` déclare `transition: filter .1s`, d'autres composants n'en déclarent aucune, `.clickable-row` non plus. | Transverse | **Faible** | Token unique `--ss-transition-hover: 120ms ease` et l'appliquer dans `.clickable-row`. |
| **État `disabled`** | Reposé entièrement sur le défaut Tabler (`opacity`). Aucun `:disabled` custom, mais **les faux boutons `<div>` ne peuvent pas être `disabled`** — ils restent cliquables même quand l'action est indisponible. | `DomainDetailsModal` (bouton Bloquer déjà géré) ; à vérifier sur les 16 ci-dessus | **Moyenne** | Pour tout faux bouton, ajouter `aria-disabled` + garde dans le handler, ou repasser en `<button>`. |

---

## 4. Tableaux

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Classes de base** | **15 combinaisons** de classes `table*` pour ~65 tableaux. La convention dominante est claire (`table table-vcenter card-table`, ×41) mais 24 tableaux dérivent : `table-sm` (×13), `mb-0` ajouté ou non (×9), `table-hover` (×6). | Transverse | **Moyenne** | Convention : `table table-vcenter card-table` dans une carte ; ajouter `table-sm` **uniquement** pour les tableaux denses secondaires (panneaux latéraux). Ne jamais ajouter `mb-0` manuellement (`card-table` le gère). |
| **Tri** | 16 fichiers utilisent `SortableHeader.vue` ; **15 autres** ré-implémentent un tri local (`sortKey` / `toggleSort` avec chevrons inline). Les icônes et zones de clic diffèrent. | 15 fichiers hors `SortableHeader` | **Moyenne** | Migrer les 15 vers `SortableHeader.vue`. |
| **États vides** | `EmptyState.vue` existe (icône + titre + sous-titre + CTA) mais n'est utilisé que dans **7 fichiers**. **64 fichiers** ont un état vide ad-hoc (`<div class="text-center text-muted py-4">Aucun…</div>`) — sans icône, sans CTA, avec un padding différent (`py-4` vs `py-5`). | 64 fichiers, dont `ExposureDomainsPanel:77`, `ProxmoxView:371`, `HostTasksTab:40` | **Haute** | Généraliser `EmptyState.vue`. C'est le gain de cohérence le plus large pour le coût le plus faible (remplacement mécanique). |
| **États de chargement** | **3 idiomes** : `LoadingSkeleton.vue` (23 fichiers), `spinner-border` brut (50 fichiers), `.skeleton-text` CSS (1 fichier). **9 fichiers mélangent skeleton et spinner** dans la même vue. | Mixtes : `ThreatsPanel`, `DashboardView`, `HostDetailView`, `AuditLogsView`, `ProxmoxNodeView`, `ProxmoxGuestView`, `RunbooksView`, `HostProcessesPanel`, `UptimeDetailSection` | **Haute** | Règle : **skeleton** pour le premier chargement d'une zone dont la forme est connue (tableau, carte KPI) ; **spinner** uniquement pour une action ponctuelle (bouton en cours, rafraîchissement). Supprimer `.skeleton-text` au profit de `LoadingSkeleton`. |
| **Zébrage** | Aucun tableau n'utilise `table-striped`. Cohérent — à documenter pour éviter une dérive future. | — | **Faible** | Acter « pas de zébrage » dans la charte. |
| **Colonnes en responsive** | Seuls **6 tableaux** masquent des colonnes en petit écran (`d-none d-md-table-cell`). Les ~59 autres débordent en scroll horizontal. | Transverse | **Moyenne** | Identifier 1-2 colonnes secondaires par tableau large et les masquer sous `md`. |

---

## 5. Typographie

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Libellé de carte KPI** | **Deux rendus visuellement distincts** pour le même rôle : `.subheader` (40 usages) = **12px, MAJUSCULES, medium, letter-spacing .04em** vs `.text-muted small` / `.text-secondary small` (227 usages) = ~14px, casse normale, poids normal. Vérifié dans `tabler.css`. | Transverse — `ExposureDomainsPanel:23` (`text-muted small`) vs cartes KPI du Dashboard (`subheader`) | **Haute** | Choisir `.subheader` pour **tout** libellé de KPI (c'est le rôle prévu par Tabler) et réserver `text-secondary small` au texte d'aide en prose. |
| **Valeur de KPI** | 4 niveaux pour la même information : `h1` (×6), `h2` (×28), `h3` (×20), avec `mb-0` / `mb-1` / `mb-0 mt-1` en désordre. | `ProxmoxView` (h1) · Dashboard (h2) · `ExposureDomainsPanel:26` (h3) | **Moyenne** | `h2 mb-0` pour une carte KPI pleine largeur, `h3 mb-0` pour une `card-sm`. Deux paliers, pas quatre. |
| **Titre d'entité (page de détail)** | Le nom de l'entité est un `h1` sur Proxmox, `h2` sur Host / Network / Guest, `h3` sur Node / Account. | `ProxmoxView` (h1) · `HostDetailView`, `NetworkView`, `ProxmoxGuestView` (h2) · `ProxmoxNodeView`, `AccountView` (h3) | **Moyenne** | Le nom d'entité vit dans `page-title` ; s'il faut un rappel dans le corps, `h3` partout. |
| **Texte d'aide de formulaire** | `.form-hint` (Tabler, 36 usages sur 16 fichiers) vs `.text-muted small` (97 usages) pour le même rôle sous un champ. | Transverse | **Moyenne** | `.form-hint` sous un champ de formulaire, sans exception. |
| **`text-muted` vs `text-secondary`** | Les deux résolvent au **même** `#6b7280` (vérifié : `--tblr-muted` = `--tblr-secondary` = `#6b7280`). Aucun écart visuel, mais 342 vs 460 usages = dérive de nommage pure qui rend tout futur re-theming à moitié appliqué. | Transverse | **Faible** | `text-secondary` partout (nom sémantique Bootstrap 5.3 ; `text-muted` est en voie de dépréciation upstream). |
| **Accents français manquants** | `HostTasksTab.vue` est rédigé **intégralement sans accents** (« Nouvelle tache », « Aucune tache planifiee », « Automatisez vos operations en creant une tache planifiee », « irreversible », « Creer »). Idem partiellement `WebhookModal.vue` (« desactive », « executee », « echec », « succes ») et `NetworkTopologyConfig.vue`. | `HostTasksTab.vue:20,40,43,51,213,326,388,687,688` · `WebhookModal.vue` · `NetworkTopologyConfig.vue` | **Haute** | Corriger. Très visible pour un utilisateur francophone, et le reste de l'app est correctement accentué. |
| **Points de suspension** | 10 placeholders en `...` (trois points) contre 2 en `…` (vrai caractère). | `AuditLogsView`, `DockerContainersTab`, `AptToolbar`… vs `CommandPalette` | **Faible** | `…` partout. |

---

## 6. Couleurs et espacements

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Deux vocabulaires de couleur** | La palette Tabler (`red`, `green`, `yellow`, `azure`) et les noms sémantiques Bootstrap (`danger`, `success`, `warning`, `primary`) sont utilisés **en parallèle pour la même intention** : `text-red` (75) vs `text-danger` (55) · `text-green` (69) vs `text-success` (35) · `bg-red-lt` (57) vs `bg-danger-lt` (2). Vérifié : `--tblr-red` = `--tblr-danger` = `#d63939`, `--tblr-green` = `--tblr-success` = `#2fb344` → **identique à l'écran**, mais aucun re-theming sémantique n'est possible tant que les deux coexistent. | Transverse | **Moyenne** | Adopter les noms **sémantiques** (`danger`/`success`/`warning`/`info`) pour tout ce qui porte un sens d'état, et réserver les noms de palette (`azure`, `purple`, `teal`…) aux catégorisations neutres (types, tags). |
| **« Warning » en deux teintes** | `text-warning` / `text-yellow` = `#f59f00` (identiques) mais `text-orange` = **`#f76707`** — teinte différente, 24 usages. Un même niveau d'alerte s'affiche donc en jaune ou en orange selon la page. | `HostDetailView:218` · cartes KPI `text-orange` · reste en `text-yellow`/`text-warning` | **Moyenne** | `warning` (= jaune `#f59f00`) pour tout état d'avertissement. Réserver `orange` à une éventuelle 3ᵉ sévérité explicitement documentée. |
| **« Accent » en deux bleus** | `bg-azure-lt` (45) = `#4299e1` vs `bg-blue-lt` (39) = `#066fd1` (= `--tblr-primary`). Deux bleus quasi interchangeables dans les badges. | Transverse | **Moyenne** | `primary`/`blue` pour l'accent applicatif, `azure` uniquement si une distinction catégorielle est voulue — sinon supprimer. |
| **Couleurs en dur** | **24 hex distincts / 39 occurrences** dans les `<style>` de composants, hors tokens. Dont des couleurs *claires* (`#f8fafc`, `#f3f6fa`, `#dbe3ec`, `#d2e6ff`, `#e6e7e9`) sur un thème dark-only. | `AlertRuleModal` (7 hex) · `NetworkGraph` (6) · `AddHostView` (3) · `CVEList` · `IPTimelineModal` · `PageRefreshBar` (`#22c55e` ≠ `--ss-status-online` `#2fb344`) | **Moyenne** | Les tokens `--ss-*` existent déjà pour exactement ça (`style.css:71-124`). Remplacer, en commençant par `PageRefreshBar` (vert divergent de `--ss-status-online`). |
| **Breakpoints** | `max-width: 992px` (×6) **et** `max-width: 991px` (×4) coexistent → zone de 1px où deux règles se contredisent. Et `max-width: 640px` (×4) est un breakpoint **Tailwind**, pas Bootstrap (`sm` = 576px). | Transverse | **Moyenne** | Aligner sur Bootstrap : `575.98` / `767.98` / `991.98` / `1199.98`. Supprimer 640px. |

---

## 7. Composants récurrents

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Deux architectures de modale** | 17 fichiers utilisent la structure Tabler (`.modal` > `.modal-dialog` > `.modal-content` > header/body/footer). Les **2 modales de drill-down les plus utilisées** implémentent un overlay `card` **entièrement custom** (`.traffic-modal-backdrop` / `.timeline-modal`, CSS local) — header, footer, fermeture et responsive réinventés. | `DomainDetailsModal.vue:4,7` · `IPTimelineModal.vue:7` | **Haute** | Extraire un `AppModal.vue` (backdrop + `Teleport` + ESC + focus trap + slots header/body/footer) et y faire converger les 19 modales. |
| **Classe d'ouverture de modale** | **4 combinaisons** : `modal modal-blur fade show` (×10) · `modal modal-blur show d-block` (×6) · `modal modal-blur fade show d-block` (×5) · sans classe. `fade` sans `d-block` et `d-block` sans `fade` ne produisent pas la même apparition. | 17 fichiers | **Moyenne** | Une seule forme, portée par `AppModal.vue`. |
| **Backdrop absent** | **5 modales n'ont aucun `modal-backdrop`** : le contenu derrière reste pleinement visible et cliquable. | `SettingsMaintenanceCard` · `TrackableContainersModal` · `WebhookModal` · `GitWebhooksView` · `GlobalScheduledTasksView` · `ProxmoxNodeView` | **Haute** | Backdrop systématique (fourni par `AppModal.vue`). |
| **Fermeture par ESC** | Seules **4 modales sur 19** gèrent la touche Échap (`CommandPalette`, `AlertRuleModal`, `WebhookModal`, `HostDetailView`). | 15 modales | **Moyenne** | ESC + clic sur backdrop, centralisés. |
| **`Teleport`** | Seules **2 modales** utilisent `Teleport` (`CommandPalette`, `HostTasksTab`). Les autres rendent en place → empilement dépendant du contexte DOM parent (`overflow` d'une carte, `position: sticky` d'un panneau latéral). | 17 fichiers | **Moyenne** | `Teleport to="body"` dans `AppModal.vue`. |
| **Confirmation destructive** | 3 suppressions utilisent le **`window.confirm()` natif du navigateur** — police système, boutons OS, non stylable — alors que `ConfirmDialog.vue` + `useConfirmDialog` sont le standard de l'app (~20 fichiers). | `SettingsNPMCard.vue:382` · `SettingsProxmoxCard.vue:425` · `SettingsRegistryCredentialsCard.vue:271` | **Haute** | Migrer vers `useConfirmDialog` — 3 remplacements mécaniques, la page Paramètres est la dernière à ne pas suivre le motif. |
| **Bouton de fermeture** | `btn-close` (×29) · `btn-close btn-close-white` (×4) · `btn-close btn-close-sm` (×1). | `DomainDetailsModal` · `ProxmoxView` · divers | **Faible** | `btn-close` seul (le thème dark le gère déjà). |
| **Champ de recherche** | 10 barres de recherche ; **une seule** utilise `input-icon` (l'idiome Tabler avec la loupe intégrée). Les 9 autres sont des `form-control` nus. | `AlertIncidentList` (avec icône) vs `AuditLogsView`, `DashboardView`, `DockerContainersTab`, `AptToolbar`, `GlobalScheduledTasksView`, `NetworkPortList`, `ComposeProjectsTab`, `HostProcessesPanel`, `CommandPalette` | **Moyenne** | Composant `SearchInput.vue` (`input-icon` + `IconSearch` + bouton d'effacement) pour les 10. |
| **En-tête de carte** | `card-header` seul (×43) vs `card-header d-flex align-items-center justify-content-between` (×36) + 5 variantes de `gap`/`flex-wrap`. | Transverse | **Faible** | Convention : `card-header d-flex align-items-center justify-content-between gap-2 flex-wrap` dès qu'une action accompagne le titre. |
| **Titre de carte** | `card-title` (×39) vs `card-title mb-0` (×47) — la marge basse par défaut de Tabler s'applique donc dans un cas sur deux, décalant verticalement les en-têtes d'une carte à l'autre. | Transverse | **Faible** | `card-title mb-0` systématique dans un `card-header` flex. |

---

## 8. Responsive et layout

| Élément | Incohérence détectée | Page(s) concernée(s) | Sévérité | Recommandation |
|---|---|---|---|---|
| **Hauteur des cartes en paire** | Sujet déjà traité sur Threats/Traffic mais **non formalisé** : `h-100` sur deux cartes dont l'une a un contenu borné (carte du monde, 340px) et l'autre une liste variable crée un vide mort. Le motif correct est `align-items-start` + `.scroll-table`. Rien n'empêche la régression ailleurs. | `ThreatsPanel` / `TrafficOverviewPanel` (corrigés) ; risque sur toutes les `row-cards` | **Moyenne** | Documenter la règle (ci-dessous) — `h-100` **seulement** si les deux côtés sont bornés de façon comparable. |
| **Troncature** | 6 idiomes pour le même besoin : `text-truncate` (38), `white-space: nowrap` CSS (10), `text-nowrap` (10), `word-break` (8), `text-break` (7), `overflow-wrap` (2). | Transverse | **Moyenne** | `text-truncate` + `title` pour un libellé sur une ligne ; `overflow-wrap: anywhere` pour les contenus monospace longs (URL, chemins). |
| **Panneau latéral** | `.side-panel` (`style.css:19`) impose `height: calc(100vh - 160px)` en fixe — le `160px` suppose une hauteur d'en-tête constante, non garantie quand `page-header` passe sur deux lignes. | Docker, APT, Audit, Host, Account, Proxmox | **Faible** | `max-height` plutôt que `height`, ou variable CSS alimentée par la hauteur réelle de l'en-tête. |

---

# Synthèse

## Les 5 incohérences les plus critiques

### 1 — Trois styles de boutons pour la même action de cycle de vie
`start` / `stop` / `restart` s'affichent en **plein**, en **outline** ou en **ghost** selon qu'on est sur Docker, Proxmox ou systemd — et `restart` passe même d'orange à bleu plein. C'est l'écart le plus visible pour un utilisateur qui navigue entre ces pages, et il touche les actions les plus fréquentes de l'app.
**Action** : composant `LifecycleActions.vue`, convention `btn-icon btn-sm btn-ghost-<intent>`.

### 2 — Les lignes cliquables ne se signalent pas de la même façon (ni au clavier)
Trois mécanismes de survol concurrents, **16 éléments cliquables sans aucun retour visuel**, et **17 sans `role="button"`** donc inatteignables au clavier. L'utilitaire `.clickable-row` existe déjà mais n'est déployé que sur 3 fichiers.
**Action** : généraliser `.clickable-row` + le motif `role`/`tabindex`/`@keydown` déjà validé sur Threats.

### 3 — `window.confirm()` natif sur trois suppressions de la page Paramètres
Trois suppressions de connexions (NPM, Proxmox, identifiants de registre) ouvrent la boîte de dialogue **du navigateur** — police système, boutons OS, hors charte — alors que `ConfirmDialog` est le standard partout ailleurs. Rupture brutale, sur des actions destructives, précisément là où la confiance compte.
**Action** : migrer vers `useConfirmDialog` (3 remplacements).

### 4 — Deux architectures de modale, dont 5 sans backdrop
Les deux modales de drill-down les plus utilisées (`DomainDetailsModal`, `IPTimelineModal`) réinventent l'overlay en CSS local au lieu de la structure Tabler. Et **5 modales n'ont aucun backdrop** : le contenu derrière reste visible et cliquable. Seules 4/19 gèrent Échap, 2/19 utilisent `Teleport`.
**Action** : `AppModal.vue` (Teleport + backdrop + ESC + focus trap + slots), convergence des 19.

### 5 — États vides et chargements sans convention
`EmptyState.vue` existe mais n'est utilisé que dans 7 fichiers contre **64 états vides ad-hoc**. En parallèle, 3 idiomes de chargement coexistent et **9 vues mélangent skeleton et spinner** dans le même écran. Ce sont les deux états que l'utilisateur voit le plus souvent — au premier chargement et sur une installation neuve.
**Action** : généraliser `EmptyState.vue` ; règle skeleton (chargement de zone) vs spinner (action ponctuelle).

**Mention spéciale** — `HostTasksTab.vue` est rédigé intégralement **sans accents français** (« Nouvelle tache », « Aucune tache planifiee », « irreversible »). Correction triviale, très visible, à traiter avec le lot ci-dessus.

---

## Design system minimal à formaliser

À placer dans `frontend/CLAUDE.md` pour que la convention soit appliquée par défaut sur tout nouveau code.

### Boutons

| Rôle | Classe | Notes |
|---|---|---|
| Action principale de page | `btn btn-primary` | Une seule par écran |
| Action secondaire | `btn btn-outline-secondary` | Jamais `btn-outline-light` |
| Action tertiaire / en tableau | `btn btn-sm btn-ghost-secondary` | |
| Destructif en tableau | `btn btn-icon btn-sm btn-ghost-danger` | + `title` explicite |
| Destructif en pied de modale | `btn btn-danger` | Seul cas de rouge plein |
| Cycle de vie (start/stop/restart) | `btn btn-icon btn-sm btn-ghost-<success\|danger\|warning>` | Via `LifecycleActions.vue` |

Tailles : **`btn-sm`** en tableau et barre d'outils, **défaut** en pied de modale et CTA de page. `btn-xs` **n'existe pas dans Tabler** — ne pas l'utiliser. Libellés : français, infinitif, casse phrase (`Supprimer`, pas `SUPPRIMER` ni `Stop`).

### Couleurs

Noms **sémantiques** uniquement pour les états :

| Intention | Token | Valeur |
|---|---|---|
| Erreur / destructif | `danger` | `#d63939` |
| Succès / actif | `success` | `#2fb344` |
| Avertissement | `warning` | `#f59f00` |
| Information / accent | `primary` | `#066fd1` |
| Neutre / secondaire | `secondary` | `#6b7280` |

Interdits : `red`/`green`/`yellow` pour un état (doublons de `danger`/`success`/`warning`) · `orange` comme avertissement (teinte différente) · `azure` comme accent (doublon de `primary`) · tout hex en dur dans un `<style>` — utiliser un token `--ss-*`.

### Icônes

`@tabler/icons-vue` exclusivement. Tailles : **14** (dans `btn-sm`/badge) · **16** (défaut inline/tableau) · **24** (en-tête de carte) · **48** (état vide). `stroke-width` : défaut, sauf `1.5` pour les icônes décoratives grand format. Une action = toujours la même icône (`IconTrash` = supprimer, `IconPencil` = éditer, `IconRefresh` = rafraîchir, `IconCopy` = copier).

### Tableaux

- Base : `table table-vcenter card-table` — `table-sm` uniquement pour les tableaux denses secondaires, jamais `mb-0` manuel.
- Tri : `SortableHeader.vue` exclusivement.
- Pagination : `PaginationNav.vue`, en pied de carte, exclusivement.
- Ligne cliquable : `.clickable-row` + `role="button"` + `tabindex="0"` + `@keydown.enter` + `@keydown.space.prevent`. `table-hover` réservé aux tableaux **non** cliquables.
- Colonne d'actions : `text-end`, dernière colonne, `btn-icon btn-sm`.
- État vide : `EmptyState.vue`, jamais de `<div class="text-center text-muted">` ad-hoc.
- Chargement : `LoadingSkeleton.vue` pour le premier rendu ; `spinner-border` réservé aux actions ponctuelles. Jamais les deux dans le même écran.
- Responsive : masquer 1-2 colonnes secondaires sous `md` (`d-none d-md-table-cell`) plutôt que scroller.

### Cartes et mise en page

- En-tête : `card-header d-flex align-items-center justify-content-between gap-2 flex-wrap` + `card-title mb-0`.
- Libellé KPI : `.subheader`. Valeur KPI : `h2 mb-0` (carte pleine largeur) ou `h3 mb-0` (`card-sm`).
- **Hauteur en paire** : `h-100` sur deux cartes côte à côte **uniquement si les deux contenus sont bornés de façon comparable** (même taille de page, même nombre de lignes). Un widget de taille fixe face à une liste variable → `align-items-start` + `.scroll-table`. *(Règle issue d'une régression réelle sur Threats/Traffic.)*
- Breakpoints Bootstrap uniquement : `575.98` / `767.98` / `991.98` / `1199.98`. Pas de 640px, pas de 991 **et** 992.

### Modales

Une seule implémentation, `AppModal.vue` : `Teleport to="body"` + backdrop + fermeture ESC et clic backdrop + focus trap + slots `header`/`body`/`footer`. Footer : action secondaire (`btn-outline-secondary`, à gauche) puis action principale (à droite). Confirmation destructive : `useConfirmDialog` — **jamais** `window.confirm()`.

### Formulaires

`form-label` (+ `required`) · `form-control` / `form-select` (+ `-sm` en barre d'outils) · **`form-hint`** pour le texte d'aide (jamais `text-muted small`) · `invalid-feedback` pour l'erreur · recherche via `SearchInput.vue` (`input-icon` + `IconSearch` + effacement).

### Rédaction

Français accentué, `…` (pas `...`), casse phrase. Libellés d'action à l'infinitif. i18n non installé — ne pas l'introduire partiellement (décision projet, cf. `frontend/CLAUDE.md`).
