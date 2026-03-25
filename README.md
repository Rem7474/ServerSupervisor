# ServerSupervisor

Système de supervision d'infrastructure : monitoring de VMs, conteneurs Docker, mises à jour APT, services systemd, tâches planifiées, suivi des releases GitHub et supervision Proxmox VE via API.

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                       Dashboard (Vue.js)                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐   │
│  │ Hosts    │ │ Docker   │ │ Network  │ │ APT Console    │   │
│  │ Dashboard│ │ Versions │ │Topology  │ │ Commandes      │   │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────┘   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐   │
│  │ Alertes  │ │ Audit    │ │ Users    │ │ System         │   │
│  │ (rules)  │ │Commandes │ │ (RBAC)   │ │ Systemd/Proc   │   │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────┘   │
│  ┌──────────────────────────┐ ┌─────────────────────────┐    │
│  │ Tâches planifiées (cron) │ │ Proxmox VE (nœuds/VMs)  │    │
│  └──────────────────────────┘ └─────────────────────────┘    │
├────────────────────────────────────────────────────────────────┤
│               Server Go (API REST + WebSocket + JWT)           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐   │
│  │ Auth+MFA │ │ Rate     │ │ Alert    │ │ Command        │   │
│  │ JWT+Keys │ │ Limiting │ │ Engine   │ │ Stream Hub     │   │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────┘   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐   │
│  │ Audit    │ │ GitHub   │ │ Settings │ │ Metrics        │   │
│  │ Logs     │ │ Tracker  │ │ (DB)     │ │ Aggregation    │   │
│  └──────────┘ └──────────┘ └──────────┘ └────────────────┘   │
│  ┌──────────────────────────┐ ┌─────────────────────────┐    │
│  │ Task Scheduler (cron)    │ │ Proxmox Poller (HTTP API)│    │
│  └──────────────────────────┘ └─────────────────────────┘    │
├────────────────────────────────────────────────────────────────┤
│                         PostgreSQL                             │
└────────────────────────────────────────────────────────────────┘
         ▲              ▲              ▲              ▲
    Push (30s)     Push (30s)     Push (30s)    Poll API PVE
         │              │              │              │
    ┌────┴────┐    ┌────┴────┐    ┌────┴────┐   ┌────┴──────┐
    │ Agent   │    │ Agent   │    │ Agent   │   │ Proxmox   │
    │ (Go)    │    │ (Go)    │    │ (Go)    │   │ VE API    │
    │ VM-1    │    │ VM-2    │    │ VM-N    │   │(pas agent)│
    └─────────┘    └─────────┘    └─────────┘   └───────────┘
```

## Fonctionnalités

### Dashboard
- **Vue d'ensemble** : tous les hôtes avec statut temps réel (CPU, RAM, uptime, version agent)
- **Détail par hôte** : graphiques CPU/RAM historiques (24h / 7j / 30j), disques, conteneurs, APT, historique de commandes toutes sources confondues
- **Docker** : vue globale de tous les conteneurs et projets docker-compose sur toute l'infrastructure
- **Network** : topologie réseau avec liens Docker (réseaux, env vars), override manuel des services
- **APT** : gestion centralisée des mises à jour avec actions groupées et console live streamée
- **Détail hôte** : exécution à distance de commandes systemd (start/stop/restart/enable/disable), logs journalctl streamés, snapshot des processus — directement depuis la page hôte
- **Streaming commandes** : affichage en temps réel de la sortie des commandes longues via WebSocket
- **Versions** : suivi des releases GitHub et comparaison avec les images Docker en cours
- **Audit → Commandes** : historique paginé de toutes les commandes (apt/docker/systemd/journal/processus), toutes sources
- **Audit → Connexions** : logs de connexion avec statistiques et IPs bloquées (admin)
- **Tâches planifiées** : création de tâches cron par hôte (apt, docker, systemd, journal, processus ou custom), déclenchement manuel immédiat, historique des exécutions
- **Alertes** : règles d'alertes configurables avec notifications email (SMTP), ntfy, webhook ou notifications navigateur
- **Compte → Sécurité** : gestion MFA/2FA du compte utilisateur sur `/account/security`
- **Sécurité (admin)** : analytics sécurité hôtes sur `/security` (connexions, IPs bloquées, détection bots/scans, NPM analytics)
- **UI cohérente** : barres de recherche/filtres/tri harmonisées sur les vues principales (Docker, APT, Audit)
- **Proxmox VE** : supervision de l'infrastructure de virtualisation via API Proxmox (sans agent sur l'hyperviseur) — nœuds, VMs QEMU, conteneurs LXC, stockage ; polling configurable par connexion

### Proxmox VE (supervision sans agent)
- Connexion à un ou plusieurs clusters / nœuds Proxmox via l'API REST officielle (token API)
- Collecte périodique configurable : nœuds (CPU, RAM, uptime, version PVE), VMs QEMU, conteneurs LXC, pools de stockage
- **Disques physiques** : liste, modèle, type (SSD/HDD/NVMe), santé SMART, usure SSD (`Sys.Audit` requis)
- **Mises à jour apt** : compteur de paquets en attente (pending/security), rafraîchissement du cache depuis le dashboard (`Sys.Modify` requis)
- **Tâches récentes** : 50 dernières tâches par nœud (vzdump, migration, création VM…)
- **Sauvegardes** : jobs configurés + dernier résultat de backup par VM (issu des tâches vzdump)
- **Liaison guest↔hôte** : détection automatique par nom, confirmation manuelle, sélection de la source de métriques (agent / proxmox / auto)
- UPSERT en base à chaque cycle + nettoyage automatique des ressources disparues
- Vue globale `/proxmox` : cartes de synthèse (connexions, nœuds, VMs, LXC, stockage) + alertes de santé + tableau des nœuds
- Vue détail `/proxmox/nodes/:id` : stats nœud + onglets VMs / LXC / Stockage / Disques / Tâches / Mises à jour
- Configuration dans **Paramètres** : ajout/édition/suppression de connexions, bouton **Tester** (sans sauvegarder), déclenchement manuel d'un poll
- Sécurité : `token_secret` stocké en base, jamais retourné au frontend ; `insecure_skip_verify` désactivé par défaut

### Agent
- Collecte automatique : CPU, RAM, disques, réseau, uptime
- Monitoring Docker via CLI (conteneurs, réseaux, projets compose, variables d'environnement)
- Détection des mises à jour APT disponibles, extraction des CVEs
- Collecte S.M.A.R.T. et métriques disques (via `smartctl`)
- Détection d'activité bot/scanner web (parsing access logs Nginx/Apache/httpd/NPM, top IPs/paths suspects)
- Collecte d'analytics NPM (style GoAccess) depuis les logs web : top domaines/hôtes, hits, bytes, erreurs 4xx/5xx
- Exécution de commandes distantes : APT, Docker/Compose, systemd, journalctl, snapshot processus
- **Tâches custom** : exécution de scripts/binaires locaux pré-déclarés dans `tasks.yaml` (allowlist, sans shell, sans exécution de code arbitraire distant)
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
collect_bot_detection: true
bot_detection_log_paths:
  - "/var/log/nginx/access.log"
  - "/var/log/apache2/access.log"
  - "/var/log/httpd/access_log"
  - "/data/logs/proxy-host-*.log"
bot_detection_tail_lines: 5000
bot_detection_top_n: 10
collect_npm_analytics: true
npm_analytics_log_paths:
  - "/var/log/nginx/access.log"
  - "/var/log/apache2/access.log"
  - "/var/log/httpd/access_log"
  - "/data/logs/proxy-host-*.log"
npm_analytics_tail_lines: 5000
npm_analytics_top_n: 10
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

### 4. Superviser un cluster Proxmox VE

1. Dans Proxmox, créer un token API avec les permissions minimales en lecture :
   ```
   # Rôle lecture seule (nœuds, VMs, LXC, stockage, disques)
   pveum role add SSAuditor -privs "Datastore.Audit Sys.Audit VM.Audit"
   pveum user add supervision@pve
   pveum aclmod / -user supervision@pve -role SSAuditor
   pveum user token add supervision@pve monitoring --privsep 0
   ```
   Copier le `token ID` (ex : `supervision@pve!monitoring`) et le `secret` affiché.

   > **Mises à jour apt (optionnel)** : l'endpoint `/nodes/{node}/apt/update` requiert `Sys.Modify`.
   > Si vous souhaitez voir les paquets en attente et rafraîchir le cache depuis le dashboard,
   > ajoutez ce privilege au rôle ou créez un second rôle complémentaire :
   > ```
   > pveum role modify SSAuditor -privs "Datastore.Audit Sys.Audit Sys.Modify VM.Audit"
   > ```
   > **Important** : si votre token a "Privilege Separation" activé (coché par défaut à la création),
   > les permissions doivent être assignées **directement au token** et pas seulement à l'utilisateur :
   > ```
   > pveum aclmod / -token supervision@pve!monitoring -role SSAuditor
   > ```

2. Dans ServerSupervisor → **Paramètres** → carte **Proxmox VE** → **Ajouter une connexion** :
   - Nom : label interne (ex : `Cluster prod`)
   - URL API : `https://pve.example.com:8006/api2/json`
   - Token ID : `supervision@pve!monitoring`
   - Token secret : le secret copié
   - Cocher **Ignorer TLS** uniquement si le certificat est auto-signé

3. Cliquer **Tester la connexion** pour valider, puis **Créer**.

4. La première collecte démarre automatiquement. L'entrée **Proxmox** apparaît dans la navigation.

### 5. Suivre des repos GitHub

1. Dashboard → **Git / Automatisation** → onglet **Suivi de releases**
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
| `collect_smart` | Activer la collecte S.M.A.R.T. | `true` | `SUPERVISOR_COLLECT_SMART` |
| `collect_bot_detection` | Activer la détection bot/scanner depuis les logs web | `true` | `SUPERVISOR_COLLECT_BOT_DETECTION` |
| `bot_detection_log_paths` | Liste de paths/globs de logs access à parser | voir exemple | `SUPERVISOR_BOT_DETECTION_LOG_PATHS` |
| `bot_detection_tail_lines` | Nombre de lignes lues (par fichier) | `5000` | `SUPERVISOR_BOT_DETECTION_TAIL_LINES` |
| `bot_detection_top_n` | Nombre max d'IP/paths retournés | `10` | `SUPERVISOR_BOT_DETECTION_TOP_N` |
| `collect_npm_analytics` | Activer la collecte analytics NPM depuis les logs web | `true` | `SUPERVISOR_COLLECT_NPM_ANALYTICS` |
| `npm_analytics_log_paths` | Liste de paths/globs de logs access à parser pour NPM analytics | voir exemple | `SUPERVISOR_NPM_ANALYTICS_LOG_PATHS` |
| `npm_analytics_tail_lines` | Nombre de lignes lues (par fichier) pour NPM analytics | `5000` | `SUPERVISOR_NPM_ANALYTICS_TAIL_LINES` |
| `npm_analytics_top_n` | Nombre max de domaines retournés dans les analytics | `10` | `SUPERVISOR_NPM_ANALYTICS_TOP_N` |
| `apt_auto_update_on_start` | Lancer `apt update` au démarrage de l'agent | `false` | `SUPERVISOR_APT_AUTO_UPDATE_ON_START` |
| `insecure_skip_verify` | Ignorer les erreurs TLS (certificats auto-signés) | `false` | `SUPERVISOR_INSECURE_SKIP_VERIFY` |

> Toutes les options sont également configurables via variables d'environnement (préfixe `SUPERVISOR_`), utile pour les déploiements Docker/Kubernetes.

### Bot detection (logs web)

L'agent peut analyser les access logs web pour identifier des comportements de scan automatisé.

Détection actuelle :
- chemins sensibles fréquemment scannés (`/.env`, `wp-admin`, `phpmyadmin`, etc.)
- user-agents typiques d'outils de scan (`masscan`, `sqlmap`, `nikto`, etc.)
- méthodes HTTP atypiques (`TRACE`, `PROPFIND`, ...)

Agrégations remontées :
- `top_suspicious_ips`
- `top_suspicious_paths`
- `suspicious_requests`

Affichage :
- onglet **Sécurité → Menaces** (agrégation globale multi-hôtes)

API :
- `GET /api/v1/auth/security` inclut un champ `bot_detection` pour les admins.

### NPM analytics (logs web)

L'agent peut également agréger les access logs pour remonter des statistiques de trafic web façon "GoAccess".

Payload remonté par hôte :
- `total_requests`
- `total_bytes`
- `top_domains` (avec `domain`, `hits`, `bytes`, `errors_4xx`, `errors_5xx`)

Affichage :
- page **Sécurité** (`/security`) côté admin, section analytics hôtes

API :
- `GET /api/v1/auth/security` inclut aussi un champ `npm_analytics` (agrégation multi-hôtes pour les admins)

### Tâches custom (`tasks.yaml`)

Les tâches custom permettent de définir localement sur l'agent des scripts ou binaires déclenchables depuis le serveur. Le serveur ne peut qu'appeler une tâche par son ID — il n'envoie jamais de code arbitraire.

Chemin par défaut : `/etc/serversupervisor/tasks.yaml` (override : variable `TASKS_CONFIG_PATH`)

```yaml
tasks:
  - id: cleanup_logs
    name: "Nettoyer les vieux logs"
    command: ["find", "/var/log", "-name", "*.log", "-mtime", "+30", "-delete"]
    timeout: 120          # secondes (défaut 60, max 3600)

  - id: backup_db
    name: "Backup PostgreSQL"
    command: ["pg_dump", "-U", "postgres", "mydb", "-f", "/backups/db.sql"]
    timeout: 300

  # Déclenchable via Git Webhook — reçoit les variables SS_BRANCH, SS_COMMIT_SHA, etc.
  - id: git-pull-test
    name: "Git pull /home/root/test"
    command: ["bash", "-c", "cd /home/root/test && git pull origin ${SS_BRANCH:-main}"]
    timeout: 60

  # Alternative : déléguer à un script shell pour plus de contrôle
  - id: deploy-test
    name: "Deploy /home/root/test"
    command: ["/opt/scripts/deploy-test.sh"]
    timeout: 120
```

Pour l'option script shell, `/opt/scripts/deploy-test.sh` :
```bash
#!/bin/bash
set -e
cd /home/root/test
git pull origin ${SS_BRANCH:-main}
echo "Déploiement terminé (commit: $SS_COMMIT_SHA)"
```

Variables d'environnement injectées automatiquement par un Git Webhook :

| Variable | Contenu |
|---|---|
| `SS_REPO_NAME` | Nom du dépôt (`owner/repo`) |
| `SS_BRANCH` | Branche poussée |
| `SS_COMMIT_SHA` | SHA du dernier commit |
| `SS_COMMIT_MESSAGE` | Message du commit |
| `SS_PUSHER` | Auteur du push |
| `SS_WEBHOOK_NAME` | Nom du webhook ServerSupervisor |
| `SS_EVENT_TYPE` | Type d'événement (`push`, `tag`, `release`) |

| Champ | Description |
|---|---|
| `id` | Identifiant unique (alphanumérique + `-` + `_`, max 64 chars) |
| `name` | Nom affiché dans le dashboard |
| `command` | Argv (tableau) — exécuté directement, **sans shell**, pas d'injection possible |
| `timeout` | Timeout en secondes (défaut 60, max 3600) |

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
| `GET` | `/api/v1/auth/security` | Résumé sécurité + IPs bloquées + agrégats `bot_detection` et `npm_analytics` | Admin |
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

Métriques additionnelles disponibles pour les règles d'alertes :
- `npm_requests`
- `npm_traffic_bytes`
- `npm_5xx_errors`

#### Utilisateurs (admin)
| Méthode | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/users` | Liste des utilisateurs |
| `POST` | `/api/v1/users` | Créer un utilisateur |
| `PATCH` | `/api/v1/users/:id/role` | Changer le rôle (`admin`/`operator`/`viewer`) |
| `DELETE` | `/api/v1/users/:id` | Supprimer un utilisateur |

#### Git Webhooks & Suivi de releases
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET/POST` | `/api/v1/webhooks/git` | Webhooks Git | Admin |
| `GET/PUT/DELETE` | `/api/v1/webhooks/git/:id` | Détail / modification / suppression | Admin |
| `POST` | `/api/v1/webhooks/git/:id/regenerate-secret` | Regénérer le secret HMAC | Admin |
| `GET` | `/api/v1/webhooks/git/:id/executions` | Historique exécutions | Admin |
| `POST` | `/api/v1/webhooks/git/:id/receive` | Réception webhook (public, HMAC) | Public |
| `GET/POST` | `/api/v1/release-trackers` | Suivi releases GitHub/GitLab | Admin |
| `GET/PUT/DELETE` | `/api/v1/release-trackers/:id` | Détail / modification / suppression | Admin |
| `POST` | `/api/v1/release-trackers/:id/check-now` | Vérification immédiate | Admin |
| `POST` | `/api/v1/release-trackers/:id/run` | Déclencher manuellement | Admin |
| `GET` | `/api/v1/release-trackers/:id/executions` | Historique exécutions | Admin |

#### Tâches planifiées
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/scheduled-tasks` | Lister les tâches d'un hôte | Authentifié |
| `POST` | `/api/v1/hosts/:id/scheduled-tasks` | Créer une tâche planifiée | Operator+ |
| `PUT` | `/api/v1/scheduled-tasks/:id` | Modifier une tâche | Operator+ |
| `DELETE` | `/api/v1/scheduled-tasks/:id` | Supprimer une tâche | Operator+ |
| `POST` | `/api/v1/scheduled-tasks/:id/run` | Déclencher manuellement | Operator+ |

#### Proxmox VE
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/proxmox/summary` | Compteurs globaux (nœuds, VMs, LXC, stockage) | Authentifié |
| `GET` | `/api/v1/proxmox/nodes` | Tous les nœuds (`?connection_id=` optionnel) | Authentifié |
| `GET` | `/api/v1/proxmox/nodes/:id` | Détail nœud avec guests + stockages | Authentifié |
| `GET` | `/api/v1/proxmox/guests` | Tous les guests (`?type=vm\|lxc`, `?status=running`) | Authentifié |
| `GET` | `/api/v1/proxmox/instances` | Liste des connexions (sans secrets) | Authentifié |
| `POST` | `/api/v1/proxmox/instances` | Créer une connexion | Admin |
| `GET` | `/api/v1/proxmox/instances/:id` | Détail d'une connexion | Admin |
| `PUT` | `/api/v1/proxmox/instances/:id` | Modifier une connexion | Admin |
| `DELETE` | `/api/v1/proxmox/instances/:id` | Supprimer une connexion | Admin |
| `POST` | `/api/v1/proxmox/instances/test` | Tester sans sauvegarder | Admin |
| `POST` | `/api/v1/proxmox/instances/:id/test` | Tester une connexion existante | Admin |
| `POST` | `/api/v1/proxmox/instances/:id/poll-now` | Déclencher un poll immédiat | Admin |
| `POST` | `/api/v1/proxmox/nodes/:id/apt-refresh` | Déclencher `apt update` sur le nœud (Sys.Modify requis) | Admin |
| `GET` | `/api/v1/proxmox/tasks` | Toutes les tâches récentes (`?connection_id=`) | Authentifié |
| `GET` | `/api/v1/proxmox/nodes/:id/tasks` | Tâches d'un nœud | Authentifié |
| `GET` | `/api/v1/proxmox/nodes/:id/disks` | Disques physiques d'un nœud | Authentifié |
| `GET` | `/api/v1/proxmox/backup-jobs` | Configurations des jobs de sauvegarde | Authentifié |
| `GET` | `/api/v1/proxmox/backup-runs` | Derniers résultats de sauvegarde par VM | Authentifié |
| `GET` | `/api/v1/proxmox/links` | Liens guest↔hôte (`?status=`) | Authentifié |
| `POST` | `/api/v1/proxmox/links` | Créer/remplacer un lien | Admin |
| `GET/PUT/DELETE` | `/api/v1/proxmox/links/:id` | Détail / modification / suppression d'un lien | Admin |

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
│       │   ├── router.go           # Routes & middleware (retourne ReleaseTrackerHandler + ProxmoxHandler)
│       │   ├── auth.go             # Auth + MFA + login events
│       │   ├── agent.go            # API agent (rapport, commandes, audit)
│       │   ├── audit.go            # Audit logs + historique commandes
│       │   ├── hosts.go            # Gestion hôtes + disques
│       │   ├── docker.go           # Docker + Compose
│       │   ├── system.go           # Systemd + journal + processus
│       │   ├── apt.go              # APT management
│       │   ├── scheduled_tasks.go  # Tâches planifiées (CRUD + run)
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
│       │   ├── db_proxmox.go       # CRUD + upsert Proxmox (connexions, nœuds, guests, storages)
│       │   ├── db_scheduled_tasks.go
│       │   ├── db_webhooks.go
│       │   ├── db_release_trackers.go
│       │   └── migrations/         # Migrations SQL (001 → 027)
│       ├── github/                 # GitHub release tracker
│       ├── handlers/
│       │   ├── proxmox.go          # ProxmoxHandler : CRUD connexions + poller (30s tick)
│       │   ├── release_trackers.go
│       │   └── git_webhooks.go
│       ├── proxmoxclient/
│       │   └── client.go           # Client HTTP Proxmox VE (PVEAPIToken, TLS, endpoints)
│       ├── scheduler/scheduler.go  # Scheduler cron (robfig/cron)
│       └── models/
│           ├── proxmox.go          # ProxmoxConnection, ProxmoxNode, ProxmoxGuest, ProxmoxStorage
│           └── models.go           # Autres modèles partagés
├── agent/                          # Agent Go (déployé dans les VMs — pas sur Proxmox)
│   ├── cmd/agent/main.go
│   └── internal/
│       ├── collector/              # system.go, docker.go, apt.go, disk.go…
│       ├── config/
│       │   ├── config.go           # Config YAML + env vars
│       │   └── tasks.go            # Loader tasks.yaml (tâches custom)
│       └── sender/sender.go        # Envoi rapports + commandes
├── frontend/                       # Dashboard Vue.js (Tabler CSS)
│   └── src/
│       ├── api/index.js            # Client API axios (inclut méthodes Proxmox)
│       ├── router/index.js         # Routes SPA
│       ├── stores/auth.js          # Store Pinia (auth + rôle)
│       ├── components/settings/
│       │   ├── SettingsProxmoxCard.vue  # Gestion connexions Proxmox (admin)
│       │   └── …                        # Autres cartes Settings
│       └── views/
│           ├── DashboardView.vue
│           ├── HostDetailView.vue       # Dashboard hôte (métriques, docker, systemd, journal, processus)
│           ├── ProxmoxView.vue          # Vue globale Proxmox (/proxmox)
│           ├── ProxmoxNodeView.vue      # Détail nœud (/proxmox/nodes/:id)
│           ├── DockerView.vue
│           ├── NetworkView.vue
│           ├── AptView.vue
│           ├── AuditLogsView.vue        # Commandes + Connexions
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
