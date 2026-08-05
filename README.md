# ServerSupervisor

Système de supervision d'infrastructure : monitoring de VMs, conteneurs Docker, mises à jour APT, services systemd, tâches planifiées, suivi des releases GitHub, supervision Proxmox VE via API et monitoring synthétique (sondes uptime, certificats SSL, intégration Nginx Proxy Manager).

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
│  ┌──────────────────────────┐ ┌─────────────────────────┐    │
│  │ NPM / Uptime / SSL       │ │ Notifications (Push)    │    │
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
│                  TimescaleDB (PostgreSQL 16)                   │
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
- **Monitoring** : sondes HTTP/TCP synthétiques (uptime) et suivi d'expiration des certificats SSL/TLS, historique et stats par sonde sur `/monitoring`
- **Audit → Commandes** : historique paginé de toutes les commandes (apt/docker/systemd/journal/processus), toutes sources
- **Audit → Connexions** : logs de connexion avec statistiques et IPs bloquées (admin)
- **Tâches planifiées** : création de tâches cron par hôte (apt, docker, systemd, journal, processus ou custom), déclenchement manuel immédiat, historique des exécutions
- **Alertes** : règles d'alertes configurables avec notifications email (SMTP), ntfy, webhook ou notifications navigateur
- **Notifications** : centre de notifications in-app sur `/notifications` + push navigateur (Web Push/VAPID), en complément des canaux SMTP/ntfy/webhook des alertes
- **Compte → Sécurité** : gestion MFA/2FA du compte utilisateur sur `/account/security`
- **Sécurité (admin)** : analytics sécurité hôtes sur `/security` (connexions, IPs bloquées, corrélation CrowdSec si activée côté agent), stats trafic web sur `/traffic`, menaces web sur `/threats`
- **UI cohérente** : barres de recherche/filtres/tri harmonisées sur les vues principales (Docker, APT, Audit)
- **Proxmox VE** : supervision de l'infrastructure de virtualisation via API Proxmox (sans agent sur l'hyperviseur) — nœuds, VMs QEMU, conteneurs LXC, stockage ; polling configurable par connexion
- **NPM (Nginx Proxy Manager)** : connexion à une ou plusieurs instances NPM, import sélectif de proxy hosts, création automatique des sondes uptime/certificats SSL correspondants, activation du monitoring par host

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
- Vue détail `/proxmox/nodes/:id` : stats nœud + onglets VMs / LXC / Stockage / Disques / Tâches / Sauvegardes / Mises à jour / Services / Journaux sécurité
- Configuration dans **Paramètres** : ajout/édition/suppression de connexions, bouton **Tester** (sans sauvegarder), déclenchement manuel d'un poll
- Sécurité : `token_secret` stocké en base, jamais retourné au frontend ; `insecure_skip_verify` désactivé par défaut

### NPM (Nginx Proxy Manager)
- Connexion à une ou plusieurs instances [Nginx Proxy Manager](https://nginxproxymanager.com/) via son API REST (identity + secret)
- Import sélectif des proxy hosts existants (modal à cocher) — pas de découverte automatique périodique ; seul un rafraîchissement léger (`npm_enabled`, `last_seen_at`) tourne en arrière-plan sur les hosts déjà importés
- Pour chaque proxy host importé : création automatique d'une sonde uptime HTTP et d'un certificat SSL suivi (voir Monitoring), avec un interrupteur maître + deux sous-interrupteurs (uptime / SSL) par host
- Vue `/npm` : gestion des connexions (admin) + liste des proxy hosts importés
- Sécurité : `secret` stocké en base, jamais retourné au frontend (même modèle que le `token_secret` Proxmox)

### Agent
- Collecte automatique : CPU, RAM, disques, réseau, uptime
- Monitoring Docker via CLI (conteneurs, réseaux, projets compose, variables d'environnement)
- Détection des mises à jour APT disponibles, extraction des CVEs
- Collecte S.M.A.R.T. et métriques disques (via `smartctl`)
- Collecte web logs unifiée (Nginx/Apache/httpd/NPM) : trafic + menaces en un seul parsing
- Ingestion incrémentale des logs web via cursor persistant (évite de relire les mêmes lignes à chaque cycle)
- **Corrélation CrowdSec** (optionnelle, désactivée par défaut) : rapproche le trafic web collecté des décisions actives de l'API locale CrowdSec (bans/captcha) — nécessite `collect_web_logs: true` et une clé bouncer CrowdSec
- Exécution de commandes distantes : APT, Docker/Compose, systemd, journalctl, snapshot processus
- **Tâches custom** : exécution de scripts/binaires locaux pré-déclarés dans `tasks.yaml` (allowlist, sans shell, sans exécution de code arbitraire distant)
- **Sauvegardes Restic** (optionnelle) : supervision passive de l'état Restic local + déclenchement de backup à la demande ou planifié, sans jamais faire remonter les credentials au serveur (voir [Sauvegardes Restic](#sauvegardes-restic))
- Streaming temps réel de la sortie des commandes longues (chunk par chunk)
- Rapport de résultat des commandes autonomes au démarrage (ex: `apt update`)
- Binaire unique sans dépendances, multi-architecture (amd64/arm64/armv7/armv6)

### Sécurité
- Authentification JWT avec refresh tokens
- MFA/2FA optionnel par compte : TOTP et/ou clés de sécurité/passkeys (WebAuthn)
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
git clone https://github.com/Rem7474/ServerSupervisor.git && cd ServerSupervisor
cp .env.example .env
# Éditer .env avec vos valeurs (JWT_SECRET, ADMIN_PASSWORD, etc.)
docker compose up -d
```

Le dashboard est accessible sur `http://localhost:8080` (login: `admin` / `admin` par défaut).

### 2. Enregistrer un hôte

1. Dashboard → **Ajouter un hôte**
2. Renseigner le nom, hostname/IP, OS
3. **Copier la clé API** affichée (elle ne sera plus visible ensuite)

### 3. Installer l'agent sur une VM

#### Installation en une commande (recommandé)

Après avoir enregistré un hôte dans le dashboard, copiez la commande affichée (URL serveur et clé API déjà injectées) :

```bash
curl -sSL https://raw.githubusercontent.com/Rem7474/ServerSupervisor/main/agent/install.sh | sudo bash -s -- --server-url http://your-server:8080 --api-key your-api-key
```

Le script détecte l'architecture (`amd64` / `arm64` / `arm`), télécharge le binaire depuis les releases GitHub, génère `/etc/serversupervisor/agent.yaml` (permissions 0600) et active le service systemd immédiatement.

Options supplémentaires :
```
--interval <sec>   Intervalle de rapport (défaut: 30)
--no-docker        Désactiver le monitoring Docker
--no-apt           Désactiver le monitoring APT
```

#### Prérequis agent (packages système)

L'agent est un binaire Go statique, mais certaines fonctionnalités s'appuient sur des outils système présents sur la VM/LXC.

| Fonctionnalité agent | Binaire / package requis | Obligatoire |
|---|---|---|
| Exécution de l'agent | `ca-certificates` | Oui |
| Monitoring Docker (`collect_docker`) + commandes Docker | `docker` / `docker-cli` | Oui si Docker activé |
| Monitoring APT (`collect_apt`) + actions APT | `apt`, `apt-get` | Oui sur Debian/Ubuntu |
| SMART disques (`collect_smart`) | `smartctl` (`smartmontools`) | Oui si SMART activé |
| Température CPU (`collect_cpu_temperature`) | `/sys/class/thermal` ou `/sys/class/hwmon`, fallback `sensors` (`lm-sensors`) | Oui si température CPU activée |
| Commandes système (services/logs) | `systemctl`, `journalctl` (systemd) | Recommandé |
| Snapshot processus | `ps` (`procps`) | Recommandé |
| Sauvegardes Restic (`collect_restic`) | `restic`, éventuellement `resticprofile` — toolkit installé/configuré séparément (non fourni par ServerSupervisor) | Oui si Restic activé |

Exemple Debian/Ubuntu:

```bash
sudo apt update
sudo apt install -y ca-certificates curl procps

# Optionnels selon les features activées
sudo apt install -y docker.io        # si collect_docker: true
sudo apt install -y smartmontools    # si collect_smart: true
sudo apt install -y lm-sensors       # si collect_cpu_temperature: true
```

Exemple RHEL/Alma/Rocky:

```bash
sudo dnf install -y ca-certificates curl procps-ng

# Optionnels selon les features activées
sudo dnf install -y docker-cli       # si collect_docker: true
sudo dnf install -y smartmontools    # si collect_smart: true
sudo dnf install -y lm_sensors       # si collect_cpu_temperature: true
```

Notes:
- Pour Docker, l'utilisateur du service agent doit avoir accès au socket Docker (groupe `docker` ou équivalent).
- Sur certains environnements virtualisés, la température CPU peut être absente même avec `lm-sensors`.

#### Via les releases GitHub (manuel)

```bash
# Remplacer ARCH par : amd64, arm64, arm
curl -fsSL https://github.com/Rem7474/ServerSupervisor/releases/latest/download/serversupervisor-agent-linux-ARCH \
  -o /usr/local/bin/serversupervisor-agent
chmod +x /usr/local/bin/serversupervisor-agent

sudo /usr/local/bin/serversupervisor-agent --init \
  --config /etc/serversupervisor/agent.yaml \
  --server-url http://your-server:8080 \
  --api-key your-key
```

#### Manuellement

```bash
# Compiler l'agent (depuis la machine de dev)
cd agent
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o serversupervisor-agent ./cmd/agent
scp serversupervisor-agent user@vm:/usr/local/bin/

# Sur la VM : générer la config (écrit le fichier en 0600)
sudo /usr/local/bin/serversupervisor-agent --init \
  --config /etc/serversupervisor/agent.yaml \
  --server-url http://your-server:8080 \
  --api-key la-cle-api-copiee

# Si le fichier existe déjà et doit être écrasé
# sudo /usr/local/bin/serversupervisor-agent --init --init-force \
#   --config /etc/serversupervisor/agent.yaml --server-url ... --api-key ...

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
| `APP_ENV` | `dev`/`development` assouplit la validation stricte des secrets (JWT auto-généré) ; toute autre valeur = production stricte | `production` |
| `LOG_LEVEL` | Niveau de log (`debug`/`info`/`warn`/`error`) | `info` |
| `LOG_FORMAT` | Format de log (`json`/`text`) | `text` en dev, `json` sinon |

#### Base de données
| Variable | Description | Défaut |
|---|---|---|
| `DB_HOST` | Hôte PostgreSQL | `localhost` |
| `DB_PORT` | Port PostgreSQL | `5432` |
| `DB_USER` | Utilisateur | `supervisor` |
| `DB_PASSWORD` | Mot de passe **(à changer !)** | `supervisor` |
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

### Sauvegarde & restauration

Le stack Docker Compose inclut un service `postgres-backup` (image
[`prodrigestivill/postgres-backup-local`](https://github.com/prodrigestivill/docker-postgres-backup-local))
qui exécute des `pg_dump` planifiés, compressés et rotés, de la base
ServerSupervisor. C'est une sauvegarde au sens disaster recovery — elle
protège contre la perte du volume Docker, une corruption, ou une erreur de
manipulation. Ce n'est **pas** la même chose que `METRICS_RETENTION_DAYS` /
les politiques de rétention TimescaleDB, qui ne font qu'expirer les anciennes
métriques dans une base par ailleurs saine.

| Variable | Description | Défaut |
|---|---|---|
| `BACKUP_SCHEDULE` | Planification (`@daily`, `@weekly`, ou cron 5 champs) | `@daily` |
| `BACKUP_KEEP_DAYS` | Sauvegardes quotidiennes conservées | `7` |
| `BACKUP_KEEP_WEEKS` | Sauvegardes hebdomadaires conservées | `4` |
| `BACKUP_KEEP_MONTHS` | Sauvegardes mensuelles conservées | `6` |

Les fichiers `.sql.gz` sont écrits dans le volume nommé `postgres_backups`
(`/backups` dans le conteneur `postgres-backup`).

**Restaurer une sauvegarde** (arrête l'API pendant la restauration ; adapter
le nom de fichier) :

```bash
# 1. Arrêter le serveur pour éviter des écritures pendant la restauration
docker compose stop server

# 2. Copier la sauvegarde choisie hors du volume
docker compose cp postgres-backup:/backups/daily/serversupervisor-<horodatage>.sql.gz ./restore.sql.gz
gunzip restore.sql.gz

# 3. Recréer une base vide (la restauration part d'un schéma propre)
docker compose exec postgres psql -U supervisor -d postgres -c "DROP DATABASE serversupervisor;"
docker compose exec postgres psql -U supervisor -d postgres -c "CREATE DATABASE serversupervisor OWNER supervisor;"

# 4. Importer le dump
cat restore.sql | docker compose exec -T postgres psql -U supervisor -d serversupervisor

# 5. Redémarrer le serveur
docker compose start server
```

> Testez cette procédure au moins une fois avant d'en avoir besoin en
> production — une sauvegarde qui n'a jamais été restaurée n'est pas
> vérifiée. Les noms de fichiers exacts (sous-dossier `daily/weekly/monthly`)
> sont listés par `docker compose exec postgres-backup ls -la /backups`.

### Revenir en arrière après une mise à jour ratée (rollback manuel de migration)

Les migrations SQL (`server/internal/database/migrations/`) s'appliquent
automatiquement au démarrage et sont **forward-only** — il n'existe pas de
mécanisme de "migration down" intégré. Avant de mettre à jour vers une
version qui embarque de nouvelles migrations, prenez un instantané ad-hoc et
gardez cette procédure à portée de main.

**Avant la mise à jour :**

```bash
docker compose exec postgres pg_dump -Fc -U supervisor serversupervisor -f /tmp/pre-upgrade.dump
docker compose cp postgres:/tmp/pre-upgrade.dump ./pre-upgrade.dump
```

**Si la nouvelle version pose problème :**

```bash
# 1. Revenir à l'image/tag précédent dans docker-compose.yml, puis arrêter le serveur
docker compose stop server

# 2. Recréer une base vide
docker compose cp ./pre-upgrade.dump postgres:/tmp/pre-upgrade.dump
docker compose exec postgres psql -U supervisor -d postgres -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='serversupervisor' AND pid <> pg_backend_pid();"
docker compose exec postgres psql -U supervisor -d postgres -c "DROP DATABASE serversupervisor;"
docker compose exec postgres psql -U supervisor -d postgres -c "CREATE DATABASE serversupervisor OWNER supervisor;"

# 3. Restaurer l'instantané pré-mise à jour
docker compose exec postgres pg_restore -U supervisor -d serversupervisor /tmp/pre-upgrade.dump

# 4. Redémarrer sur l'ancienne image
docker compose up -d server
```

> **Cette procédure a été testée de bout en bout** (pas seulement rédigée en
> théorie) : schéma complet migré (baseline + les 71 migrations), une
> migration destructrice simulée (colonne supprimée, lignes effacées), puis
> `DROP DATABASE` / `CREATE DATABASE` / `pg_restore` depuis l'instantané —
> l'état obtenu (schéma, `schema_migrations`, données) est identique bit à
> bit à celui du moment de l'instantané. Cette validation a été faite sur un
> PostgreSQL 16 nu (hors conteneur, sans l'extension TimescaleDB
> disponible dans cet environnement de test) : le mécanisme `pg_dump`/
> `pg_restore` lui-même est donc vérifié, mais les commandes `docker compose
> exec` ci-dessus n'ont pas pu être rejouées telles quelles faute de démon
> Docker disponible pendant cette validation. Testez-la une fois sur votre
> propre stack avant d'en dépendre en production.

---

### Configuration agent (`agent.yaml`)

Cette configuration est identique quel que soit le mode d'installation de l'agent (release GitHub, build manuel, ou script d'installation).

Initialiser une config (écriture fichier) :
```bash
serversupervisor-agent --init \
  --config /etc/serversupervisor/agent.yaml \
  --server-url http://your-server:8080 \
  --api-key your-key
```

Comportement de `--init` :
- Écrit la config sur le chemin passé via `--config` (défaut: `/etc/serversupervisor/agent.yaml`)
- Refuse d'écraser un fichier existant sauf avec `--init-force`
- Crée automatiquement le dossier parent si nécessaire
- Écrit le fichier avec permissions `0600`
- Mode compatibilité stdout: `--config -`

| Champ | Description | Défaut | Variable d'env |
|---|---|---|---|
| `server_url` | URL du serveur | `http://localhost:8080` | `SUPERVISOR_SERVER_URL` |
| `api_key` | Clé API de l'hôte **(requis)** | — | `SUPERVISOR_API_KEY` |
| `report_interval` | Intervalle d'envoi en secondes | `30` | `SUPERVISOR_REPORT_INTERVAL` |
| `max_report_body_bytes` | Taille max du payload JSON envoyé (bytes) | `3145728` | `SUPERVISOR_MAX_REPORT_BODY_BYTES` |
| `collect_docker` | Activer le monitoring Docker | `true` | `SUPERVISOR_COLLECT_DOCKER` |
| `collect_apt` | Activer le monitoring APT | `true` | `SUPERVISOR_COLLECT_APT` |
| `collect_smart` | Activer la collecte S.M.A.R.T. | `false` | `SUPERVISOR_COLLECT_SMART` |
| `collect_cpu_temperature` | Activer la collecte de température CPU | `false` | `SUPERVISOR_COLLECT_CPU_TEMPERATURE` |
| `collect_web_logs` | Activer l'analyse unifiée des logs web | `false` | `SUPERVISOR_COLLECT_WEB_LOGS` |
| `web_logs_log_paths` | Liste de paths/globs de logs access à parser | voir exemple | `SUPERVISOR_WEB_LOGS_LOG_PATHS` |
| `web_logs_tail_lines` | Nombre de lignes lues (par fichier) | `5000` | `SUPERVISOR_WEB_LOGS_TAIL_LINES` |
| `web_logs_top_n` | Nombre max d'IP/domaines/paths retournés | `10` | `SUPERVISOR_WEB_LOGS_TOP_N` |
| `web_logs_requests_limit` | Nombre max de requêtes brutes envoyées | `200` | `SUPERVISOR_WEB_LOGS_REQUESTS_LIMIT` |
| `web_logs_cursor_file` | Fichier de cursor incrémental web logs | `/var/lib/serversupervisor/web_logs_cursor.json` | `SUPERVISOR_WEB_LOGS_CURSOR_FILE` |
| `apt_auto_update_on_start` | Lancer `apt update` au démarrage de l'agent | `false` | `SUPERVISOR_APT_AUTO_UPDATE_ON_START` |
| `insecure_skip_verify` | Ignorer les erreurs TLS (certificats auto-signés) | `false` | `SUPERVISOR_INSECURE_SKIP_VERIFY` |

> Toutes les options sont également configurables via variables d'environnement (préfixe `SUPERVISOR_`), utile pour les déploiements Docker/Kubernetes.
>
> Compatibilité: les anciennes variables `SUPERVISOR_COLLECT_BOT_DETECTION`, `SUPERVISOR_COLLECT_NPM_ANALYTICS` et leurs variantes `*_LOG_PATHS`, `*_TAIL_LINES`, `*_TOP_N` restent supportées comme alias hérités.

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

### Sauvegardes Restic

Supervision optionnelle des sauvegardes [Restic](https://restic.net)/[resticprofile](https://creativeprojects.github.io/resticprofile/) sur les hôtes supervisés. Le toolkit Restic (binaire, `resticconf`, `run_backup.sh`, `resticprofile.yaml`) doit déjà être installé et configuré sur la machine — ServerSupervisor ne l'installe pas et ne stocke **jamais** ses credentials (mot de passe du dépôt, clés Swift/S3/B2, identifiants SMTP) : ils restent dans `resticconf` sur l'hôte, lu localement par l'agent et jamais transmis au serveur.

#### Activer la collecte (`agent.yaml`)

```yaml
collect_restic: true
restic_bin: "/usr/local/bin/restic"
restic_conf_path: "/home/user/restic-backups/resticconf"
restic_run_script_path: "/home/user/restic-backups/run_backup.sh"
restic_status_file_path: "/home/user/restic-backups/backup-status.json"
restic_enable_progress: true
restic_progress_fps: 0.1
restic_backup_idle_timeout_minutes: 20
```

Seuls des chemins et des indicateurs de fonctionnalité vivent ici — jamais un secret. `restic_status_file_path` pointe vers le status-file JSON de resticprofile (`status-file: ...` dans `resticprofile.yaml`, idéalement avec `extended-status: true`) : c'est la source privilégiée pour le monitoring passif ; sans ce fichier, l'agent retombe sur `restic snapshots --json` / `restic stats --json`.

#### Monitoring passif vs déclenchement actif

- **Passif** : à chaque rapport périodique, l'agent lit l'état Restic local (status-file ou fallback commandes) et le remonte au serveur — visible dans l'onglet **Sauvegardes** de la fiche hôte, sans qu'aucun backup n'ait été déclenché par ServerSupervisor.
- **Actif** : le bouton **Lancer un backup** de cet onglet dispatche une commande agent (`module=restic action=run_backup`) qui exécute directement `run_backup.sh`, avec suivi de progression en direct (pourcentage, fichiers/octets traités, ETA) tant que le navigateur reste sur la page.
- **Planifié** : un backup récurrent se programme comme n'importe quelle autre tâche planifiée — page **Tâches planifiées**, module `restic`, action `run_backup`, cible = nom du profil resticprofile (`files`, `db`, …, laisser vide pour le profil par défaut du script). Il n'y a pas de webhook ni de cron externe à configurer côté ServerSupervisor.

#### Limites du suivi en direct

- La progression en direct dépend de `RESTIC_PROGRESS_FPS` et de la sortie `--json` de restic (forcés automatiquement par l'agent) — un backup lancé en dehors de ServerSupervisor (cron système, ligne de commande) n'est jamais suivi en direct, seul son résultat final apparaît via le monitoring passif au prochain rapport.
- Un backup manuel n'a pas de limite de durée fixe : il est coupé uniquement s'il reste silencieux plus de `restic_backup_idle_timeout_minutes` (pas de plafond absolu, contrairement aux autres commandes agent).
- Fermer l'onglet interrompt seulement l'affichage, pas le backup lui-même — revenir sur la page plus tard affiche le résultat final une fois le rapport de statut à jour.

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
| `GET/POST` | `/api/v1/auth/mfa/*` | Gestion MFA/2FA TOTP (setup/verify/disable) | Authentifié |
| `GET` | `/api/v1/auth/webauthn/credentials` | Liste des clés de sécurité/passkeys | Authentifié |
| `POST` | `/api/v1/auth/webauthn/register/begin\|finish` | Enregistrer une clé de sécurité/passkey | Authentifié |
| `DELETE` | `/api/v1/auth/webauthn/credentials/:id` | Supprimer une clé de sécurité/passkey | Authentifié |
| `POST` | `/api/auth/webauthn/login/begin\|finish` | Connexion via clé de sécurité/passkey (étape MFA) | Public |

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
| `POST` | `/api/v1/docker/command` | Envoyer une commande Docker/Compose | Operator+ |
| `GET` | `/api/v1/network` | Snapshot réseau | Authentifié |
| `GET` | `/api/v1/network/topology` | Topologie réseau | Authentifié |
| `GET/PUT` | `/api/v1/network/config` | Config topologie (overrides) | Authentifié |

#### APT
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/apt` | Statut APT d'un hôte | Authentifié |
| `POST` | `/api/v1/apt/command` | Envoyer une commande APT | Operator+ |

#### Sauvegardes Restic
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/hosts/:id/backup` | Statut agrégé (dernier run + état passif) | Authentifié |
| `GET` | `/api/v1/hosts/:id/backup/runs` | Historique des backups d'un hôte | Authentifié |
| `GET` | `/api/v1/backup/runs/:runId` | Détail d'un run | Authentifié |
| `POST` | `/api/v1/hosts/:id/backup/run` | Déclencher un backup manuel | Operator+ |

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

#### Notifications & Push
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/notifications` | Centre de notifications in-app | Authentifié |
| `POST` | `/api/v1/notifications/mark-read` | Marquer comme lues | Authentifié |
| `GET` | `/api/v1/push/vapid-public-key` | Clé publique VAPID | Authentifié |
| `POST` | `/api/v1/push/subscribe` | Enregistrer un abonnement Web Push | Authentifié |
| `DELETE` | `/api/v1/push/subscribe` | Supprimer l'abonnement | Authentifié |

#### Monitoring (sondes uptime & certificats SSL)
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/uptime/probes` | Liste des sondes uptime | Authentifié |
| `GET` | `/api/v1/uptime/probes/:id` | Détail d'une sonde | Authentifié |
| `GET` | `/api/v1/uptime/probes/:id/history` | Historique des checks | Authentifié |
| `GET` | `/api/v1/uptime/probes/:id/stats` | Statistiques agrégées | Authentifié |
| `POST` | `/api/v1/uptime/probes` | Créer une sonde | Admin |
| `PUT` | `/api/v1/uptime/probes/:id` | Modifier une sonde | Admin |
| `DELETE` | `/api/v1/uptime/probes/:id` | Supprimer une sonde | Admin |
| `POST` | `/api/v1/uptime/probes/:id/check-now` | Vérification immédiate | Admin |
| `GET` | `/api/v1/ssl/certificates` | Liste des certificats suivis | Authentifié |
| `GET` | `/api/v1/ssl/certificates/:id` | Détail d'un certificat | Authentifié |
| `GET` | `/api/v1/ssl/certificates/:id/history` | Historique des checks | Authentifié |
| `POST` | `/api/v1/ssl/certificates` | Ajouter un certificat à suivre | Admin |
| `PUT` | `/api/v1/ssl/certificates/:id` | Modifier | Admin |
| `DELETE` | `/api/v1/ssl/certificates/:id` | Supprimer | Admin |
| `POST` | `/api/v1/ssl/certificates/:id/check-now` | Vérification immédiate | Admin |

#### NPM (Nginx Proxy Manager)
| Méthode | Endpoint | Description | Rôle |
|---|---|---|---|
| `GET` | `/api/v1/npm/connections` | Liste des connexions NPM (sans secrets) | Authentifié |
| `GET` | `/api/v1/npm/connections/:id/proxy-hosts` | Proxy hosts importés d'une connexion | Authentifié |
| `GET` | `/api/v1/npm/proxy-hosts` | Tous les proxy hosts importés | Authentifié |
| `POST` | `/api/v1/npm/connections` | Créer une connexion | Admin |
| `PUT` | `/api/v1/npm/connections/:id` | Modifier une connexion | Admin |
| `DELETE` | `/api/v1/npm/connections/:id` | Supprimer une connexion | Admin |
| `POST` | `/api/v1/npm/connections/test` | Tester sans sauvegarder | Admin |
| `POST` | `/api/v1/npm/connections/:id/refresh-now` | Rafraîchir immédiatement | Admin |
| `PATCH` | `/api/v1/npm/proxy-hosts/:id` | Modifier le monitoring d'un proxy host | Admin |
| `PATCH` | `/api/v1/npm/proxy-hosts/:id/npm-enabled` | Activer/désactiver le suivi | Admin |

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
| `POST` | `/api/v1/proxmox/guests/:id/action` | Démarrer / arrêter / redémarrer une VM ou CT (`{"action":"start\|shutdown\|reboot"}`) | Admin |
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
| `/api/v1/ws/notifications` | Flux notifications (in-app + déclenche le push) |

> Authentification WebSocket : cookie de session envoyé automatiquement à la connexion, avec repli sur l'envoi de `{"type":"auth","token":"<jwt>"}` en message une fois la connexion établie (pour les clients qui ne peuvent pas compter sur le cookie). Il n'y a **pas** de fallback `?token=` en query string — retiré volontairement (fuite potentielle dans les logs de proxy/l'historique navigateur).

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
- Go 1.25+ (version exacte dans `server/go.mod` / `agent/go.mod`)
- Node.js 22+ (utilisé en CI ; le Dockerfile build sur Node 26)
- TimescaleDB 2.27.2 (PostgreSQL 16) — prérequis obligatoire (hypertables, time_bucket, retention policies)

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
├── server/                          # API Go (Gin) + WebSocket + scheduler + pollers
│   ├── cmd/server/main.go           # Bootstrap : config, migrations, background jobs, HTTP server
│   └── internal/
│       ├── api/                     # router.go (routes/middleware wiring) + middleware.go (JWT/CSRF/rate limit)
│       ├── handlers/                # Traduction HTTP : bind → service → respondError (fichiers par domaine)
│       ├── services/<domaine>/      # Logique métier + port Repository, un package par domaine :
│       │                            #   agent, alertrule, apt, audit, authn, docker, gitwebhook, host, hostperm,
│       │                            #   network, notifications, npm, proxmox, push, releasetracker,
│       │                            #   scheduledtask, settings, ssl, uptime, user, weblogs
│       ├── database/                # Implémentation des ports Repository (db_*.go) + migrations/*.sql
│       ├── models/                  # Structs partagés, un fichier par domaine (pas de models.go unique)
│       ├── apperr/                  # Erreurs typées → enveloppe HTTP uniforme {"error","code"}
│       ├── events/                  # Bus pub/sub in-process (déclenche les push WebSocket sur écriture)
│       ├── ws/                      # WSHandler, CommandStreamHub, NotificationHub (snapshots event-driven)
│       ├── alerts/                  # Moteur d'évaluation des règles (engine/metrics/authfailures/severity/notify)
│       ├── background/              # Jobs supervisés : audit cleanup, host status, alert eval, rétentions, uptime, SSL
│       ├── safego/                  # Helper recover()+log partagé, utilisé par toute goroutine détachée
│       ├── poller/                  # Boucle générique Every(ctx, interval, ...)
│       ├── scheduler/               # Scheduler cron (tâches planifiées)
│       ├── dispatch/                # Persistance des remote_commands (file de commandes agent)
│       ├── proxmoxclient/           # Client HTTP Proxmox VE
│       ├── npmclient/               # Client HTTP Nginx Proxy Manager
│       ├── gitprovider/             # Client releases GitHub/GitLab/Gitea
│       ├── releasetracker/          # Helpers purs de comparaison de version (pas le tracker lui-même)
│       ├── synthetic/               # Sondes uptime HTTP/TCP + vérification de certificats SSL
│       ├── config/                  # Config env vars + override runtime depuis la table settings
│       └── notify/                  # Envoi SMTP + ntfy + template HTML d'alerte
├── agent/                           # Collecteur Go déployé sur chaque VM/hôte supervisé (pas sur Proxmox)
│   ├── cmd/agent/main.go            # Flags, --init, --internal-update, --internal-healthcheck
│   └── internal/
│       ├── reporter/                # Collecte parallèle → POST /api/agent/report
│       ├── dispatcher/              # Exécution des commandes (mutex apt + sémaphore + registry par module)
│       ├── collector/               # Un fichier par domaine : system, docker, apt, disk, web_logs, systemd,
│       │                            #   journal, processes, crowdsec
│       ├── sender/                  # Structs Report/PendingCommand/CommandResult + client HTTP
│       └── config/                  # Config YAML + env vars ; tasks.go charge tasks.yaml
├── frontend/                        # SPA Vue 3 + TypeScript (Tabler CSS)
│   └── src/
│       ├── api/                     # client.ts (axios + intercepteurs CSRF/401) + modules par domaine
│       ├── router/                  # Routes lazy-loaded + retry sur ChunkLoadError
│       ├── stores/                  # Pinia : auth, hosts, dashboard, alertRules
│       ├── composables/             # useWebSocket, useDashboard, useHostDetail, use<Domaine> par vue
│       ├── components/              # Organisés par domaine (proxmox/, npm/, security/, settings/, ...)
│       ├── views/                   # Une vue par route
│       └── types/                   # generated.ts (généré par tygo depuis les modèles Go) + types par domaine
├── protocol/                        # Fixture golden du contrat agent↔serveur + README
├── .github/workflows/               # ci-{server,agent,frontend}.yml, release.yml, security.yml, pr-checks.yml, stale.yml
├── docker-compose.yml                # postgres (TimescaleDB) + server + postgres-backup
├── .env.example
└── README.md
```

---

## Licence

MIT
