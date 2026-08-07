# Webhooks Git et suivi de releases

Deux façons complémentaires de réagir à un événement Git dans
ServerSupervisor : le **suivi de releases** interroge périodiquement une
API de dépôt ou de registre Docker (modèle *pull*), tandis qu'un **webhook
Git** est appelé directement par votre plateforme Git à chaque push/tag
(modèle *push*, temps réel). Les deux peuvent, optionnellement, déclencher
la même chose côté agent : une [tâche custom](../README.md#tâches-custom-tasksyaml)
déclarée dans `tasks.yaml`.

---

## 1. Suivi de releases (Release Trackers)

Dashboard → **Git / Automatisation** → onglet **Suivi de releases**.

### 1.1 Tracker Git (GitHub / GitLab / Gitea)

| Champ | Valeur |
|---|---|
| Nom | Libre |
| Provider | `github` (défaut) / `gitlab` / `gitea` |
| Dépôt | `owner` + `repo` (deux champs séparés) |
| Déclencher une tâche lors d'une mise à jour | Décoché = **surveillance seule**, enregistre juste la nouvelle version détectée. Coché = exécute une tâche `tasks.yaml` sur l'hôte choisi |
| Cooldown (heures) | `0`–`168`, délai avant de redéclencher après détection |

> **Jetons d'authentification — limite actuelle** : une seule variable
> `GITHUB_TOKEN` existe côté serveur, réutilisée telle quelle comme
> `Authorization: Bearer` (GitHub) ou `PRIVATE-TOKEN` (GitLab). Il n'y a
> **pas** de `GITLAB_TOKEN`/`GITEA_TOKEN` dédié, et les clients GitLab/Gitea
> sont **codés en dur** sur `gitlab.com`/`gitea.io` — un dépôt sur une
> instance GitLab ou Gitea auto-hébergée n'est actuellement pas supporté
> par le suivi de releases. Sans `GITHUB_TOKEN`, GitHub reste utilisable en
> anonyme (rate limit public, 60 req/h).

### 1.2 Tracker Docker (registre d'images)

| Champ | Valeur |
|---|---|
| Image Docker | Requis (ex : `linuxserver/sonarr`) |
| Tag surveillé | Défaut `latest` |
| Déclencher une tâche lors d'une mise à jour | Idem — décoché = surveillance seule |
| Mode | **Compose (natif)** ou **Tâche (`tasks.yaml`)** |

**Mode Compose** — `pull` + `up -d` directement dispatchés à l'agent, sans
script à écrire :

| Champ | Rôle |
|---|---|
| Hôte cible + Projet compose | Requis (contrainte en base : les deux ou aucun) |
| Service | Optionnel — vide = tout le projet |
| Healthcheck (s) | `0`–`3600` |
| Rollback si échec / unhealthy | Revert automatique si le healthcheck échoue |
| Nettoyer les images orphelines | `docker image prune` ciblé après mise à jour |
| Pré/post-update task | Deux hooks `tasks.yaml` optionnels autour du déploiement |
| Réconcilier automatiquement en cas de dérive | Voir [§1.3](#13-réconciliation-de-dérive-mode-compose) |
| Identifiants de registre | Sélection d'un `RegistryCredential` sauvegardé, pour un registre privé |

**Mode Tâche** — comme un tracker Git avec dispatch : exécute une tâche
`tasks.yaml` de votre choix sur l'hôte cible.

**Import groupé** : `POST /api/v1/release-trackers/bulk` accepte jusqu'à
100 trackers Docker en un seul appel (utilisé par l'UI "détecter les
conteneurs suivables" — chaque entrée est validée indépendamment, le
résultat liste succès/erreurs par item).

### 1.3 Réconciliation de dérive (mode Compose)

Si `reconcile_drift` est activé, le poller ne se limite pas à "y a-t-il une
nouvelle image en amont ?" — quand le digest du registre est inchangé mais
que le digest **réellement déployé** (rapporté par l'agent) ne correspond
plus à ce que le tracker a enregistré (modification manuelle, échec
silencieux, rollback externe…), il redispatche le même `compose_pull` +
`up -d` pour ramener le déploiement en conformité. Désactivé par défaut ;
sans ce flag, une dérive est seulement signalée en lecture (`DriftDetected`,
recalculé à chaque affichage, jamais stocké).

### 1.4 Cadence de vérification

Un seul intervalle global pilote **tous** les trackers, Git comme Docker :
`GITHUB_POLL_INTERVAL` (défaut `15m`). Il n'y a pas de fréquence par
tracker — utilisez **Vérifier maintenant** sur un tracker pour forcer un
check immédiat sans attendre le cycle.

### 1.5 Notifications

Cases à cocher `smtp` / `ntfy` / `browser`, plus une option "Notifier à
chaque mise à jour détectée" (grisée tant qu'aucun canal n'est coché).

---

## 2. Webhooks Git

Dashboard → **Git / Automatisation** → onglet **Webhooks**.

### 2.1 Créer un webhook

| Champ | Valeur |
|---|---|
| Nom | Libre |
| Provider | `github` / `gitlab` / `gitea` / `forgejo` / `custom` |
| Événement | `push` (défaut) / `tag` (couvre aussi `create`/`tag_push`) / `release` |
| Filtre repo | Glob optionnel (ex : `myorg/*`) |
| Filtre branche | Glob optionnel (ex : `main`) |
| Tâche (`tasks.yaml`) | Requis — l'id de la tâche à exécuter côté agent |
| Notifications | En cas de succès / échec, canaux `smtp`/`ntfy`/`browser` |
| Activer ce webhook | Switch |

### 2.2 Récupérer l'URL et le secret

La page détail d'un webhook affiche :
- **URL** à coller côté GitHub/GitLab/Gitea/Forgejo : `<origine du dashboard>/api/v1/webhooks/git/<id>/receive`, avec bouton copier.
- **Secret** masqué (bouton révéler + copier). Généré par 32 octets aléatoires (hex) à la création.
- Un bloc d'aide spécifique au provider : GitLab attend le secret dans un header `X-Gitlab-Token`, les autres providers attendent `Content-Type: application/json` et le secret collé dans le champ "Secret" du webhook côté plateforme (signature HMAC, voir ci-dessous).

**Régénérer le secret** invalide l'ancien **immédiatement** — pensez à
mettre à jour la configuration côté plateforme Git dans la foulée, ou le
prochain push échouera la vérification de signature.

### 2.3 Vérification de signature par provider

| Provider | Mécanisme |
|---|---|
| GitLab | Header `X-Gitlab-Token`, comparé directement (temps constant) au secret — pas de HMAC, GitLab envoie le token brut |
| GitHub / Gitea / Forgejo / Custom | `X-Hub-Signature-256` (ou `X-Gitea-Signature` / `X-Forgejo-Signature`) au format `sha256=<hex>`, HMAC-SHA256 du corps brut avec le secret |

### 2.4 Variables d'environnement injectées

Disponibles dans la commande/le script `tasks.yaml` déclenché :

| Variable | Contenu |
|---|---|
| `SS_REPO_NAME` | Dépôt (`owner/repo`) |
| `SS_BRANCH` | Branche poussée |
| `SS_COMMIT_SHA` | SHA du dernier commit |
| `SS_COMMIT_MESSAGE` | Message du commit |
| `SS_PUSHER` | Auteur du push |
| `SS_WEBHOOK_NAME` | Nom du webhook ServerSupervisor |
| `SS_EVENT_TYPE` | Événement normalisé (`push`/`tag`/`release`) |

### 2.5 Filtres et anti-doublon

À la réception, dans l'ordre : vérification de signature → filtre
événement (ignoré seulement si non vide et différent de `push`) → filtre
repo (glob) → filtre branche (glob) → garde anti-doublon (si une exécution
est déjà `pending`/`running` pour ce webhook, la nouvelle requête est
ignorée) → dispatch de la tâche.

### 2.6 Limites

- Endpoint **public**, sans JWT — seule protection : la signature/le token
  ci-dessus, plus un rate limiter dédié **5 req/s (burst 10) par IP source**
  (respecte `TRUSTED_PROXIES` pour résoudre l'IP réelle derrière un reverse
  proxy).
- Taille max du payload accepté : **5 Mo**. Le payload brut est conservé
  (tronqué à 64 Ko) pour debug via le bouton **Voir le payload reçu** sur
  chaque exécution.
- Pas de liste blanche d'IP — ni pour ce endpoint, ni ailleurs dans l'app.

---

## 3. Historique d'exécution

Commun aux deux features (`WebhookExecutionList.vue`, paginé 20/page) :

- **Trackers** : date, release (tag + nom, lien vers la release), hôte,
  statut, un badge cliquable **⚠ N** quand N incidents d'alerte se sont
  déclenchés sur l'hôte cible dans les 15 minutes suivant le déploiement
  (renvoie vers `/alertes` → onglet Incidents), et l'accès aux logs de la
  commande.
- **Webhooks** : date, repo + branche, commit (SHA court + message tronqué),
  statut, logs, et le bouton **Voir le payload reçu** si disponible.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Tracker GitHub bloqué en erreur de rate limit | `GITHUB_TOKEN` non configuré (60 req/h en anonyme, partagé par tous les trackers GitHub) |
| Tracker GitLab/Gitea sur une instance self-hosted ne trouve jamais de release | Non supporté actuellement — les clients sont codés en dur sur `gitlab.com`/`gitea.io` (voir [§1.1](#11-tracker-git-github--gitlab--gitea)) |
| Tracker Docker en mode Compose échoue silencieusement | Vérifiez `Hôte cible`/`Projet compose` (les deux sont requis ensemble) et que le projet compose existe bien sur l'agent visé |
| Webhook reçu par la plateforme Git mais rien ne se passe côté ServerSupervisor | Vérifiez le filtre événement/repo/branche (glob), ou qu'une exécution n'est pas déjà `pending`/`running` pour ce webhook (garde anti-doublon) |
| Webhook renvoie une erreur de signature | Secret désynchronisé après un **Régénérer**, ou header de signature absent/mal formé côté plateforme (voir [§2.3](#23-vérification-de-signature-par-provider) pour le header attendu par provider) |
| Payload volumineux tronqué dans "Voir le payload reçu" | Normal — seuls les 64 premiers Ko du payload sont conservés, même si jusqu'à 5 Mo sont acceptés en entrée |
| `reconcile_drift` ne redéploie jamais malgré une dérive visible | Le flag doit être explicitement activé sur le tracker — par défaut la dérive est seulement affichée, jamais corrigée automatiquement |

## Pour aller plus loin

Voir aussi les sections [Runbooks & Tâches planifiées](runbooks-scheduled-tasks.md)
pour l'autre mécanisme de dispatch de commandes, et la section
[Git Webhooks & Suivi de releases](../README.md#git-webhooks--suivi-de-releases)
du README pour le tableau complet des routes API.
