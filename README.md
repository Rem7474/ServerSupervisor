# ServerSupervisor

Système de supervision d'infrastructure : monitoring de VMs, conteneurs Docker, mises à jour APT, services systemd et suivi des releases GitHub.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Dashboard (Vue.js)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Hosts    │ │ Docker   │ │ Network  │ │ APT Console   │  │
│  │ Dashboard│ │ Versions │ │Topology  │ │ Commandes     │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Alertes  │ │ Audit    │ │ Users    │ │ System        │  │
│  │ (rules)  │ │Commandes │ │ (RBAC)   │ │ Systemd/Proc  │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
├─────────────────────────────────────────────────────────────┤
│              Server Go (API REST + WebSocket + JWT)         │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Auth+MFA │ │ Rate     │ │ Alert    │ │ Command       │  │
│  │ JWT+Keys │ │ Limiting │ │ Engine   │ │ Stream Hub    │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────────┐  │
│  │ Audit    │ │ GitHub   │ │ Settings │ │ Metrics       │  │
│  │ Logs     │ │ Tracker  │ │ (DB)     │ │ Aggregation   │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                       PostgreSQL                            │
└─────────────────────────────────────────────────────────────┘
         ▲              ▲              ▲
    Push (30s)     Push (30s)     Push (30s)
         │              │              │
    ┌────┴────┐    ┌────┴────┐    ┌────┴────┐
    │ Agent   │    │ Agent   │    │ Agent   │
    │ (Go)    │    │ (Go)    │    │ (Go)    │
    │ VM-1    │    │ VM-2    │    │ VM-N    │
    └─────────┘    └─────────┘    └─────────┘
```

## Fonctionnalités

### Dashboard
- **Vue d'ensemble** : tous les hôtes avec statut temps réel (CPU, RAM, uptime, version agent)
- **Détail par hôte** : graphiques CPU/RAM historiques (24h / 7j / 30j), disques, conteneurs, APT, historique de commandes toutes sources confondues
- **Docker** : vue globale de tous les conteneurs et projets docker-compose sur toute l'infrastructure
- **Network** : topologie réseau avec liens Docker (réseaux, env vars), override manuel des services
- **APT** : gestion centralisée des mises à jour avec actions groupées et console live streamée
- **System** : exécution à distance de commandes systemd (start/stop/restart/enable/disable), logs journalctl streamés, snapshot des processus
- **Streaming commandes** : affichage en temps réel de la sortie des commandes longues via WebSocket
- **Versions** : suivi des releases GitHub et comparaison avec les images Docker en cours
- **Audit → Commandes** : historique paginé de toutes les commandes (apt/docker/systemd/journal/processus), toutes sources
- **Audit → Connexions** : logs de connexion avec statistiques et IPs bloquées (admin)
- **Alertes** : règles d'alertes configurables avec notifications email (SMTP), ntfy, webhook ou notifications navigateur
- **Sécurité** : résumé des connexions 24h, IPs bloquées, déblocage manuel

### Agent
- Collecte automatique : CPU, RAM, disques, réseau, uptime
- Monitoring Docker via CLI (conteneurs, réseaux, projets compose, variables d'environnement)
- Détection des mises à jour APT disponibles, extraction des CVEs
- Collecte S.M.A.R.T. et métriques disques (via `smartctl`)
- Exécution de commandes distantes : APT, Docker/Compose, systemd, journalctl, snapshot processus
- Streaming temps réel de la sortie des commandes longues (chunk par chunk)
- Rapport de résultat des commandes autonomes au démarrage (ex: `apt update`)
- Binaire unique sans dépendances, multi-architecture (amd64/arm64/armv7/armv6)

### Sécurité
- Authentification JWT avec refresh tokens
- MFA/2FA (TOTP) optionnel par compte
- API Keys uniques par agent avec rotation
- Vérification stricte de l'appartenance des commandes à chaque hôte
- Rate limiting par IP avec cleanup automatique et support reverse proxy
- CORS multi-origines configurable
- Audit logs de toutes les actions utilisateurs et agent
- RBAC 3 niveaux : `admin` / `operator` / `viewer`
- Blocage automatique des IPs sur échecs répétés

---

## Démarrage rapide

### 1. Déployer le serveur

```bash
git clone <repo-url> && cd ServerSupervisor
cp .env.example .env
# Éditer .env avec vos valeurs (JWT_SECRET, ADMIN_PASSWORD, etc.)
docker compose up -d
```

Le dashboard est accessible sur `http://localhost:8080` (login: `admin` / `admin` par défaut, **à changer**).

### 2. Enregistrer un hôte

1. Dashboard → **Ajouter un hôte**
2. Renseigner le nom, hostname/IP, OS
3. **Copier la clé API** affichée (elle ne sera plus visible ensuite)

### 3. Installer l'agent sur une VM

#### Via les releases GitHub (recommandé)

```bash
# Remplacer ARCH par : amd64, arm64, armv7, armv6
curl -L https://github.com/<org>/serversupervisor/releases/latest/download/agent-linux-ARCH.gz | \
  gunzip > /usr/local/bin/serversupervisor-agent
chmod +x /usr/local/bin/serversupervisor-agent
```

#### Via le script d'installation

```bash
sudo bash agent/install.sh --server-url http://your-server:8080 --api-key your-key
```

#### Manuellement

```bash
# Compiler l'agent (depuis la machine de dev)
cd agent
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o serversupervisor-agent ./cmd/agent
scp serversupervisor-agent user@vm:/usr/local/bin/

# Sur la VM : créer la config
sudo mkdir -p /etc/serversupervisor
sudo tee /etc/serversupervisor/agent.yaml <<EOF
server_url: "http://your-server:8080"
api_key: "la-clé-api-copiée"
report_interval: 30
collect_docker: true
collect_apt: true
collect_smart: true
apt_auto_update_on_start: false
insecure_skip_verify: false
EOF

# Installer le service systemd
sudo tee /etc/systemd/system/serversupervisor-agent.service <<EOF
[Unit]
Description=ServerSupervisor Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/serversupervisor-agent --config /etc/serversupervisor/agent.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now serversupervisor-agent
sudo journalctl -u serversupervisor-agent -f
```

### 4. Suivre des repos GitHub

1. Dashboard → **Versions & Repos**
2. Ajouter un repo (ex: `home-assistant` / `core`)
3. Optionnel : associer un nom d'image Docker pour la comparaison automatique
4. Le serveur vérifie les nouvelles releases toutes les 15 minutes

---

## Configuration

### Variables d'environnement serveur

#### Serveur
| Variable | Description | Défaut |
|---|---|---|
| `SERVER_PORT` | Port d'écoute | `8080` |
| `BASE_URL` | URL publique (CORS + WebSocket) | `http://localhost:8080` |
| `TRUSTED_PROXIES` | CIDRs des reverse proxies (ex: `172.18.0.0/16`) | `` |
| `ALLOWED_ORIGINS` | Origins CORS supplémentaires autorisées (virgule) | `` |

#### Base de données
| Variable | Description | Défaut |
|---|---|---|
| `DB_HOST` | Hôte PostgreSQL | `localhost` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_USER` | Utilisateur | `supervisor` |
| `DB_PASSWORD` | Mot de passe | `supervisor` |
| `DB_NAME` | Nom de la base | `serversupervisor` |
| `DB_SSLMODE` | Mode SSL | `disable` |

#### Authentification
| Variable | Description | Défaut |
|---|---|---|
| `JWT_SECRET` | Secret JWT **(à changer !)** | `change-me...` |
| `JWT_EXPIRATION` | Durée de vie du token JWT | `24h` |
| `REFRESH_TOKEN_EXPIRATION` | Durée de vie du refresh token | `168h` |
| `ADMIN_USER` | Nom du compte admin initial | `admin` |
| `ADMIN_PASSWORD` | Mot de passe admin initial **(à changer !)** | `admin` |

#### Rate limiting
| Variable | Description | Défaut |
|---|---|---|
| `RATE_LIMIT_RPS` | Requêtes par seconde max par IP | `100` |
| `RATE_LIMIT_BURST` | Burst max par IP | `200` |

#### GitHub
| Variable | Description | Défaut |
|---|---|---|
| `GITHUB_TOKEN` | Token GitHub (augmente rate limit 60→5000/h) | `` |
| `GITHUB_POLL_INTERVAL` | Intervalle de vérification | `15m` |

#### Alertes & notifications
| Variable | Description | Défaut |
|---|---|---|
| `NOTIFY_URL` | URL ntfy/webhook par défaut | `` |
| `SMTP_HOST` | Serveur SMTP | `` |
| `SMTP_PORT` | Port SMTP | `587` |
| `SMTP_USER` | Utilisateur SMTP | `` |
| `SMTP_PASS` | Mot de passe SMTP | `` |
| `SMTP_FROM` | Email expéditeur | `` |
| `SMTP_TLS` | Activer TLS | `true` |

#### Rétention
| Variable | Description | Défaut |
|---|---|---|
| `METRICS_RETENTION_DAYS` | Rétention des métriques en jours | `30` |
| `AUDIT_RETENTION_DAYS` | Rétention des logs d'audit en jours | `90` |

> Les paramètres de notifications et de rétention sont également éditables depuis le dashboard (Settings) et persistés en base de données.

---

### Configuration agent (`agent.yaml`)

Générer une config par défaut :
```bash
serversupervisor-agent --init
```

| Champ | Description | Défaut | Variable d'env |
|---|---|---|---|
| `server_url` | URL du serveur | `http://localhost:8080` | `SUPERVISOR_SERVER_URL` |
| `api_key` | Clé API de l'hôte **(requis)** | — | `SUPERVISOR_API_KEY` |
| `report_interval` | Intervalle d'envoi en secondes | `30` | `SUPERVISOR_REPORT_INTERVAL` |
| `collect_docker` | Activer le monitoring Docker | `true` | `SUPERVISOR_COLLECT_DOCKER` |
| `collect_apt` | Activer le monitoring APT | `true` | `SUPERVISOR_COLLECT_APT` |
| `apt_auto_update_on_start` | Lancer `apt update` au démarrage de l'agent | `false` | `SUPERVISOR_APT_AUTO_UPDATE_ON_START` |
| `insecure_skip_verify` | Ignorer les erreurs TLS (certificats auto-signés) | `false` | `SUPERVISOR_INSECURE_SKIP_VERIFY` |

> Toutes les options sont également configurables via variables d'environnement (préfixe `SUPERVISOR_`), utile pour les déploiements Docker/Kubernetes.

---

## API REST

### Authentification
```bash
# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Avec TOTP (si MFA activé)
curl -X POST http://localhost:8080/api/auth/login \
  -d '{"username":"admin","password":"admin","totp_code":"123456"}'

# Utiliser le token
curl http://localhost:8080/api/v1/hosts \
  -H "Authorization: Bearer <token>"
```

### Endpoints (JWT requis sauf indication)

#### Authentification
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `POST` | `/api/auth/login` | Connexion (JWT + refresh token) | Public |
| `POST` | `/api/auth/refresh` | Renouveler le token | Public |
| `POST` | `/api/auth/logout` | Déconnexion | Authentifié |
| `GET` | `/api/v1/auth/profile` | Profil utilisateur | Authentifié |
| `POST` | `/api/v1/auth/change-password` | Changer le mot de passe | Authentifié |
| `GET` | `/api/v1/auth/login-events` | Ses propres connexions | Authentifié |
| `GET` | `/api/v1/auth/login-events/admin` | Toutes les connexions | Admin |
| `POST` | `/api/v1/auth/revoke-all-sessions` | Révoquer toutes les sessions | Authentifié |
| `GET` | `/api/v1/auth/security` | Résumé sécurité + IPs bloquées | Admin |
| `DELETE` | `/api/v1/auth/blocked-ips/:ip` | Débloquer une IP | Admin |
| `GET/POST` | `/api/v1/auth/mfa/*` | Gestion MFA/2FA (setup/verify/disable) | Authentifié |

#### Hôtes & Métriques
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts` | Liste des hôtes | Authentifié |
| `POST` | `/api/v1/hosts` | Enregistrer un hôte | Admin |
| `GET` | `/api/v1/hosts/:id` | Détails d'un hôte | Authentifié |
| `PATCH` | `/api/v1/hosts/:id` | Modifier un hôte | Admin |
| `DELETE` | `/api/v1/hosts/:id` | Supprimer un hôte | Admin |
| `POST` | `/api/v1/hosts/:id/rotate-key` | Rotation de clé API | Admin |
| `GET` | `/api/v1/hosts/:id/dashboard` | Dashboard rapide d'un hôte | Authentifié |
| `GET` | `/api/v1/hosts/:id/metrics/history` | Métriques brutes (≤24h) | Authentifié |
| `GET` | `/api/v1/hosts/:id/metrics/aggregated` | Métriques agrégées (heure/jour) | Authentifié |
| `GET` | `/api/v1/metrics/summary` | Résumé global (toutes VMs) | Authentifié |
| `GET` | `/api/v1/hosts/:id/disk/metrics` | Métriques disques | Authentifié |
| `GET` | `/api/v1/hosts/:id/disk/health` | Santé S.M.A.R.T. | Authentifié |

#### Docker & Network
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/containers` | Conteneurs d'un hôte | Authentifié |
| `GET` | `/api/v1/docker/containers` | Tous les conteneurs | Authentifié |
| `GET` | `/api/v1/docker/compose` | Tous les projets Compose | Authentifié |
| `GET` | `/api/v1/docker/versions` | Comparaison versions | Authentifié |
| `POST` | `/api/v1/docker/command` | Envoyer une commande Docker/Compose | Operator+ |
| `GET` | `/api/v1/network` | Snapshot réseau | Authentifié |
| `GET` | `/api/v1/network/topology` | Topologie réseau | Authentifié |
| `GET/PUT` | `/api/v1/network/config` | Config topologie (overrides) | Authentifié |

#### APT
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/apt` | Statut APT d'un hôte | Authentifié |
| `POST` | `/api/v1/apt/command` | Envoyer une commande APT | Operator+ |

#### Système (systemd / journal / processus)
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `POST` | `/api/v1/system/service` | Commande systemd (start/stop/restart…) | Operator+ |
| `POST` | `/api/v1/system/journalctl` | Logs journalctl d'un service | Operator+ |
| `POST` | `/api/v1/system/processes` | Snapshot des processus | Operator+ |

#### Commandes & Audit
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/commands/history` | Historique toutes commandes (hôte) | Authentifié |
| `GET` | `/api/v1/commands/:id` | Statut d'une commande par UUID | Authentifié |
| `GET` | `/api/v1/audit/logs` | Logs d'audit paginés | Admin |
| `GET` | `/api/v1/audit/logs/me` | Ses propres logs d'audit | Authentifié |
| `GET` | `/api/v1/audit/logs/host/:host_id` | Logs d'audit par hôte | Admin |
| `GET` | `/api/v1/audit/logs/user/:username` | Logs d'audit par utilisateur | Admin |
| `GET` | `/api/v1/audit/commands` | Historique paginé toutes commandes | Operator+ |

#### Alertes
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/alerts/incidents` | Incidents déclenchés | Authentifié |
| `GET` | `/api/v1/alert-rules` | Règles d'alertes | Authentifié |
| `POST` | `/api/v1/alert-rules` | Créer une règle | Admin |
| `PATCH` | `/api/v1/alert-rules/:id` | Modifier une règle | Admin |
| `DELETE` | `/api/v1/alert-rules/:id` | Supprimer une règle | Admin |
| `POST` | `/api/v1/alert-rules/test` | Tester une règle | Admin |

#### Utilisateurs (admin)
| Méthode | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/users` | Liste des utilisateurs |
| `POST` | `/api/v1/users` | Créer un utilisateur |
| `PATCH` | `/api/v1/users/:id/role` | Changer le rôle (`admin`/`operator`/`viewer`) |
| `DELETE` | `/api/v1/users/:id` | Supprimer un utilisateur |

#### Repos GitHub
| Méthode | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/repos` | Repos suivis |
| `POST` | `/api/v1/repos` | Ajouter un repo |
| `DELETE` | `/api/v1/repos/:id` | Supprimer un repo |

#### Settings
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET/PUT` | `/api/v1/settings` | Paramètres globaux | Admin |
| `POST` | `/api/v1/settings/test-smtp` | Tester la config SMTP | Admin |
| `POST` | `/api/v1/settings/test-ntfy` | Tester ntfy | Admin |
| `POST` | `/api/v1/settings/cleanup-metrics` | Purger les métriques | Admin |
| `POST` | `/api/v1/settings/cleanup-audit` | Purger les audit logs | Admin |

#### WebSocket (streaming temps réel)
| Endpoint | Description |
|---|---|
| `/api/v1/ws/dashboard` | Flux dashboard global |
| `/api/v1/ws/hosts/:id` | Flux détail hôte (métriques, conteneurs, APT…) |
| `/api/v1/ws/docker` | Flux conteneurs Docker |
| `/api/v1/ws/network` | Flux réseau |
| `/api/v1/ws/apt` | Flux statut APT |
| `/api/v1/ws/commands/stream/:id` | Sortie live d'une commande par UUID |

> Authentification WebSocket : envoyer `{"type":"auth","token":"<jwt>"}` après connexion, ou passer `?token=<jwt>` en query string.

#### Agent (API Key requise)
| Méthode | Endpoint | Description |
|---|---|---|
| `POST` | `/api/agent/report` | Rapport agent (métriques + docker + apt + disques) |
| `POST` | `/api/agent/command/result` | Résultat d'une commande |
| `POST` | `/api/agent/command/stream` | Chunk de sortie en streaming |
| `POST` | `/api/agent/audit` | Log d'action autonome (ex: apt update au démarrage) |

---

## RBAC

| Rôle | Description |
|---|---|
| `admin` | Accès complet — gestion des utilisateurs, hôtes, alertes, settings |
| `operator` | Peut exécuter des commandes (apt, docker, systemd) et consulter l'historique |
| `viewer` | Lecture seule — dashboards, métriques, statuts |

---

## Développement

### Prérequis
- Go 1.22+
- Node.js 20+
- PostgreSQL 16+ (ou Docker)

### Développement local

```bash
# Terminal 1 : PostgreSQL
docker compose up postgres

# Terminal 2 : Serveur Go
cd server && go run ./cmd/server

# Terminal 3 : Frontend Vue.js (proxy → serveur Go)
cd frontend && npm install && npm run dev
```

### Build

```bash
# Build complet via Docker
docker compose build

# Build agent multi-arch
cd agent && bash build.sh v1.0.0

# Build server + frontend séparément
cd server && go build ./...
cd frontend && npm run build
```

---

## Structure du projet

```
ServerSupervisor/
├── server/                         # Serveur Go
│   ├── cmd/server/main.go
│   └── internal/
│       ├── api/
│       │   ├── router.go           # Routes & middleware
│       │   ├── auth.go             # Auth + MFA + login events
│       │   ├── agent.go            # API agent (rapport, commandes, audit)
│       │   ├── audit.go            # Audit logs + historique commandes
│       │   ├── hosts.go            # Gestion hôtes + disques
│       │   ├── docker.go           # Docker + Compose
│       │   ├── system.go           # Systemd + journal + processus
│       │   ├── apt.go              # APT management
│       │   ├── network.go          # Topologie réseau
│       │   ├── alert_rules.go      # Règles d'alertes (CRUD unifié)
│       │   ├── alerts.go           # Incidents d'alertes
│       │   ├── users.go            # Gestion utilisateurs (RBAC)
│       │   ├── settings.go         # Settings dynamiques (DB)
│       │   ├── ws.go               # WebSocket handlers
│       │   ├── command_stream.go   # Hub streaming commandes
│       │   └── middleware.go       # JWT, API Key, CORS, rate limiter
│       ├── alerts/engine.go        # Moteur d'évaluation des alertes
│       ├── config/config.go        # Config + OverrideFromDB
│       ├── database/               # Couche PostgreSQL (db_*.go)
│       │   └── migrations/         # Migrations SQL (001 → 011)
│       ├── github/                 # GitHub release tracker
│       └── models/models.go        # Modèles de données partagés
├── agent/                          # Agent Go
│   ├── cmd/agent/main.go
│   └── internal/
│       ├── collector/              # system.go, docker.go, apt.go, disk.go…
│       ├── config/config.go        # Config YAML + env vars
│       └── sender/sender.go        # Envoi rapports + commandes
├── frontend/                       # Dashboard Vue.js (Tabler CSS)
│   └── src/
│       ├── api/index.js            # Client API axios
│       ├── router/index.js         # Routes SPA
│       ├── stores/auth.js          # Store Pinia (auth + rôle)
│       └── views/
│           ├── DashboardView.vue
│           ├── HostDetailView.vue
│           ├── SystemView.vue      # Systemd / journal / processus
│           ├── DockerView.vue
│           ├── NetworkView.vue
│           ├── AptView.vue
│           ├── AuditLogsView.vue   # Commandes + Connexions
│           ├── AlertsView.vue
│           ├── SecurityView.vue
│           ├── SettingsView.vue
│           ├── UsersView.vue
│           └── AccountView.vue
├── .github/workflows/
│   ├── release.yml                 # Release multi-arch (agent + image Docker)
│   ├── ci-server.yml
│   ├── ci-agent.yml
│   └── ci-frontend.yml
├── docker-compose.yml
├── .env.example
└── README.md
```

---

## Licence

MIT
