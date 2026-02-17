# ServerSupervisor

Système de supervision d'infrastructure : monitoring de VMs, conteneurs Docker, mises à jour APT et suivi des releases GitHub.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Dashboard (Vue.js)                │
│  ┌──────────┐ ┌──────────┐ ┌────────┐ ┌─────────┐  │
│  │ Hosts    │ │ Docker   │ │  APT   │ │ Versions│  │
│  └──────────┘ └──────────┘ └────────┘ └─────────┘  │
├─────────────────────────────────────────────────────┤
│              Server Go (API REST + JWT)             │
│  ┌──────────┐ ┌──────────┐ ┌────────────────────┐  │
│  │ Auth     │ │ Rate     │ │ GitHub Release     │  │
│  │ (JWT+Key)│ │ Limiting │ │ Tracker            │  │
│  └──────────┘ └──────────┘ └────────────────────┘  │
├─────────────────────────────────────────────────────┤
│                  PostgreSQL                         │
└─────────────────────────────────────────────────────┘
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
- **Vue d'ensemble** : tous les hôtes avec statut temps réel (CPU, RAM, uptime)
- **Détail par hôte** : graphiques CPU/RAM historiques, disques, conteneurs, APT
- **Docker** : vue globale de tous les conteneurs sur toute l'infrastructure
- **APT** : gestion centralisée des mises à jour avec actions groupées
- **Versions** : suivi des releases GitHub et comparaison avec les images Docker en cours

### Agent
- Collecte automatique : CPU, RAM, disque, réseau, uptime
- Monitoring Docker via CLI (pas de SDK requis)
- Détection des mises à jour APT disponibles
- Exécution de commandes APT poussées depuis le dashboard
- Binaire unique sans dépendances

### Sécurité
- Authentification JWT pour le dashboard
- API Keys uniques par agent
- Rate limiting
- Support HTTPS

## Démarrage rapide

### 1. Déployer le serveur

```bash
# Cloner le repo
git clone <repo-url> && cd ServerSupervisor

# Configurer
cp .env.example .env
# Éditer .env avec vos valeurs (JWT_SECRET, ADMIN_PASSWORD, etc.)

# Lancer
docker compose up -d
```

Le dashboard est accessible sur `http://localhost:8080` (login: admin/admin par défaut).

### 2. Enregistrer un hôte

1. Ouvrir le dashboard → **Ajouter un hôte**
2. Renseigner hostname, IP, OS
3. **Copier la clé API** affichée (elle ne sera plus visible)

### 3. Installer l'agent sur une VM

```bash
# Compiler l'agent (depuis la machine de dev)
cd agent
GOOS=linux GOARCH=amd64 go build -o serversupervisor-agent ./cmd/agent

# Copier sur la VM cible
scp serversupervisor-agent user@vm:/usr/local/bin/

# Sur la VM : créer la config
sudo mkdir -p /etc/serversupervisor
sudo cat > /etc/serversupervisor/agent.yaml <<EOF
server_url: "http://your-server:8080"
api_key: "la-clé-api-copiée"
report_interval: 30
collect_docker: true
collect_apt: true
EOF

# Installer le service systemd
sudo cat > /etc/systemd/system/serversupervisor-agent.service <<EOF
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

# Démarrer
sudo systemctl daemon-reload
sudo systemctl enable --now serversupervisor-agent

# Vérifier
sudo journalctl -u serversupervisor-agent -f
```

Ou utiliser le script d'installation :
```bash
sudo bash agent/install.sh --server-url http://your-server:8080 --api-key your-key
```

### 4. Suivre des repos GitHub

1. Dashboard → **Versions & Repos**
2. Ajouter un repo (ex: `home-assistant` / `core`)
3. Optionnel : associer un nom d'image Docker pour la comparaison automatique
4. Le serveur vérifie les nouvelles releases toutes les 15 minutes

## Configuration

### Variables d'environnement serveur

| Variable | Description | Défaut |
|---|---|---|
| `SERVER_PORT` | Port du serveur | `8080` |
| `DB_HOST` | Hôte PostgreSQL | `localhost` |
| `DB_PASSWORD` | Mot de passe DB | `supervisor` |
| `JWT_SECRET` | Secret JWT (à changer !) | `change-me...` |
| `ADMIN_USER` | Utilisateur admin | `admin` |
| `ADMIN_PASSWORD` | Mot de passe admin | `admin` |
| `GITHUB_TOKEN` | Token GitHub (optionnel, augmente le rate limit) | `` |
| `GITHUB_POLL_INTERVAL` | Intervalle de vérification GitHub | `15m` |
| `METRICS_RETENTION_DAYS` | Rétention des métriques en jours | `30` |
| `RATE_LIMIT_RPS` | Requêtes par seconde max | `100` |

### Configuration agent (`agent.yaml`)

| Champ | Description | Défaut |
|---|---|---|
| `server_url` | URL du serveur | (requis) |
| `api_key` | Clé API de l'hôte | (requis) |
| `report_interval` | Intervalle d'envoi en secondes | `30` |
| `collect_docker` | Activer le monitoring Docker | `true` |
| `collect_apt` | Activer le monitoring APT | `true` |
| `insecure_skip_verify` | Ignorer les erreurs TLS | `false` |

## API REST

### Authentification
```bash
# Login (obtenir un token JWT)
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Utiliser le token
curl http://localhost:8080/api/v1/hosts \
  -H "Authorization: Bearer <token>"
```

### Endpoints principaux

| Méthode | Endpoint | Description |
|---|---|---|
| `POST` | `/api/auth/login` | Connexion |
| `GET` | `/api/v1/hosts` | Liste des hôtes |
| `POST` | `/api/v1/hosts` | Enregistrer un hôte |
| `GET` | `/api/v1/hosts/:id/dashboard` | Dashboard complet d'un hôte |
| `GET` | `/api/v1/hosts/:id/metrics/history` | Historique des métriques |
| `GET` | `/api/v1/docker/containers` | Tous les conteneurs |
| `GET` | `/api/v1/docker/versions` | Comparaison de versions |
| `GET/POST` | `/api/v1/repos` | Repos GitHub suivis |
| `POST` | `/api/v1/apt/command` | Envoyer une commande APT |
| `POST` | `/api/agent/report` | Réception rapport agent (API Key) |

## Développement

### Prérequis
- Go 1.22+
- Node.js 20+
- PostgreSQL 16+ (ou Docker)
- Docker & Docker Compose

### Développement local

```bash
# Terminal 1 : PostgreSQL
docker compose up postgres

# Terminal 2 : Serveur Go (avec hot-reload si air installé)
cd server
go run ./cmd/server

# Terminal 3 : Frontend Vue.js (avec proxy vers le serveur Go)
cd frontend
npm install
npm run dev
```

### Build

```bash
# Build complet via Docker
docker compose build

# Build agent pour Linux
cd agent
bash build.sh v1.0.0
```

## Structure du projet

```
ServerSupervisor/
├── server/                     # Serveur Go
│   ├── cmd/server/main.go      # Point d'entrée
│   ├── internal/
│   │   ├── api/                # Handlers HTTP (Gin)
│   │   ├── config/             # Configuration
│   │   ├── database/           # Couche PostgreSQL
│   │   ├── github/             # GitHub release tracker
│   │   └── models/             # Modèles de données
│   ├── Dockerfile
│   └── go.mod
├── agent/                      # Agent Go
│   ├── cmd/agent/main.go       # Point d'entrée
│   ├── internal/
│   │   ├── collector/          # Collecteurs (system, docker, apt)
│   │   ├── config/             # Configuration YAML
│   │   └── sender/             # Envoi des rapports
│   ├── build.sh                # Build multi-arch
│   ├── install.sh              # Script d'installation
│   └── go.mod
├── frontend/                   # Dashboard Vue.js
│   ├── src/
│   │   ├── api/                # Client API
│   │   ├── router/             # Routes
│   │   ├── stores/             # Pinia stores
│   │   └── views/              # Pages
│   └── package.json
├── docker-compose.yml
├── .env.example
└── README.md
```

## Licence

MIT
