# Runbooks et Tâches planifiées

Deux mécanismes pour déclencher une action agent (apt/docker/systemd/journal/
processus/custom/restic) sans passer par un clic manuel à chaque fois. Ils
partagent le même composant de formulaire (`DispatchStepEditor`) mais
reposent sur des modèles de confiance très différents — c'est le point le
plus important à comprendre avant de choisir lequel utiliser.

---

## 1. Runbooks — séquences multi-étapes, admin uniquement

Dashboard → **Automatisation** → **Runbooks** (admin uniquement, toute la
section).

Un runbook est une suite ordonnée d'étapes, chacune ciblant **un hôte** et
**une action déjà whitelistée côté serveur** :

| Champ (étape) | Rôle |
|---|---|
| Hôte | Cible de cette étape |
| Module | `docker` / `apt` / `systemd` / `journal` / `processes` / `custom` |
| Action | Dépend du module (voir tableau ci-dessous) |
| Cible (`target`) | Requis pour `journal` / `systemd` / `custom`, sinon optionnel |
| Continuer même si cette étape échoue | Décoché par défaut — une étape en échec arrête tout le runbook |

Actions whitelistées par module (identiques côté frontend et serveur — le
serveur revalide indépendamment, le frontend ne fait que refléter la même
liste) :

| Module | Actions autorisées |
|---|---|
| `docker` | `logs`, `restart`, `start`, `stop`, `compose_up`, `compose_down`, `compose_pull`, `compose_logs`, `compose_restart` |
| `apt` | `update`, `upgrade`, `full-upgrade`, `autoremove` |
| `systemd` | `status`, `start`, `stop`, `restart`, `list` |
| `journal` | `read` |
| `processes` | `list` |
| `custom` | `run` |

`restic` n'est **pas** dans cette liste — un runbook ne peut pas déclencher
un backup Restic (voir [§3](#3-lasymétrie-en-un-coup-dœil)).

Un runbook nécessite **au moins une étape** à la création (`min=1` côté
validation serveur).

### Déclenchement et progression

**Manuel uniquement** — pas de cron, pas de déclenchement par une règle
d'alerte. `POST /api/v1/runbooks/:id/run` ne dispatche que la **première**
étape. La progression est gérée en réaction à la fin de chaque commande :
si l'étape échoue et que `continue_on_failure` est décoché, l'exécution
entière passe `failed` ; sinon (succès, ou échec toléré) l'étape suivante
est dispatchée automatiquement. Plus d'étape suivante → exécution
`completed`.

Après un clic sur **Exécuter**, l'UI ouvre l'historique et interroge
`GET .../executions` toutes les 3 secondes jusqu'à un statut terminal — il
n'y a pas de flux push pour la progression multi-étapes, seul le résultat
de chaque commande individuelle est streamé en direct (bouton logs par
étape).

## 2. Tâches planifiées — un dispatch par hôte, sur cron

Dashboard → **Tâches planifiées**.

| Champ | Rôle |
|---|---|
| Hôte | Cible unique |
| Module | `apt` / `docker` / `systemd` / `journal` / `processes` / `custom` / **`restic`** |
| Action | Texte libre suggéré par une liste indicative selon le module — **non validé côté serveur**. Masqué et facultatif pour `module=custom`, où l'agent l'ignore entièrement (seul l'identifiant de tâche compte) |
| Cible (`target`) | Selon module (ex : nom de profil Restic, unité systemd, identifiant de tâche `custom`…) |
| Planification | `CronBuilder` — mode **Visuel** (fréquence quotidienne/hebdo/mensuelle/personnalisée) ou **Expert** (cron brut à 5 champs, `minute heure jour-du-mois mois jour-de-la-semaine`) ; l'heure saisie est interprétée dans le fuseau du serveur, pas du navigateur — voir [§2.4](#24-fuseau-horaire-dexécution) |
| Exécution manuelle uniquement | Coché = pas de planification — voir [§2.1](#21-exécution-manuelle-uniquement--le-détail-dimplémentation) |

### 2.1 "Exécution manuelle uniquement" — le détail d'implémentation

Il n'existe pas de colonne dédiée "manuel" en base : cocher cette case
force `enabled=false` et remplace l'expression cron par une sentinelle
(`0 0 29 2 *` — 29 février, ne se déclenche jamais). C'est purement une
convention frontend ; si vous interrogez l'API directement, une tâche
"manuelle" ressemble à une tâche désactivée avec un cron impossible, pas à
un type distinct.

### 2.2 Lancer une tâche custom sans la planifier

Une tâche déclarée dans le `tasks.yaml` d'un hôte n'a pas besoin d'une tâche
planifiée pour être exécutée une fois : la fiche hôte a un onglet **Tâches
personnalisées** qui les liste (telles que l'agent les voit) avec un bouton
**Exécuter** — `POST /api/v1/hosts/:id/custom-tasks/:taskId/run`, Operator+
sur l'hôte, aucune ligne persistée dans `scheduled_tasks`.

Créez une tâche planifiée `module=custom` uniquement quand vous voulez
réellement une récurrence (ou une entrée réutilisable à déclencher d'un
clic). Pour un « lance-moi ça maintenant », l'onglet est le bon chemin —
c'est l'équivalent des boutons ad-hoc que `docker`/`apt`/`systemd` ont déjà.

### 2.3 Modules non listés

Le sélecteur d'action bascule automatiquement en champ texte libre pour
tout module sans liste d'actions suggérées prédéfinie — c'est voulu, pas un
bug d'UI : le serveur ne validant pas `action`, l'UI ne peut de toute façon
pas prétendre connaître la liste exhaustive.

### 2.4 Fuseau horaire d'exécution

Le scheduler cron (`robfig/cron`) interprète les champs heure/minute de
chaque tâche dans le fuseau horaire du **process serveur** (`time.Local`),
pas dans celui du navigateur utilisé pour la créer. Sans configuration
explicite, le conteneur Docker tourne en UTC : une tâche planifiée pour
"23h00" via `CronBuilder` se déclenche réellement à 23h00 **UTC**, soit
01h00 le lendemain pour un opérateur en UTC+2 (heure d'été Europe/Paris) —
`CronBuilder` affiche un hint le rappelant, dans les deux modes (Visuel et
Expert).

Pour aligner l'exécution sur votre propre fuseau, définissez la variable
d'environnement `TZ` du service `server` (ex : `TZ=Europe/Paris` dans
`.env`, voir [`docker-compose.yml`](../docker-compose.yml) et
[`.env.example`](../.env.example)) — Go/tzdata la respecte nativement (le
paquet `tzdata` est déjà installé dans l'image finale), aucun rebuild n'est
nécessaire, un simple `docker compose up -d --force-recreate server`
suffit. Le champ "Prochaine exécution" affiché dans l'UI est, lui, toujours
converti dans le fuseau du **navigateur** ; une fois `TZ` réglé côté
serveur sur votre propre fuseau, les deux coïncident.

## 3. L'asymétrie, en un coup d'œil

| | Runbooks | Tâches planifiées |
|---|---|---|
| Qui peut créer/modifier/supprimer | **Admin uniquement** (toute la section) | `Operator`+ (vérifié par hôte, `requireHostAccess(..., "operator")`) |
| Qui peut exécuter manuellement | Admin (dans le groupe admin-only) | `Operator`+ (vérifié par hôte, `requireHostAccess(..., "operator")`) |
| `action` validée côté serveur ? | **Oui** — whitelist stricte par module | **Non** — seul `module` est vérifié dans une liste connue, `action` est fait confiance |
| Modules disponibles | 6 (docker/apt/systemd/journal/processes/custom) | 7 (les 6 + `restic`) |
| Peut cibler plusieurs hôtes en un déclenchement | Oui — un runbook entier peut traverser toute la flotte | Non — un hôte par tâche |
| Peut être planifiée (cron) | Non — manuel uniquement | Oui |

Cette asymétrie est un choix documenté, pas un oubli : un runbook peut
enchaîner des actions sur plusieurs hôtes différents en un seul
déclenchement (surface de casse plus large), d'où le verrouillage
admin-only + whitelist stricte. Une tâche planifiée reste bornée à un seul
hôte à la fois.

> **Historique** : jusqu'à la correction de cette faille (voir
> [ROADMAP.md](../ROADMAP.md), item #1), la création/modification/
> suppression n'étaient vérifiées par aucun rôle côté API — un `viewer`
> capable d'appeler l'API directement (pas via le dashboard, qui masquait
> déjà ces actions) pouvait créer une tâche planifiée sur n'importe quel
> hôte. Les trois handlers (`Create`/`Update`/`DeleteScheduledTask`)
> appellent désormais `requireHostAccess(..., "operator")` sur l'hôte
> ciblé, exactement comme `run` — `Update`/`Delete` résolvent d'abord la
> tâche pour retrouver son `host_id` (`:id` dans l'URL est l'id de la
> tâche, pas de l'hôte).

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Un runbook s'arrête après une étape en échec sans continuer | Comportement par défaut — cochez `continue_on_failure` sur cette étape si l'échec ne doit pas bloquer la suite |
| Impossible d'ajouter `restic` comme module dans un runbook | Non supporté par design — utilisez une tâche planifiée pour un backup Restic récurrent, ou le bouton **Lancer un backup** de l'onglet Sauvegardes pour un déclenchement ponctuel |
| L'action d'une tâche planifiée semble accepter n'importe quel texte | Normal — seul `module` est validé côté serveur, `action` est fait confiance (voir [§2.3](#23-modules-non-listés)) ; une faute de frappe échouera silencieusement côté agent, pas côté validation |
| Une tâche "manuelle" apparaît quand même dans un export/API brut avec un cron | C'est la sentinelle `0 0 29 2 *` + `enabled=false` (voir [§2.1](#21-exécution-manuelle-uniquement--le-détail-dimplémentation)) — pas un vrai cron actif |
| La "Prochaine exécution" affichée ne correspond pas à l'heure définie dans le cron (ex : décalée de 1-2h) | Le serveur tourne en UTC par défaut — définissez `TZ` sur votre propre fuseau (voir [§2.4](#24-fuseau-horaire-dexécution)) |
| Runbook bloqué en `running` sans jamais passer `completed`/`failed` | La commande de l'étape en cours n'a jamais atteint un état terminal côté agent (agent déconnecté, commande perdue) — vérifiez les logs de l'étape en cours avant de relancer |

## Pour aller plus loin

Voir aussi [Webhooks Git et suivi de releases](git-webhooks-releases.md) pour
l'autre famille de déclencheurs, [Sauvegardes Restic](backup-restic.md) pour
le détail du module `restic` utilisable en tâche planifiée, et la section
[Runbooks & Tâches planifiées](../README.md#runbooks--tâches-planifiées) du
README pour le tableau complet des routes API.
