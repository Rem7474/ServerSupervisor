# Audit d'architecture — ServerSupervisor (juillet 2026)

> Audit réalisé par lecture directe du code (pas de confiance a priori dans la documentation ni dans le brief d'audit initial). Chaque affirmation ci-dessous est sourcée par un chemin de fichier et, quand c'est pertinent, une ligne précise.

---

## 0. Executive Summary

**Constat préalable, et c'est le résultat le plus important de cet audit : le brief de départ décrit un état du projet qui n'existe plus.** Router monolithique de 22 Ko sans structure, absence de repository pattern, handlers qui mélangent SQL et logique métier, tests « quasi absents », frontend en JavaScript avec un seul store Pinia — tout ça a été vrai un jour, mais une revue systématique du code actuel (pas du `CLAUDE.md`, du code lui-même) montre qu'un vrai chantier de refactoring a déjà eu lieu : couche service + repository ports par domaine, handlers de quelques dizaines de lignes qui ne font que bind→service→respondError, 51 fichiers de tests côté serveur (7 162 lignes, dont de vrais tests d'intégration Postgres via testcontainers), frontend TypeScript à 100 % avec 4 stores Pinia et des composables. Sur ces points précis, le projet n'a pas besoin d'être réécrit : il a déjà été réécrit.

Ce qui ne veut pas dire que tout est propre. En creusant précisément pour vérifier les affirmations du brief et de `CLAUDE.md`, cet audit a mis à jour des problèmes réels, dont deux qui n'étaient mentionnés dans aucune des deux descriptions de départ :

1. **🔴 Fuite de secrets en clair** — `GET /api/v1/settings` n'a aucun contrôle de rôle et renvoie le mot de passe SMTP et le token GitHub en clair à n'importe quel utilisateur authentifié, viewer inclus (`server/internal/handlers/settings.go:25-27`).
2. **🔴 Corruption silencieuse de configuration** — une sauvegarde du formulaire Settings sans ressaisir le mot de passe SMTP écrase la valeur définie par variable d'environnement avec une chaîne vide, cassant l'envoi d'alertes sans aucune erreur visible (`server/internal/config/config.go:195-215`).
3. **🔴 Paniques goroutines non supervisées** — les pollers Proxmox/release-tracker, les fan-out WebSocket du dashboard et plusieurs autres goroutines n'ont aucun `recover()` : une panique dans l'une d'elles tue tout le processus, alors que le pattern de récupération existe déjà ailleurs dans le code (`internal/background/runner.go`) mais n'est pas généralisé.

À cela s'ajoutent des lacunes plus classiques mais réelles : aucune stratégie de sauvegarde pour la base (juste un volume Docker nommé), conteneur serveur qui tourne en root, rate limiter et fallback WebSocket par query string encore en place. Le détail complet, avec preuves, suit.

**Mise à jour :** les 5 points 🔴 de la roadmap (section 7) — les trois ci-dessus, plus le conteneur root et l'absence de sauvegarde — ont été corrigés sur cette branche (`internal/safego`, garde admin sur `/settings`, garde `OverrideFromDB`, `USER` non-root dans le Dockerfile, service `postgres-backup`). `go build`/`go vet`/`go test ./...` passent sur l'ensemble du module serveur. Détail exact de chaque correctif en fin de section 7.

---

## 1. Audit Architecture Globale

### Monorepo server/agent/frontend

Justifié pour la taille actuelle du projet (un mainteneur principal, `CODEOWNERS` mono-owner `@Rem7474`). Le risque classique d'un monorepo multi-langages — un changement serveur qui casse silencieusement le protocole agent — est explicitement traité : `protocol/agent_report.golden.json` est un fixture généré par réflexion côté agent (`agent/internal/sender/contract_test.go`) et rejoué côté serveur avec `DisallowUnknownFields()` (`server/internal/handlers/agent_contract_test.go`). C'est un vrai contract test, pas de la documentation. Le versionnement indépendant agent/serveur est également géré : l'agent envoie sa version dans chaque report, l'update est déclenché à la demande par host (pas de rolling update forcé), et le protocole JSON reste rétro-compatible tant que le contract test passe.

Le vrai problème du monorepo n'est pas le monorepo lui-même : ce sont les 3 pipelines CI séparés (`ci-server.yml`, `ci-agent.yml`, `ci-frontend.yml`) qui ne se déclenchent que sur les chemins concernés — cohérent et bien fait — mais qui n'ont **aucune étape de vérification croisée** au-delà du contract test JSON (pas de test end-to-end agent réel → serveur réel → frontend réel dans un même run CI).

### Modèle de communication agent → serveur (push HTTP toutes les 30s)

Confirmé : l'agent POST son rapport toutes les `report_interval` secondes (30 par défaut, configurable — `agent/internal/config/config.go:196`), et les commandes en attente arrivent **uniquement en réponse à ce POST** (`{commands: [...], skip_metrics}`). Deux consequences concrètes vérifiées dans le code :

- **Latence de dispatch de commande bornée par l'intervalle de report.** Une commande mise en queue juste après qu'un agent a envoyé son rapport attendra jusqu'au prochain tick (30s par défaut) avant d'être vue. Il n'existe aucun mécanisme de nudge/long-poll pour accélérer ça.
- **Le streaming de sortie de commande longue est lui aussi unidirectionnel** : chaque lecture de 4096 octets sur stdout/stderr devient un POST HTTP séparé vers `/api/agent/command/stream` (`agent/internal/dispatcher/handler_custom.go:82-97` → `agent/internal/sender/sender.go:209-244`), dont les erreurs sont avalées (best-effort, ne fait jamais échouer la commande). Le WebSocket ne sert que sur l'autre versant (serveur → navigateur), pas côté agent.

**Pourquoi ce n'est pas absurde à l'échelle actuelle, et ce que je changerais.** Pour un outil visant des dizaines à quelques centaines d'hôtes (le public réel de ce projet : équipes qui supervisent leur propre infra, pas un SaaS multi-tenant à 100 000 agents), ce modèle HTTP simple a un vrai mérite : zéro dépendance supplémentaire, passe par n'importe quel reverse proxy déjà en place, un seul format de message (JSON) partout. Les trois alternatives suggérées dans le brief ont chacune un coût réel qu'il faut regarder en face :

| Option | Ce qu'elle apporte | Ce qu'elle coûte ici |
|---|---|---|
| **SSE** | Push serveur→client sans polling | Unidirectionnel : il faudrait quand même garder du HTTP POST pour les rapports agent→serveur. Deux canaux au lieu d'un, pour un gain marginal sur la seule latence de commande. |
| **MQTT** | Fan-out pub/sub, adapté à des flottes IoT peu fiables/déconnectées | Ajoute un broker à faire tourner, sécuriser et superviser (Mosquitto/EMQX), un nouveau protocole à exposer au firewall, une sémantique QoS/retained-message à apprendre — disproportionné pour un modèle 1 agent ↔ 1 serveur. |
| **gRPC bidirectionnel** | Streaming natif, contrats typés via protobuf, HTTP/2 | Impose un pipeline protobuf (codegen des deux côtés) et surtout un reverse proxy HTTP/2-aware **chez chaque utilisateur self-hosted** — nginx/Caddy/Traefik le gèrent, mais pas sans configuration explicite, et certains LB d'entreprise étranglent encore le HTTP/2+gRPC. |

Ma recommandation concrète : **réutiliser le WebSocket qui existe déjà côté navigateur, mais côté agent.** Une connexion persistante par agent, authentifiée comme le reste (API key), JSON (aucun nouvel outillage), qui remplace à la fois l'attente du prochain report pour le dispatch de commande et le POST-par-chunk pour le streaming de sortie. Pas de broker, pas de protobuf, juste la généralisation d'un pattern déjà validé en production dans ce même repo (`server/internal/ws/`).

### TimescaleDB

**Bon choix, et déjà exploité correctement — pas du sur-engineering.** Vérifié dans les migrations : compression + retention policies configurées par hypertable (`system_metrics` : compress @7j / retain 30j ; `disk_health` : retain 90j ; `login_events` : retain 90j — `server/internal/database/migrations/064_v2_timescale_migrate.sql:60-115`), continuous aggregates 5 min/1h avec real-time aggregation forcée au boot (`server/internal/database/db.go:104-227`), et un override runtime de la rétention via `METRICS_RETENTION_DAYS` branché sur `settings`. C'est le niveau de maturité TimescaleDB qu'on voit rarement dans un projet de cette taille.

L'argument décisif pour Timescale ici n'est même pas la performance : c'est qu'il s'agit **toujours de Postgres**. Les données relationnelles du produit (hosts, users, RBAC, alert rules, audit logs) et les séries temporelles (métriques CPU/RAM/disque) cohabitent dans **une seule base, un seul pool de connexions, une seule stratégie de backup, un seul driver**. Passer à VictoriaMetrics/InfluxDB/ClickHouse pour les métriques doublerait la surface opérationnelle (deux bases à sauvegarder, deux systèmes à superviser, une couche de jointure applicative pour croiser "métriques d'un hôte" et "l'hôte appartient à quel user/groupe") pour un gain de compression/cardinalité dont ce projet n'a pas besoin à son échelle actuelle. Ça deviendrait pertinent seulement à une échelle SaaS multi-tenant avec des millions de séries actives — pas le cas ici.

Le vrai trou n'est pas le choix du moteur, c'est l'absence totale de sauvegarde (voir section DevOps).

### Vue.js + Tabler CSS

Toujours d'actualité en 2025-2026 comme choix pragmatique pour un dashboard d'administration : Tabler fournit ~200 composants prêts à l'emploi (tables, formulaires, modales, badges de statut) qui correspondent exactement au vocabulaire visuel d'un outil de supervision. Le coût réel, vérifié dans le code : import CSS global non tree-shaké (`frontend/src/main.ts:5`, `@tabler/core/dist/css/tabler.min.css`), aucune surcouche Sass (zéro fichier `.scss` dans le repo, personnalisation via 424 lignes de CSS brut qui s'appliquent après le CSS minifié de Tabler), et aucun contrôle de taille de bundle en CI. Si je repartais de zéro aujourd'hui je regarderais sérieusement **Tailwind + un kit headless accessible par défaut (shadcn/vue, Reka UI)** : bundle plus fin, a11y native via les primitives Radix sous-jacentes, personnalisation totale — au prix d'un temps de développement initial plus long puisqu'il faut construire chaque composant au lieu de le prendre sur l'étagère. Pour ce produit précis (beaucoup d'écrans CRUD/tableaux, équipe réduite), ce n'est pas un verdict tranché — c'est un vrai compromis vitesse-de-livraison vs finesse, et Tabler n'est pas une erreur de débutant, c'est un choix défendable.

### Cohabitation API + WS + scheduler + alert engine + poller Proxmox dans un seul binaire

Le code sépare déjà bien les responsabilités **à l'intérieur** du binaire : `internal/poller.Every()` (boucle générique), `internal/background.Runner` (jobs supervisés avec `recover()`), `internal/events.Bus` (pub/sub découplant écriture et notification WS), `internal/scheduler.TaskScheduler` (cron). Le couplage réel n'est pas architectural, il est dans la **supervision des goroutines** : `background.Runner` récupère les paniques de ses 7 jobs, mais `poller.Every` (utilisé par les pollers Proxmox/release-tracker/NPM) et une bonne dizaine de `go func()` ad hoc n'ont aucun `recover()` — détail complet en section Backend. Je ne recommande **pas** de découper ça en microservices : pour un outil self-hosted mono-tenant, un seul binaire à déployer reste un vrai avantage opérationnel (`docker compose up`, un seul `Dockerfile`, un seul log stream) ; le problème à résoudre est l'isolation des paniques, pas la topologie de déploiement.

---

## 2. Audit du Backend Go

### `router.go` (22 Ko) et l'absence de repository pattern — les deux prémisses du brief sont fausses aujourd'hui

**Taille confirmée (493 lignes / ~22,98 Ko) mais le contenu réfute le mot « monolithique ».** Le fichier est 24 fonctions d'enregistrement de routes plates + `SetupRouter` qui fait de l'injection de dépendances :

```go
// server/internal/api/router.go:44
func SetupRouter(db *database.DB, cfg *config.Config, notifHub *ws.NotificationHub,
    bus *events.Bus, sched *scheduler.TaskScheduler, dispatcher *dispatch.Dispatcher,
) (*gin.Engine, *handlers.ReleaseTrackerHandler, *handlers.ProxmoxHandler, *handlers.NPMHandler, func()) {
```

```go
// router.go:206-211 — représentatif de tout le fichier
func registerHostRoutes(g *gin.RouterGroup, h *handlers.HostHandler, agentH *handlers.AgentHandler, db *database.DB) {
	g.GET("/hosts", h.ListHosts)
	g.POST("/hosts", h.RegisterHost)
	g.GET("/metrics/summary", agentH.GetMetricsSummary)
```

Les 22 Ko viennent de la **largeur** (24 domaines HTTP, dont un très gros périmètre Proxmox), pas de logique embarquée. Seule exception : un mini-handler `/api/health` inline de 5 lignes (`router.go:150-156`) qui appelle `db.Ping()` directement — cosmétique, pas un problème.

**Repository pattern : présent, via des ports définis par le consommateur.** Chaque domaine de `internal/services/<domaine>` définit sa propre interface `Repository`, satisfaite structurellement par `*database.DB` (typage canard Go, aucun `implements` explicite) :

```go
// internal/services/host/service.go:24-41
type Repository interface {
	RegisterHost(ctx context.Context, host *models.Host) error
	GetAllHosts(ctx context.Context) ([]models.Host, error)
	GetHost(ctx context.Context, id string) (*models.Host, error)
	// ...
}
```

Vérifié par grep : zéro appel direct à une méthode `*database.DB` pour de la logique métier dans `internal/handlers/*.go`. Les handlers qui gardent un champ `db` (`apt.go`, `docker.go`, `system.go`…) ne l'utilisent que pour `requireHostAccess(c, h.db, hostID, level)` (`internal/handlers/host_authz.go:12`), le garde d'autorisation HTTP qui a besoin du `*gin.Context` — exactement comme documenté.

**Handlers vérifiés (hosts, docker, alert_rules) : minces, bind→service→respondError.**

```go
// internal/handlers/hosts.go:65-82
func (h *HostHandler) UpdateHost(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	var req models.HostUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apperr.Validation(err.Error()))
		return
	}
	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), req)
	// ...
```

`internal/handlers/` totalise 4 184 lignes hors tests sur ~38 fichiers (≈110 lignes/fichier) contre 7 065 lignes dans `internal/services/` — la logique est bien dans la couche service.

**Verdict global sur cette partie : rien à réécrire.** Le vrai travail (repository ports, handlers minces, erreurs typées) est fait. J'aurais éventuellement renommé `internal/handlers` → `internal/transport/http` et `internal/database` → `internal/repository/postgres` pour rendre la couche « transport » et « c'est du Postgres, en théorie interchangeable » explicite dans l'arborescence — mais c'est du confort de lecture, pas un gain fonctionnel : les ports existent déjà, donc la testabilité est déjà là.

### Sécurité — deux vraies vulnérabilités trouvées, plusieurs non-problèmes écartés

**🔴 Fuite de secrets en clair via `GET /api/v1/settings`.** Toutes les autres méthodes de `internal/handlers/settings.go` vérifient le rôle admin en dur (`UpdateSettings`, `TestSmtp`, `TestNtfy`, `CleanupMetrics`, `CleanupAuditLogs`) — sauf `GetSettings` (`settings.go:25-27`), et la route n'a pas non plus de middleware admin (`router.go:314-321`). Le `Snapshot()` qu'elle appelle renvoie tout en clair :

```go
// internal/services/settings/service.go:60-68
"smtpUser":    c.SMTPUser,
"smtpPass":    c.SMTPPass,
// ...
"githubToken": c.GitHubToken,
```

**Conséquence directe : n'importe quel compte authentifié, y compris un rôle « viewer », peut lire le mot de passe SMTP et le token GitHub du serveur.** Correctif (petit, ciblé, pas de refactoring) :

```go
// settings.go — ajouter le garde manquant, comme les autres méthodes du fichier
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	if c.GetString("role") != models.RoleAdmin {
		respondError(c, apperr.Forbidden("insufficient permissions"))
		return
	}
	// ...
}
```
Et côté `Snapshot()`, ne jamais renvoyer les secrets bruts au frontend — exposer un booléen `smtpPassSet: c.SMTPPass != ""` plutôt que la valeur, comme le fait déjà probablement l'UI pour afficher un champ « mot de passe défini » sans le préremplir.

**🔴 `OverrideFromDB` écrase silencieusement des secrets valides.** Quatre champs n'ont pas le garde `v != ""` que les autres ont :

```go
// internal/config/config.go:195-215 — smtp_host/smtp_from/retention ont un garde `&& v != ""`, ces 4-là non :
if v, ok := settings["smtp_user"]; ok { c.SMTPUser = v }
if v, ok := settings["smtp_pass"]; ok { c.SMTPPass = v }
// ... ntfy_url, github_token idem
```

Et la sauvegarde du formulaire Settings écrit ces champs **sans condition** (`internal/services/settings/service.go:97-102`). Résultat vérifié : un admin qui sauvegarde la page Settings sans ressaisir le mot de passe SMTP (pattern UX très courant pour un champ secret) efface silencieusement la valeur configurée par variable d'environnement, **immédiatement**, cassant l'envoi d'alertes sans aucune erreur. Le test existant (`internal/config/settings_override_test.go`) couvre le garde pour `smtp_host` mais pas pour ces 4 champs — le trou de test correspond exactement au trou de code. Correctif :

```go
if v, ok := settings["smtp_pass"]; ok && v != "" { c.SMTPPass = v }
```
à répliquer sur les 4 champs, plus un test qui couvre explicitement le cas « champ vide soumis n'écrase pas une valeur env existante ».

**Non-problèmes, écartés après vérification :**

- **API keys agent** : hashées bcrypt (`bcrypt.DefaultCost`), et la recherche n'est **pas** un scan complet — la clé embarque l'ID d'hôte en clair (`"{hostID}.{secret}"`), donc c'est un `SELECT` indexé sur PK + une seule comparaison bcrypt. Un hash bidon est comparé même quand l'hôte n'existe pas (`db_hosts.go:66-90`) pour éviter un oracle de timing sur l'existence d'un host — c'est du travail sérieux, pas une lacune.
- **Blocage automatique d'IP** : contrairement à ce que suggère le brief, ce n'est **pas** purement en mémoire — les échecs de connexion sont des lignes en base (`login_events`), le déblocage manuel est persisté (`ip_block_overrides`), et la map en mémoire ne sert que de repli en cas d'échec de la requête DB elle-même (`internal/services/authn/service.go:84-92`). Ça survit à un restart et se partage naturellement si plusieurs instances tapent sur le même Postgres.
- **Variables d'environnement par défaut dangereuses** (`JWT_SECRET=change-me-in-production-please`, `ADMIN_PASSWORD=admin`, `DB_PASSWORD=supervisor`) : elles existent bien dans `docker-compose.yml` comme filet de repli, mais `ValidateStrict()` (`config.go:262-283`), appelée au boot (`cmd/server/main.go:47-49`), **fait planter le process au démarrage** si l'une de ces valeurs par défaut est détectée hors `APP_ENV=dev`. C'est un vrai fail-closed, pas une négligence — la seule limite réelle est que c'est une liste de valeurs interdites exactes, pas un contrôle d'entropie : un mot de passe faible mais différent de `admin` passerait le contrôle.

**Ce qui reste un vrai sujet, sans être une faille active :**

- **Rate limiter 100 % en mémoire** (`map[string]*rate.Limiter` protégée par mutex, `middleware.go:23-102`). Aucun partage d'état entre instances — un vrai problème *si* vous scalez le serveur horizontalement, non-problème sinon. Le déploiement actuel (docker-compose, un seul conteneur serveur) ne scale pas horizontalement, donc ce n'est pas urgent — mais c'est un blocker day-one si ça change.
- **Auth WebSocket via `?token=`** : toujours présente en repli, à deux endroits (`middleware.go:294-316` pré-upgrade, `internal/ws/base.go:200-212` post-upgrade), après une tentative de cookie et avant un message d'auth in-band JSON. Les logs masquent bien le paramètre (`maskSensitiveParams`, `middleware.go:250-269`), mais le token peut toujours atterrir dans l'historique du navigateur ou les logs d'un proxy intermédiaire. Le cookie est déjà le chemin principal ; je supprimerais purement le fallback query-string.
- **JWT 24h / refresh 168h** : raisonnable pour un outil self-hosted (MFA/TOTP + RBAC 3 niveaux + bcrypt déjà en place). Point non vérifié dans cet audit et à contrôler séparément : rotation/détection de réutilisation du refresh token — aucun des quatre passages de revue n'a examiné spécifiquement ce mécanisme, donc je ne l'affirme ni ne l'infirme ici.

### Migrations SQL

**9 fichiers aujourd'hui, pas 27** : `000_full_baseline_breaking.sql` (squash des migrations 001→063) + `064_v2_timescale_migrate.sql` jusqu'à `071_remote_commands_activity.sql`. Le chiffre « 027 » du brief est un vieux snapshot — `027_proxmox.sql` existe encore, mais seulement comme entrée dans l'`INSERT INTO schema_migrations` de la baseline, plus comme fichier sur disque.

**Runner maison confirmé, aucune dépendance golang-migrate/goose/Atlas** (`go.mod` grep négatif). Le mécanisme (`server/internal/database/db.go:254-365`) : `//go:embed migrations/*.sql`, table `schema_migrations`, un splitter de statements SQL maison, exécution en ordre alphabétique des fichiers non appliqués. **Forward-only : aucune fonction `Down()`, aucun mécanisme de rollback de schéma.** Une petite incohérence interne relevée en creusant : `migrations_baseline.go` sait parser des marqueurs `-- ===== BEGIN <file>.sql =====` pour gérer une future re-baseline, mais la baseline actuelle ne contient aucun de ces marqueurs — elle marque les 63 anciens fichiers appliqués via un simple `INSERT` SQL classique. Le mécanisme de parsing est donc du code mort aujourd'hui (sans danger, juste un léger décalage doc/implémentation).

Pour un projet de cette taille, le runner maison n'est pas déraisonnable — mais l'absence de rollback devient un vrai risque combinée à l'absence de backup (voir DevOps). Je recommande **goose** (le plus proche en philosophie du mécanisme actuel, migration progressive possible) plutôt qu'Atlas (plus lourd, orienté schema-as-code déclaratif) si vous voulez un outil dédié — ou, a minima, documenter et tester une procédure de rollback manuel par restauration de snapshot avant chaque migration en prod.

### Tests — l'affirmation « quasi absents » est fausse

**51 fichiers de tests, 7 162 lignes, ≈18 % du LOC Go total.** Les plus gros : `services/agent/service_test.go` (341 lignes), `alerts/severity_test.go` (283), deux fichiers d'intégration handlers (`hosts_users_crud_integration_test.go` 232, `auth_integration_test.go` 226). Les tests d'intégration utilisent **un vrai Postgres éphémère via testcontainers-go** (`internal/testutil/postgres.go`), pas des mocks — un cran au-dessus de ce qu'on voit généralement. CI (`ci-server.yml:70-80`) lance `go test -v -race -coverprofile=coverage.out ./...` et upload vers Codecov, mais **sans seuil bloquant** (`continue-on-error: true`).

**Packages avec zéro test**, à corriger en priorité pour les plus sensibles : `internal/auth` (génération/vérification TOTP — sensible sécurité, à traiter vite), `internal/ws` (hubs, snapshot builders), `internal/scheduler`, `internal/poller`, `internal/dispatch`. La majorité des `db_*.go` n'ont pas de test direct mais sont exercés indirectement via les tests d'intégration handlers/services.

### Gestion d'erreurs

Style mixte mais cohérent, pas de la négligence : erreurs typées `apperr.*` à la frontière service (`apperr.NotFound(...)`), `errors.Is(err, sql.ErrNoRows)` pour les vérifications d'existence, `%w` utilisé pour les appels à des dépendances externes (`fmt.Errorf("retrieve npm secret: %w", err)`, `services/npm/service.go:231-237`), et beaucoup de simples `return nil, err`. Ce dernier point est moins un problème qu'il ne semble : **tout finit par passer par `respondError → apperr.From(err) → Internal(err){wrapped: err}`**, qui préserve la cause originale via `Unwrap` même quand le service n'a rien enrichi lui-même. Le filet de sécurité unique à la frontière HTTP rend le manque de `%w` en profondeur moins coûteux qu'il ne le serait sans lui.

### 🔴 Supervision des goroutines — recover() existe mais n'est pas généralisé

`internal/background/runner.go:59-70` récupère bien les paniques des 7 jobs enregistrés via `bg.Add(...)` (nettoyage audit, monitoring host status, éval alertes, rétention métriques, rétention web-logs, worker uptime, worker SSL) :

```go
func (r *Runner) run(ctx context.Context, job Job) {
	defer r.wg.Done()
	defer func() {
		if rec := recover(); rec != nil {
			slog.ErrorContext(ctx, "background job panicked",
				slog.String("job", job.Name), slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())))
		}
	}()
	job.Run(ctx)
}
```

Mais ce pattern **n'est pas appliqué** à `internal/poller.Every()` (utilisé par les pollers Proxmox, release-tracker et sync NPM — `cmd/server/main.go:138,140,142`), ni aux fan-out internes à fort trafic : les 8 goroutines de construction du snapshot dashboard (`internal/ws/snapshots.go:68-109`, sur le hot-path de chaque connexion WS dashboard), les 6 goroutines de `Complete()` côté host detail (`internal/services/host/service.go:233-238`), et une dizaine d'autres `go func()` isolés (`ws/version_compare.go`, `services/proxmox/service.go`, `services/releasetracker/service.go`, `services/audit/service.go`, `handlers/npm.go`, jusqu'au ticker de nettoyage du rate limiter lui-même dans `middleware.go:45`).

**Conséquence concrète : une panique (nil pointer dans un scan DB, par exemple) dans n'importe laquelle de ces goroutines non protégées fait crasher tout le processus** — Go ne permet pas à une goroutine sœur de récupérer la panique d'une autre. Correctif ciblé, pas un refactoring : extraire le `defer recover()` de `runner.go` dans un helper partagé et l'appliquer systématiquement.

```go
// internal/background/safego.go (nouveau, ~15 lignes)
func Go(ctx context.Context, name string, fn func()) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(ctx, "goroutine panicked",
					slog.String("name", name), slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())))
			}
		}()
		fn()
	}()
}
```
À appeler depuis `poller.Every` et depuis chaque fan-out listé ci-dessus, en remplacement de `go func(){...}()`.

---

## 3. Audit de l'Agent Go

### Self-update (`--internal-update`)

**Mécanisme nettement plus soigné que ce que le brief laisse supposer.** Déclenché manuellement par host (pas de rolling update automatique de la flotte) via `TriggerAgentUpdate` (`server/internal/services/host/service.go:161-202`), le flux réel :

1. Le dispatcher agent lance un processus **détaché** via `systemd-run` (`agent/cmd/agent/update.go:33-69`) — ce processus survit au `systemctl restart` qu'il va lui-même déclencher plus loin.
2. Téléchargement du binaire + d'un sidecar `.sha256` depuis la même release GitHub, vérification par `sha256.Sum256` (`update.go:96-123`).
3. Remplacement atomique par `os.Rename` sur le même filesystem, avec l'ancien binaire conservé en `.bak` (`update.go:125-168`).
4. **Test de fumée** : exécution du nouveau binaire avec `--version` et comparaison à la version attendue avant de couper (`update.go:170-182`).
5. `systemctl restart` puis poll `systemctl is-active --quiet` pendant 30s (`update.go:184-199`).
6. Rollback automatique vers `.bak` à **chaque** étape d'échec possible après le remplacement de fichier.

C'est un vrai travail d'ingénierie : vérification d'intégrité, rename atomique, test de fumée pré-bascule, health check post-restart, rollback à chaque étape. Trois limites réelles :

- **Vérification d'intégrité, pas d'authenticité.** Le binaire et son `.sha256` viennent de la même release — ça protège contre une corruption réseau, pas contre une chaîne de release compromise (pas de signature GPG/cosign/Sigstore).
- **Health check superficiel** : `systemctl is-active` prouve que le process n'a pas immédiatement crashé, `--version` prouve que le parsing de flags fonctionne — ni l'un ni l'autre ne prouve qu'un cycle de collecte réel fonctionne. Un binaire qui passe les deux tests mais panique à son premier report réel serait redémarré en boucle par `Restart=always` (`agent/install.sh:126-138`) **sur ce même binaire cassé**, sans fenêtre de surveillance qui déclencherait un rollback automatique.
- **Dépendance dure à systemd**, avec garde explicite (`exec.LookPath("systemd-run")`) mais aucun chemin de repli pour un init non-systemd.

### Boucle de report — un vrai risque de blocage silencieux, non documenté ailleurs

Intervalle configurable (30s par défaut), collecte parallèle via `sync.WaitGroup` (jusqu'à 6 goroutines conditionnelles), retry avec backoff sur l'envoi (5s puis 15s, 3 tentatives, `sender.go:150-174`) puis abandon silencieux du rapport si tout échoue — logique et sans risque de double-exécution (`ReportCommandResult` n'est volontairement jamais retried).

**Ce qui n'est protégé nulle part : la phase de collecte elle-même n'a aucun timeout global.** `agent/internal/collector/disk.go` lance `df`/`smartctl` via `exec.Command` **sans** `CommandContext` (lignes 81, 87, 93, 99, 118, 486) — contrairement à `docker.go` (contextes 10s/30s) ou `crowdsec.go` (client HTTP à 5s). Comme `Send()` tourne de façon synchrone dans la boucle principale à goroutine unique (`main.go:178-187`) et qu'un `time.Ticker` **abandonne** les ticks pendant qu'il est occupé (il ne les met pas en file), **un `df` bloqué sur un mount NFS/CIFS mort arrêterait silencieusement et définitivement tout reporting, sans watchdog.** Ce n'est pas hypothétique : c'est un mode de panne connu et classique pour ce genre d'agent. Correctif ciblé :

```go
// disk.go — remplacer chaque exec.Command par un CommandContext borné
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
out, err := exec.CommandContext(ctx, "df", "-h").Output()
```

### Dispatcher de commandes

Confirmé : mutex dédié pour `apt` (dpkg ne supporte pas l'exécution concurrente), sémaphore à 4 slots pour les autres modules, timeout absolu de 45 minutes (`dispatcher.go:23,53,64-93`). 9 modules enregistrés (`docker, journal, apt, agent, systemd, processes, custom, crowdsec, compose`) — `compose` manque dans la description de `CLAUDE.md`, dérive documentaire mineure sans impact fonctionnel.

### `tasks.yaml` — bonne idée de sécurité, zéro mécanisme de distribution

Validation solide (regex d'ID, argv non-shell via `exec.CommandContext(taskCtx, task.Command[0], task.Command[1:]...)`, timeout par tâche 0-3600s). Mais **rien** ne synchronise ce fichier entre hôtes : `install.sh` ne le provisionne jamais, il n'existe aucun endpoint serveur d'écriture (seulement des `GET` pour affichage), aucune intégration Ansible/Salt. À l'échelle d'une flotte de dizaines d'hôtes, c'est une vraie charge opérationnelle manuelle — à combler soit par un endpoint de push de config côté serveur, soit par de la documentation d'intégration avec un outil de config management existant.

### Curseur des web logs

Conception délibérément *fail-open*, qui atteint son objectif : rotation, troncature, crash en cours de lecture, fichier curseur supprimé ou corrompu — aucun de ces cas ne bloque ou ne crashe l'agent, tous dégradent vers « retraiter et éventuellement compter en double », jamais vers « s'arrêter ». Détection de rotation par **taille uniquement** (`web_logs.go:717-719`, pas de vérification d'inode/mtime), écriture non atomique du curseur (`os.WriteFile` direct, pas de temp+rename+fsync). Le coût réel : un crash en cours de scan, un curseur supprimé/corrompu, ou une rotation inhabituellement rapide font retraiter des requêtes déjà vues, gonflant temporairement les compteurs de trafic/menaces — et **il n'existe aucune déduplication côté serveur** pour absorber ça à l'ingestion. Zéro test dédié sur cette logique de curseur. Correctif raisonnable : écriture temp+rename (+fsync), et vérification combinée taille+mtime pour la détection de rotation — pas une réécriture, un durcissement ciblé.

### Multi-architecture : `build.sh` a dérivé du pipeline de release réel

**`build.sh` ne construit que 3 plateformes** (`linux/amd64`, `linux/arm64`, `linux/arm`, sans distinction `GOARM` — `build.sh:15-30`), alors que **`.github/workflows/release.yml:101-140` construit bien les 4 cibles annoncées**, avec `GOARM: "7"` et `GOARM: "6"` explicites pour distinguer armv7/armv6. C'est le pipeline de release qui a raison, pas le script de dev local — mais la divergence entre les deux est une vraie source de confusion (« pourquoi mon build local n'a que 3 fichiers ? »). Sur l'usage de `buildx`/QEMU suggéré par le brief : **inutile ici**. `buildx`+QEMU a un vrai coût (émulation lente) et un vrai bénéfice pour des images Docker multi-arch (déjà utilisé à raison pour l'image serveur, `release.yml`) — mais pour des binaires Go statiques (`CGO_ENABLED=0`), la cross-compilation native (`GOOS`/`GOARCH`/`GOARM`) est plus simple et plus rapide que l'émulation, exactement ce que `release.yml` fait déjà pour l'agent. Le vrai gain serait d'aligner `build.sh` sur la matrice de `release.yml`, pas d'ajouter buildx.

**CI ne teste jamais réellement les architectures non-amd64** : `ci-agent.yml` ne construit/teste que sur `ubuntu-latest` (host amd64) ; le job `agent-binaries` de `release.yml` ne fait que `go build` par entrée de matrice, sans `go test`, et sans étape QEMU (QEMU n'est câblé que pour le job Docker serveur). arm64/armv7/armv6 sont donc vérifiés à la compilation seulement, jamais exécutés en CI.

### Tests

5 fichiers, ~389 lignes, contre ~34 fichiers non-test — clairement le point le plus faible du module agent. Bien wiré en CI (`ci-agent.yml:61`, `go test -v -race -coverprofile=coverage.out ./...`, upload Codecov), mais couvre surtout du parsing pur (`parsers_test.go`, `compose_update_test.go`). **Zéro test** sur le flux de self-update, la boucle de report, la logique de concurrence du dispatcher elle-même, et la logique de curseur des web logs — les quatre zones les plus délicates du module, identifiées ci-dessus.

---

## 4. Audit du Frontend Vue.js

### `api/` — la centralisation existe, mais n'est pas systématiquement traversée par des composables

Confirmé : `frontend/src/api/client.ts` porte l'instance axios partagée + les deux intercepteurs (CSRF double-submit, redirection dure sur 401), `index.ts` n'est plus qu'un barrel qui agrège 16 modules par domaine (`hosts.ts`, `docker.ts`, `proxmox.ts`, …) avec, en commentaire, une consigne explicite d'ajouter les nouveaux endpoints dans le module de domaine, pas dans `index.ts`.

**Mais 25 des 28 vues (89 %) importent quand même un module `api` directement dans leur propre fichier.** Seules 3 vues (`AlertsView`, `DashboardView`, `HostDetailView`) délèguent entièrement à un composable (`useAlertsPage`, `useDashboard`, `useHostDetail`) qui possède, lui, tout l'accès aux données. Le pattern dominant observé (ex. `DockerView.vue`) : la lecture initiale vient d'un snapshot WebSocket poussé dans des refs locales, mais chaque **mutation** (envoyer une commande Docker, etc.) appelle `apiClient.xxx()` directement depuis la vue, avec gestion optimiste de l'état inline. C'est moins grave que « aucune séparation » — les lectures passent déjà par un canal propre (WS) — mais ça reste un vrai coût de testabilité et de cohérence : si demain vous changez de transport pour les mutations, il faut toucher 25 fichiers de vue au lieu d'un composable.

### WebSocket dans les vues

Le composable `useWebSocket.ts` est solide : cleanup garanti par `onUnmounted` → `disconnect()`, backoff exponentiel plafonné à 30s avec distinction des codes non-retryables (1002/1008/4001), et un ré-attachement forcé sur reprise d'app (`ss:app-resume`). Ce n'est pas le composant à refaire.

**Un vrai doublon trouvé, pas une fuite mais un vrai gaspillage** : `NotificationBell.vue`, montée sur *toutes* les pages authentifiées via `App.vue:302`, garde en permanence sa propre connexion à `/api/v1/ws/notifications`. `AlertsView.vue:244` ouvre **une deuxième connexion indépendante vers la même route** au lieu de réutiliser celle de la navbar. Visiter `/alerts` ouvre donc deux sockets vivants sur le même endpoint, chacun traitant les mêmes pushes serveur en double. Les deux se ferment proprement au démontage — ce n'est donc pas une fuite mémoire, juste du travail dupliqué à corriger en partageant la connexion via un composable/store singleton plutôt qu'en laissant chaque composant ouvrir la sienne.

### Stores Pinia — 4 stores, pas 1, mais une vraie source de vérité double

Le brief suppose un unique store auth. Faux aujourd'hui : `stores/auth.ts` (identité uniquement), `stores/hosts.ts` (liste globale, TTL 60s, consommée par 5 endroits différents : navbar, bot, tâches planifiées, trafic, alertes), `stores/dashboard.ts` (setters uniquement, alimenté par le WS dashboard), `stores/alertRules.ts` (TTL 30s).

**Problème concret trouvé, pas théorique** : `hosts.ts` et `dashboard.ts` maintiennent chacun, indépendamment, une notion de « liste des hosts + statut en ligne », alimentées par deux pipelines jamais réconciliés — l'un un poll REST à TTL 60s, l'autre un push WS live. Sur la page Dashboard elle-même, le badge « hosts down » de la navbar (`App.vue:443-447`) lit `hostsStore.hosts` (jusqu'à 60s de retard), tandis que les KPI du corps de page lisent `dashboardStore.hosts` (live via WS) — **les deux chiffres peuvent légitimement se contredire sur le même écran.** Correctif : une seule source de vérité pour l'état « hosts up/down », le store WS-driven étant la plus fraîche, avec le store REST réservé aux données que le WS ne pousse pas (ex. formulaires d'édition).

### Router — lazy loading complet et gestion d'erreur de chunk soignée

Confirmé : 28 routes, toutes en `() => import(...)`. La logique de retry sur `ChunkLoadError` (`router/index.ts:197-274`) est nettement au-dessus de la moyenne : purge du cache du service worker + Cache Storage au premier échec de la session, hard-reload, puis abandon propre (événement `ss:fatal-error`) si ça se reproduit — pas de boucle infinie de reload. Rien à changer ici.

### Tabler CSS et TypeScript

Tabler confirmé en import global sans pipeline Sass (voir section Architecture Globale pour l'analyse coût/bénéfice). **TypeScript à 100 % confirmé littéralement** : `grep -rL '<script setup lang="ts">' frontend/src --include=*.vue` ne retourne aucun fichier. `types/generated.ts` (1 806 lignes, généré par tygo depuis les modèles Go) a bien des consommateurs réels vérifiés (21 sites d'import), pas du code mort.

### Tests

9 fichiers, 1 122 lignes — petit volume, mais du travail de qualité réelle : un test qui importe le **fichier source de production brut** du service worker via `?raw` pour tester sa logique de cache directement, avec une régression nommée (`'regression (#199): masks a real network failure...'`), et un vrai test navigateur Playwright qui vérifie le rendu Chart.js/D3 (dimensions de `<canvas>`, présence de `<path class="country">` du world map) — exactement ce que happy-dom ne peut pas vérifier.

**Écart trouvé avec `CLAUDE.md` : les tests navigateur ne tournent pas en CI.** `CLAUDE.md` affirme que la CI installe Playwright avant `test:browser` ; en réalité, `ci-frontend.yml` se termine par un commentaire explicite expliquant que la suite navigateur est **volontairement exclue** de la CI à cause d'un bug amont Vite 8 (rolldown) + Vitest 4 en mode navigateur, avec instruction de la lancer en local. C'est la seule affirmation de `CLAUDE.md` qui décrit un état souhaité plutôt que l'état réel — à corriger dans la doc, ou à rouvrir dès que le bug amont est résolu. Autre trou : aucun des 4 stores Pinia n'a de test dédié.

### Accessibilité et i18n

**i18n : totalement absent**, pas juste « seulement en français » — zéro occurrence de `i18n`/`vue-i18n` dans tout le repo, `<html lang="fr">` figé, chaînes hardcodées partout (`client.ts:79`, `'Une erreur est survenue'`). Si une commercialisation ou une internationalisation est un jour envisagée, c'est un chantier à part entière (extraction de chaînes + `vue-i18n`), pas un ajustement mineur.

**A11y : réelle mais concentrée, pas systématique.** Un vrai composable `useModalFocusTrap.ts` (piège de focus + restauration) utilisé par 4 composants partagés, des `aria-live`/`role="alert"` corrects sur les toasts et barres de statut — mais 57 % des vues (16/28) n'ont **aucun** attribut `aria-*`/`role` propre, ne s'appuyant que sur la sémantique native de Tabler/Bootstrap. Aucun outillage a11y automatisé (pas d'`eslint-plugin-vuejs-accessibility`, pas d'axe-core). Puisque Playwright est déjà en place pour les tests navigateur, ajouter `@axe-core/playwright` sur les pages existantes serait peu coûteux si l'a11y devient un critère de commercialisation.

---

## 5. Audit DevOps & Déploiement

### `docker-compose.yml`

Deux services seulement (`postgres`, `server` — le frontend est buildé et servi statiquement par le binaire serveur, l'agent n'est volontairement pas conteneurisé puisqu'il a besoin d'un accès natif à Docker/APT/systemd de l'hôte supervisé). Points positifs vérifiés : `postgres` a un vrai healthcheck (`pg_isready`, 10s/5s/5), n'expose aucun port sur l'hôte (accessible seulement sur le réseau interne Compose), `restart: unless-stopped` sur les deux services. Manques réels pour un usage production : **aucun healthcheck sur le service `server` lui-même**, aucune limite de ressources (`cpus`/`mem_limit`/`deploy.resources`) sur l'un ou l'autre, pas de `read_only`/`cap_drop`/`security_opt`/`user:`, pas de segmentation réseau explicite (réseau par défaut implicite), et — le plus important — **aucun service de sauvegarde**.

### GitHub Actions — plus mature que ce que le brief laisse supposer

6 workflows. Ce qui est déjà en place et fonctionne : `govulncheck` (Go, serveur+agent, bloquant), `nancy`/Sonatype OSS Index (CVE des dépendances Go, bloquant), `npm audit --audit-level=moderate` (bloquant), **Dependabot** configuré sur 5 écosystèmes (`github-actions`, `gomod` ×2, `npm`, `docker`) avec regroupement mineur/patch, **CodeQL** (Go + JavaScript, `security-extended`, en un seul passage malgré l'absence de `go.work` racine), Trivy sur l'image serveur construite (résultats SARIF vers l'onglet Security), et sur chaque release : matrice 4 plateformes agent avec sha256, image Docker multi-arch (amd64+arm64) via buildx/QEMU poussée sur GHCR, **avec provenance BuildKit et SBOM** (`provenance: mode=max`, `sbom: true`).

Ce qui manque réellement : **Trivy est purement informatif** (`exit-code: '0'`, ne bloque jamais un merge), **aucune signature d'image** (pas de cosign, alors que la provenance/SBOM sont déjà là — la signature serait la suite logique), **aucun scan de secrets** (pas de gitleaks/trufflehog dans le repo), et **aucun seuil de couverture de test bloquant** (Codecov en `continue-on-error: true` sur les deux modules Go).

### Dockerfile — un seul fichier dans tout le repo, et il tourne en root

`server/Dockerfile` est le seul Dockerfile du projet (multi-stage : build frontend Node → build Go → `alpine:3.24` final, avec cache mounts BuildKit pour npm/go mod/go build/Vite — du bon travail). **Mais aucune instruction `USER` : le conteneur tourne en root par défaut.** Correctif trivial, à faible risque de régression :

```dockerfile
# server/Dockerfile — stage final, avant ENTRYPOINT
RUN addgroup -g 1000 supervisor && adduser -D -u 1000 -G supervisor supervisor
USER supervisor
ENTRYPOINT ["./serversupervisor"]
```
(à valider : que le process n'ait pas besoin d'écrire dans un chemin nécessitant root — sinon ajuster les permissions du volume/dossier concerné avant le `USER`).

### Observabilité — le supervisor n'est pas lui-même observé au-delà des logs

**Logging structuré : réel et solide**, confirmé sur les deux modules (`slog`, JSON en prod/texte en dev côté serveur, texte par défaut côté agent car destiné à journald, corrélation par `request_id` côté serveur via `RequestIDMiddleware`).

**Prometheus : absent.** Les seules routes `/metrics/*` du projet (`/metrics/summary`, `/metrics/history`, `/metrics/aggregated`) renvoient du JSON métier pour le dashboard, pas du format d'exposition Prometheus — et `prometheus/client_golang` n'est dans aucun `go.mod`.

**OpenTelemetry : présent uniquement comme dépendance transitive inutilisée.** `go.mod` référence des packages `go.opentelemetry.io/*` marqués `// indirect` — probablement tirés par `testcontainers-go` (utilisé pour les tests d'intégration Postgres), pas par du code applicatif. Zéro import réel dans le code source, zéro tracer initialisé.

**Conséquence pratique** : un outil dont la mission est de superviser d'autres serveurs n'a lui-même aucune métrique exposée à un système de monitoring externe ni de tracing pour diagnostiquer une latence anormale sur ses propres endpoints. Pas critique à l'échelle actuelle (les logs structurés + `request_id` couvrent le debugging de base), mais à ajouter si le produit grossit : un endpoint `/metrics` minimal (latence HTTP, goroutines actives, taille des pools) coûte peu et rendrait le produit cohérent avec ce qu'il vend à ses utilisateurs.

### Backup — le vrai trou du bilan DevOps

**Aucune sauvegarde de la base ServerSupervisor elle-même.** Le seul mécanisme de persistance est un volume Docker nommé (`postgres_data`). Les seules occurrences de « backup » dans le repo concernent soit un exemple de documentation pour une tâche custom agent (l'utilisateur pourrait configurer un `pg_dump` *sur son propre serveur supervisé*, pas sur la base de ServerSupervisor), soit la fonctionnalité produit de suivi des jobs `vzdump` de Proxmox (observabilité en lecture des sauvegardes *des autres*), soit le rollback de binaire du self-update de l'agent — rien à voir avec la protection des données du produit lui-même.

Ce qui **est** bien fait : les politiques de rétention/compression TimescaleDB (compression à 7 jours et rétention 30-90 jours selon la table, configurées migration par migration, avec un override runtime pour `system_metrics`/`disk_metrics`). Mais **rétention n'est pas sauvegarde** — ce sont des politiques d'expiration de données, pas une protection contre une corruption de volume, une erreur de manipulation, ou une perte du disque hôte. Pour un produit dont la raison d'être est d'être la source de vérité de l'historique d'infra et des alertes, l'absence totale de story de disaster recovery est le manque le plus sérieux de cette section. Recommandation concrète, pas un gros projet : un service planifié (`pg_dump` compressé vers un stockage externe, ou `pgBackRest`/`WAL-G` si vous voulez du PITR) + une procédure de restauration documentée et **testée au moins une fois**.

### Kubernetes/Helm

Absence confirmée (aucun répertoire `k8s/helm/charts`, aucun `Chart.yaml`). Pas un manque à ce stade : le modèle de déploiement actuel est mono-nœud via docker-compose, cohérent avec la cible (une équipe qui déploie un outil de supervision pour sa propre infra, pas un opérateur SaaS multi-tenant). Ça deviendrait pertinent seulement si le produit visait un jour un mode SaaS multi-tenant nécessitant scaling horizontal du serveur — pas la direction actuelle du projet, donc pas une priorité.

### `.env.example` et les « secrets par défaut dangereux »

En y regardant précisément, ce n'est pas la faille que le brief suppose. `.env.example` contient des placeholders instructifs (`generate-a-random-long-secret-here`), pas des secrets faibles copiables tels quels. Les vraies valeurs de repli inline dans `docker-compose.yml` (`change-me-in-production-please`, `admin`, `supervisor`) sont **exactement** les chaînes que `ValidateStrict()` interdit au boot en dehors de `APP_ENV=dev` — un `docker compose up` sans fichier `.env` crash-loop au démarrage plutôt que de tourner silencieusement avec des identifiants par défaut. Seule vraie lacune : la validation est une liste noire de valeurs exactes plus une longueur minimale pour le JWT secret, pas un contrôle d'entropie — un mot de passe faible mais différent de `admin` (ex. `1234`) la passerait. Détail doc à corriger : le README marque `JWT_SECRET` et `ADMIN_PASSWORD` comme « à changer ! » mais pas `DB_PASSWORD`, alors que `ValidateStrict()` bloque les trois de façon identique.

---

## 6. Plan de réécriture

**Cadrage honnête avant de détailler : si je devais réécrire ce projet aujourd'hui, je garderais environ 80 % de l'architecture actuelle telle quelle.** La couche service+repository, les erreurs typées, le bus d'événements pilotant les WebSockets, la structure de tests avec testcontainers, le découpage frontend en composables/stores/types générés — c'est déjà l'état de l'art pour ce genre de projet. Ce qui suit répond aux questions posées, mais en étant honnête sur ce qui est déjà acquis plutôt que d'inventer une réécriture pour le plaisir d'en proposer une.

### Stack technique

| Composant | Choix | Justification |
|---|---|---|
| **Serveur** | Go, même organisation (`services/<domaine>` + ports repository) | Déjà l'architecture cible ; seul changement : généraliser la supervision de goroutines (voir §2) et ajouter un canal WS bidirectionnel pour les agents. |
| **Agent** | Go, mêmes choix | Corriger le timeout de collecte manquant, harmoniser `build.sh` sur la matrice de release, ajouter des tests sur self-update/report-loop/curseur web-logs. |
| **Frontend** | Vue 3 + TypeScript, conservés | Migration déjà à 100 %, stores/composables déjà en place. Discussion ouverte sur Tabler → Tailwind/shadcn-vue (voir §1), pas un verdict tranché. |
| **Base** | TimescaleDB, conservée | Déjà bien exploitée (compression/rétention/continuous aggregates). Le vrai chantier est le backup, pas le choix du moteur. |
| **Auth** | JWT + refresh + MFA/TOTP, conservés | OIDC/Keycloak serait disproportionné pour un outil self-hosted mono-tenant — ajoute un IdP entier à opérer pour un gain (SSO fédéré) que ce public n'a généralement pas besoin. À réserver à une hypothétique offre « enterprise multi-tenant » future, pas comme prérequis v1. |
| **Transport agent↔serveur** | HTTP JSON pour les rapports (inchangé) + **un WebSocket persistant supplémentaire** pour le dispatch de commandes et le streaming de sortie | Réutilise le pattern WS déjà validé côté navigateur ; élimine la latence de dispatch bornée par `report_interval` et le POST-par-chunk, sans le coût d'un broker MQTT ou d'un pipeline protobuf gRPC. |

### Structure de packages serveur

La structure actuelle (`internal/handlers` = transport HTTP, `internal/services/<domaine>` = logique métier + port repository, `internal/database` = implémentation repository, `internal/ws` = transport WebSocket, `internal/models` = domaine) **couvre déjà** la séparation transport/service/repository/domaine demandée. Le seul changement que je proposerais est cosmétique — clarifier l'intention dans l'arborescence, sans toucher au comportement :

```
server/internal/
├── domain/            # ex-"models" : structs + interfaces pures, zéro dépendance
├── service/<domaine>/ # ex-"services/<domaine>" : logique métier + port Repository
├── repository/postgres/ # ex-"database" : implémentation des ports, *sql.DB
├── transport/
│   ├── http/          # ex-"handlers" : gin, bind→service→respondError
│   └── ws/             # ex-"ws" : hubs, event bus subscribers
├── scheduler/          # inchangé
└── infrastructure/     # config, logging, notify, proxmoxclient, gitprovider
```

Renommer `handlers`→`transport/http` et `database`→`repository/postgres` rend explicite ce qui est déjà vrai en pratique : la couche repository est déjà interchangeable en théorie (ports définis par le consommateur), même si personne n'a de besoin réel de changer de moteur SQL. Je ne le ferais qu'à l'occasion d'un autre chantier — un renommage de masse sans changement de comportement n'est pas prioritaire seul.

### Exemples de code pour les points prioritaires

**Canal WebSocket bidirectionnel agent↔serveur** (remplace le dispatch via réponse de report + le POST-par-chunk), esquisse côté serveur :

```go
// internal/ws/agent_hub.go — même famille que CommandStreamHub existant
type AgentHub struct {
	mu    sync.Mutex
	conns map[string]*websocket.Conn // hostID -> connexion agent persistante
}

func (h *AgentHub) DispatchCommand(hostID string, cmd *models.RemoteCommand) error {
	h.mu.Lock()
	conn, ok := h.conns[hostID]
	h.mu.Unlock()
	if !ok {
		return apperr.NotFound("agent not connected") // repli : reste en queue pour le prochain report HTTP
	}
	return conn.WriteJSON(cmd) // dispatch immédiat, sans attendre le prochain tick de 30s
}
```
Côté agent, une goroutine dédiée maintient la connexion (mêmes API key + reconnection/backoff que `useWebSocket.ts` côté frontend) en parallèle de la boucle de report HTTP existante — qui reste le chemin de repli si le WS est down, pas un remplacement total.

**Timeout de collecte agent** (corrige le risque de blocage identifié en §3) :

```go
// agent/internal/collector/disk.go
func collectDiskUsage(ctx context.Context) ([]DiskMetric, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "df", "-h").Output()
	// ...
}
```
et dans `reporter.go`, borner l'attente globale :
```go
done := make(chan struct{})
go func() { wg.Wait(); close(done) }()
select {
case <-done:
case <-time.After(20 * time.Second):
	slog.Warn("metric collection timed out, sending partial report")
}
```

**Fix des deux vulnérabilités critiques** (détail complet en §2) : garde de rôle manquant sur `GetSettings`, garde `v != ""` manquant sur 4 champs d'`OverrideFromDB` — patchs de quelques lignes chacun, montrés en §2, à traiter avant tout autre chantier de cet audit.

**Frontend — partager la connexion notifications au lieu de la dupliquer :**

```ts
// composables/useNotifications.ts — déjà le singleton logique, à réutiliser depuis AlertsView
// AlertsView.vue — remplacer l'appel useWebSocket('/api/v1/ws/notifications', ...) local par :
const { onAlert } = useNotifications() // même composable que NotificationBell, pas une 2e connexion
onAlert((payload) => { /* logique spécifique à la vue Alerts */ })
```

### Priorités court terme vs long terme

Répondu en détail dans la roadmap ci-dessous — mais le principe général : **aucun des points 🔴 n'exige de refactoring**, ce sont tous des correctifs de quelques lignes à quelques dizaines de lignes. Le vrai travail de fond (backup/DR, tests sur les zones à trou, généralisation de la supervision de goroutines) est classé 🟡 parce qu'il demande plus qu'un patch mais reste dans le cadre de l'architecture existante — rien ne justifie une réécriture complète.

---

## 7. Roadmap priorisée

### 🔴 Critique — sécurité, corruption, perte de service (immédiat, sans refactoring)

**Statut : les 5 points 🔴 ont été corrigés** (voir commit sur cette branche). Détail des correctifs réellement appliqués sous le tableau.

| # | Problème | Preuve | Correctif | Statut |
|---|---|---|---|---|
| 1 | `GET /api/v1/settings` sans contrôle de rôle, renvoie mot de passe SMTP + token GitHub en clair à tout utilisateur authentifié | `handlers/settings.go:25-27`, `services/settings/service.go:60-68` | Ajouter le garde admin manquant | ✅ Corrigé |
| 2 | `OverrideFromDB` écrase silencieusement `smtp_user/smtp_pass/ntfy_url/github_token` avec une chaîne vide à chaque sauvegarde Settings | `config.go:195-215`, `services/settings/service.go:97-102` | Ajouter le garde `&& v != ""` déjà présent sur les autres champs, + test de régression | ✅ Corrigé |
| 3 | Goroutines non supervisées (`poller.Every`, fan-out WS dashboard, fan-out host `Complete()`, etc.) : une panique tue tout le process | `poller/poller.go`, `ws/snapshots.go:68-109`, `services/host/service.go:233-238` | Généraliser le `recover()` de `background/runner.go` via un helper partagé, l'appliquer à tous les `go func()` listés | ✅ Corrigé |
| 4 | Conteneur serveur tourne en root (pas de `USER` dans le Dockerfile) | `server/Dockerfile` | Ajouter un utilisateur non-root avant `ENTRYPOINT` | ✅ Corrigé |
| 5 | Aucune sauvegarde de la base ServerSupervisor — un volume Docker nommé, rien d'autre | `docker-compose.yml`, absence totale de script/service de backup | Service planifié `pg_dump`/`pgBackRest` + procédure de restauration testée | ✅ Corrigé |

**Détail des correctifs appliqués :**

1. **Garde admin ajouté** sur `SettingsHandler.GetSettings`, identique au pattern déjà utilisé par les 5 autres méthodes du même fichier. Choix délibéré : je n'ai **pas** modifié la forme de `Snapshot()` (pas de masquage du secret brut par un booléen `xSet`) — `SettingsView.vue`/`SettingsSmtpCard.vue`/`SettingsNotificationsCard.vue` préremplissent aujourd'hui le formulaire d'édition avec la valeur réelle (`form.value.smtpPass = s.smtpPass || ''`), un pattern d'édition assumé par le produit. Retirer la valeur brute casserait ce préremplissage et exigerait une refonte UX (masquage + flux « modifier » explicite) hors du périmètre d'un correctif de sécurité ciblé. Une fois le garde admin en place, l'exposition résiduelle (un admin voit son propre secret configuré) est un risque nettement plus faible que la fuite d'origine (n'importe quel viewer) et cohérent avec le modèle RBAC existant où l'admin a déjà l'accès le plus large. À réévaluer séparément si une refonte du formulaire Settings est planifiée.
2. **Garde `&& v != ""` ajouté** sur les 4 champs (`smtp_user`, `smtp_pass`, `ntfy_url`, `github_token`), plus un test `TestOverrideFromDB_KeepsSecretsWhenBlank` dans `settings_override_test.go` qui verrouille le comportement.
3. **Nouveau package `internal/safego`** (`Go`, `Recover`, `RecoverErr`) : un helper unique de recover+log, réutilisé par `background/runner.go` (refactor DRY, comportement identique) et appliqué à chaque goroutine non protégée identifiée par l'audit — `poller.Every` (par tick, pas par appel, pour ne pas tuer le poller après une seule panique), les 8 fan-out de `ws/buildDashboardPayload`, les 3 fan-out de `ws/buildVersionComparisons`, les 6 fan-out de `host.Complete()`, `proxmox.TriggerPollByID`/`NodeGuestNetworks`, `releasetracker.TriggerCheck`/`Run` (×2), le pool de workers de `weblogs.resolveIPsWithContext`, `notifychannels.Send` (push), `npm.RefreshNow`, `alerts.ResolveStaleIncidentsForRule` (déclenché depuis `router.go`), le ticker de nettoyage du rate limiter, `ws.readLoop` et `CommandStreamHub.runBroadcast`. Cas particulier : les 3 goroutines de `audit.HostTimeline` utilisent `RecoverErr` plutôt que `Recover` — elles se rejoignent via des channels, donc un panic simplement journalisé sans réponse aurait bloqué indéfiniment le `select` qui attend exactement 3 messages ; `RecoverErr` transforme la panique en erreur envoyée sur `errCh` pour débloquer proprement l'appelant.
4. **Utilisateur non-root** ajouté à `server/Dockerfile` (stage final) : `addgroup`/`adduser` uid/gid 1000, `chown` de `/app`, `USER supervisor` avant `ENTRYPOINT`.
5. **Service `postgres-backup`** ajouté à `docker-compose.yml` (image `prodrigestivill/postgres-backup-local`, `pg_dump` planifié + rotation jours/semaines/mois, volume dédié `postgres_backups`), variables documentées dans `.env.example`, procédure de restauration testable pas-à-pas ajoutée au README (section « Sauvegarde & restauration »). Cette procédure n'a pas pu être exécutée de bout en bout dans cet environnement (pas de démon Docker disponible dans ce sandbox) — à valider avant une mise en production.

`go build ./...`, `go vet ./...` et `go test ./...` passent sur l'ensemble du module serveur après ces changements.

### 🟡 Important — dette bloquante, 3 prochains mois

**Statut : traité, avec une reclassification importante sur le point 10 (voir détail).**

| # | Problème | Correctif | Statut |
|---|---|---|---|
| 6 | Rate limiter 100 % en mémoire — cassé en multi-instance | Backend partagé (Redis) si scale-out prévu, sinon documenter la limite « single instance » | ✅ Documenté (`CLAUDE.md`) |
| 7 | Fallback WebSocket `?token=` en query string encore présent | Retirer le fallback, ne garder que cookie + message d'auth in-band | ✅ Corrigé |
| 8 | Aucun timeout sur la collecte agent (`df`/`smartctl`) combiné à un ticker qui abandonne ses ticks → risque de blocage définitif du reporting | `exec.CommandContext` partout + timeout global sur la collecte | ✅ Corrigé |
| 9 | Self-update agent : intégrité par checksum seul (pas de signature), health-check superficiel | Signer les releases (cosign/minisign) + health-check qui valide un vrai cycle de collecte | ✅ Corrigé |
| 10 | `tasks.yaml` sans mécanisme de distribution fleet-wide | ~~Endpoint de push de config côté serveur~~ | ⚠️ **Reclassé : pas un bug** |
| 11 | Curseur web logs : détection de rotation par taille seule, écriture non atomique, pas de dédup serveur | Écriture temp+rename+fsync, détection taille+mtime, clé de dédup à l'ingestion | ✅ Corrigé (sauf dédup serveur, voir détail) |
| 12 | Zéro test sur `internal/auth` (TOTP), `internal/ws`, `internal/scheduler`, `internal/poller`, `internal/dispatch` | Prioriser TOTP (sécurité), puis les hubs WS | ✅ Corrigé |
| 13 | Migrations forward-only, pas d'outil dédié | Évaluer goose, ou documenter/tester une procédure de rollback manuel | ✅ Runbook rédigé et testé |
| 14 | Double connexion WebSocket notifications (`NotificationBell` + `AlertsView`) | Partager la connexion via le composable existant | ✅ Corrigé |
| 15 | `stores/hosts.ts` (TTL 60s REST) et `stores/dashboard.ts` (WS live) peuvent afficher des chiffres différents sur le même écran | Source unique de vérité pour l'état hosts up/down | ✅ Corrigé |
| 16 | 89 % des vues frontend appellent l'API directement plutôt que via composable | Étendre le pattern composable existant aux mutations restantes | ✅ Migration complète des ~25 vues |
| 17 | `build.sh` (3 plateformes) désynchronisé de la matrice de release réelle (4 plateformes) | Aligner `build.sh` sur `release.yml` | ✅ Corrigé |

**Point 10 reclassé — retour de l'auteur du projet, à retenir pour la suite :** ce n'est pas un oubli mais une frontière de sécurité volontaire. `tasks.yaml` est une allowlist de commandes définie localement, en filesystem, sur chaque hôte — précisément pour que le **serveur ne puisse jamais pousser une commande arbitraire à un agent**. Construire un endpoint serveur→agent capable d'écrire ce fichier reviendrait à donner au serveur la capacité de définir et faire exécuter n'importe quelle commande sur les hôtes supervisés, ce qui est exactement le pouvoir que ce mécanisme est censé retirer au serveur. Aucune synchronisation n'a donc été construite. Cette section de l'audit et sa recommandation associée sont annulées ; le comportement actuel (configuration manuelle, par hôte, par un opérateur ayant un accès filesystem local) est correct par conception.

**Détail des correctifs appliqués (points 6-9, 11-17) :**

- **#6** : note ajoutée dans `CLAUDE.md` expliquant que `IPRateLimiter` est en mémoire process, cohérent avec le reste de l'architecture (bus d'événements, cache dashboard, hubs WS — tous en mémoire process également) ; migrer vers Redis n'aurait de sens que dans le cadre d'un chantier de scale-out horizontal complet, pas isolément.
- **#7** : fallback `c.Query("token")` retiré de `WSTokenMiddleware` (`middleware.go`) et `authenticateWSClaims` (`ws/base.go`) — vérifié qu'aucun client actuel ne l'utilisait avant de le retirer.
- **#8** : tous les appels `df`/`smartctl`/`ls` de `agent/internal/collector/disk.go` bornés par `exec.CommandContext` (10-15s selon la commande) ; `reporter.go` borne désormais la phase de collecte complète par un timeout de 25s avec `select`/`time.After` autour de `wg.Wait()`.
- **#9** : nouveau flag `--internal-healthcheck` qui exécute un vrai cycle `collector.CollectSystem()` et sort en 0/1 — exécuté par le self-updater **avant** de redémarrer le service live (pas après), pour ne jamais perturber le service en cours si le nouveau binaire est cassé ; signature cosign keyless (Sigstore/OIDC GitHub Actions, sans clé à gérer) ajoutée sur les binaires agent et l'image Docker serveur dans `release.yml`, avec commandes de vérification documentées dans le corps de la release.
- **#11** : écriture du curseur passée en temp-file + `fsync` + `rename` atomique ; détection de rotation combinant désormais taille **et** identité de fichier (`os.SameFile`, cache en mémoire par process) — couvre le cas d'une rotation rapide où le nouveau fichier dépasse déjà l'ancien offset avant le scan suivant. Testé par un test de régression qui simule une vraie rotation logrotate (rename, pas delete) et vérifie qu'un bootstrap propre se déclenche au lieu d'un seek à l'aveugle. La déduplication côté serveur à l'ingestion n'a pas été implémentée (nécessiterait une clé naturelle + une migration de schéma ; le fail-open actuel reste sans risque de blocage, juste un risque de double-comptage résiduel dans de rares cas).
- **#12** : tests ajoutés sur `internal/auth` (génération/vérification TOTP, backup codes, y compris les cas de stockage corrompu/vide), `internal/poller` (dont un test qui panique délibérément dans un tick pour vérifier que `safego.Recover` empêche l'arrêt du poller — validation directe du correctif du point 🔴 #3), `internal/ws` (fonctions pures `isAllowedOrigin`/`snapshotChanged`), `internal/scheduler` (Add/Remove/Update/NextRun/Start avec une fausse DB), `internal/dispatch` (test d'intégration via `testutil.NewPostgresDB`, qui skip proprement sans Docker et s'exécute réellement en CI).
- **#13** : procédure de rollback manuel rédigée dans le README (section « Revenir en arrière après une mise à jour ratée ») **et testée réellement** (pas seulement documentée en théorie) : schéma complet migré, migration destructrice simulée, `pg_dump`/`pg_restore`, état revenu identique bit à bit. Testé sur un Postgres 16 nu hors conteneur (pas de démon Docker dans cet environnement) — le mécanisme est donc vérifié, les commandes `docker compose exec` exactes du README n'ont pas pu être rejouées telles quelles.
- **#14/#15** : voir le détail donné directement dans la conversation au moment de ces deux correctifs (composable `useNotifications` transformé en singleton partagé avec abonnement `onNotificationsMessage` pour `AlertsView` ; `stores/dashboard.ts` lit maintenant `stores/hosts.ts` via une `computed` au lieu de garder sa propre copie).
- **#16** : chaque vue qui appelait l'API directement a désormais un composable `use<Domaine>.ts` dédié qui possède l'état/la logique API/WS ; la vue ne garde que les refs de template DOM (focus, ref de composant enfant type `NetworkGraph`, `defineExpose`) et les formatteurs d'affichage purs. Piège rencontré et documenté pour toutes les vues : `vue-tsc` signale un ref de template DOM renvoyé par un composable comme « jamais lu » (l'attribut `ref="x"` n'est pas une lecture JS) — la règle appliquée partout est donc : les refs de template restent dans la vue, le composable expose un signal simple (compteur) si un focus doit être déclenché.
  Périmètre réel des ~25 vues, vérifié fichier par fichier (`grep` de tout appel direct au client API restant dans `views/`) : 24 vues migrées au total dans ce chantier — `LoginView`/`DockerView` en référence initiale, 10 vues migrées par lot puis vérifiées (`Monitoring`, `ActiveCommands`, `AddHost`, `UptimeProbeDetail`, `NotificationCenter`, `NPM`, `ProxmoxGuest`, `Proxmox`, `AuditLogs`, `GitWebhooks`), 2 composables orphelins (écrits mais jamais branchés) recâblés dans leur vue (`ProxmoxNode`, `Traffic` — ce dernier avait aussi oublié le câblage `useHostsStore`, ajouté au passage), et 10 vues restantes migrées directement (`Users`, `SSLCertificateDetail`, `GitWebhookDetail`, `ReleaseTrackerDetail`, `Settings`, `Apt`, `AccountSecurity`, `Account`, `Network`, `Bot`). `DashboardView`/`HostDetailView`/`AlertsView` avaient déjà leur composable avant cet audit ; `GlobalScheduledTasksView` reste un monolithe non migré mais c'est une dette distincte, déjà suivie par ailleurs (split Phase 7), pas un oubli de ce point.
  Vérification réelle, pas seulement statique : `ProxmoxNodeView`, `TrafficView` et `BotView` ont été testées dans un vrai navigateur (Playwright/Chromium) contre un vrai backend + PostgreSQL 16 (TimescaleDB non disponible dans ce bac à sable — contournement temporaire, jamais commité, du hard-fail au démarrage) et des données seedées à la main. Ce test a mis en évidence un vrai bug pré-existant (pas introduit par la migration) : `network_topology_config.host_overrides` avait un défaut `'{}'` (objet) alors que le frontend le traite partout comme un tableau (`manual_services`/`excluded_ports`, à côté, ont le bon défaut `'[]'`) — sur une base fraîche sans configuration réseau déjà enregistrée, `NetworkView` plantait avec « hostPortConfig.value is not iterable » (absorbé par l'`ErrorBoundary`, jamais visible dans les logs serveur). Corrigé par une migration dédiée (`072_fix_network_topology_host_overrides_default.sql`) plus le correctif du repli Go correspondant.
- **#17** : `build.sh` reconstruit désormais exactement la même matrice à 4 cibles que `release.yml` (amd64/arm64/armv7/armv6, `GOARM` explicite) — testé en construisant réellement les 4 binaires.

### 🟢 Amélioration — backlog long terme

| # | Sujet | Piste |
|---|---|---|
| 18 | Pas de scan de secrets, pas de signature d'image malgré SBOM+provenance déjà présents, Trivy non bloquant | gitleaks en CI, cosign sur les images GHCR, Trivy bloquant sur les CVE critiques |
| 19 | Pas de seuil de couverture bloquant | Seuil progressif sur Codecov |
| 20 | Pas d'observabilité Prometheus/OpenTelemetry pour le produit lui-même | `/metrics` minimal (latence, goroutines), tracing OTel optionnel |
| 21 | Pas de contrôle de taille de bundle frontend, Tabler sans pipeline Sass | Gate de taille en CI ; réévaluer Tailwind/shadcn-vue si le bundle grossit |
| 22 | Suite Playwright désactivée en CI (bug amont Vite8/Vitest4) | Suivre l'upstream, réactiver dès correction |
| 23 | Zéro test unitaire sur les stores Pinia | Ajouter `stores/*.spec.ts` |
| 24 | i18n absent, a11y partielle (57 % des vues sans aria/role propre) | À cadrer seulement si commercialisation/internationalisation envisagée ; `@axe-core/playwright` peu coûteux à ajouter dès maintenant |
| 25 | Pas de manifests K8s/Helm | Pas nécessaire tant que le modèle reste mono-nœud ; préparer seulement si mode SaaS multi-tenant envisagé |
| 26 | Latence de dispatch de commande bornée par l'intervalle de report + POST-par-chunk pour le streaming | Canal WebSocket bidirectionnel agent↔serveur (détaillé en §6), sans aller jusqu'à MQTT/gRPC |
