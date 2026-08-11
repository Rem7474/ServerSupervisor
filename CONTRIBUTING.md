# Contribuer à ServerSupervisor

## Prérequis

- Go 1.25+ (version exacte dans `server/go.mod` / `agent/go.mod`)
- Node.js 22+
- TimescaleDB 2.27.2 (PostgreSQL 16) — prérequis obligatoire (hypertables, `time_bucket`, retention policies)

## Build & tests

```bash
cd server   && go build ./... && go test -v -race ./... && golangci-lint run
cd agent    && go build ./... && go test -v -race -coverprofile=coverage.out ./... && golangci-lint run
cd frontend && npm run typecheck && npm run lint && npm run test && npm run build
```

CI bloque aussi sur `go mod tidy` (les deux modules Go) et sur la synchronisation
de `frontend/src/types/generated.ts` avec les modèles Go (`ci-server.yml`
régénère et diff ce fichier) — voir la note sur les types générés dans
[CLAUDE.md](CLAUDE.md) avant de modifier un modèle.

## Mode démo

Un jeu de données de démonstration fixe et réaliste (hôtes/VMs, conteneurs
Docker dans des états variés, paquets APT en attente, alertes actives et
résolues, tâches planifiées, historique d'audit, sondes uptime/certificats
SSL — dont un certificat bientôt expiré) pour :

- la vérification visuelle manuelle avant une release ;
- le développement quotidien contre des données réalistes, sans dépendre
  d'une vraie infra Proxmox/Docker/APT ;
- tester des cas limites (hôte hors ligne, conteneur `unhealthy`, disque
  presque plein, certificat SSL bientôt expiré, alerte non résolue) sans
  avoir à les reproduire sur une vraie machine.

`DEMO_MODE=true` désactive tous les pollers qui font un appel réseau réel
(Proxmox VE, Nginx Proxy Manager, sondes uptime/SSL, suivi de releases Git)
ainsi que le job qui repasserait les hôtes "hors ligne" faute de heartbeat
réel — aucune dépendance réseau externe en mode démo. Le seed
(`server/cmd/seed-demo`) est idempotent : le relancer ne duplique rien, il
remet les données à leur état canonique.

### Option A — boucle rapide (développement backend/frontend)

La même boucle que le développement normal (voir [README.md § Développement
local](README.md#développement-local)), avec `DEMO_MODE=true` en plus :

```bash
# Terminal 1 : PostgreSQL (mode démo, port 55432 pour ne pas entrer en
# conflit avec un Postgres local existant)
docker compose -f docker-compose.demo.yml up -d postgres-demo

# Terminal 2 : serveur Go en mode démo
cd server && DEMO_MODE=true APP_ENV=dev DB_PORT=55432 go run ./cmd/server

# Terminal 3 : seed (à relancer à volonté, idempotent)
cd server && DEMO_MODE=true DB_PORT=55432 go run ./cmd/seed-demo

# Terminal 4 : frontend avec hot-reload (proxy → serveur Go, inchangé)
cd frontend && npm run dev
```

Rebuild du backend quasi instantané (`go run`), hot-reload Vite intact —
c'est le mode à privilégier pour développer une fonctionnalité en la testant
contre des données réalistes.

### Option B — stack complète (vérification pré-release)

Reconstruit l'image Docker réelle (celle publiée en release) et démarre tout
en une commande — plus lent à chaque changement de code, mais c'est
exactement la stack que `.github/workflows/screenshots.yml` utilise pour
régénérer les captures du README à chaque tag :

```bash
docker compose -f docker-compose.demo.yml up -d --build
docker compose -f docker-compose.demo.yml exec server-demo /app/seed-demo
```

Puis ouvrir http://localhost:8080 — identifiants `demo` / `DemoPass123!`
(fixes, locaux, jamais à utiliser en production).

### Réinitialiser

Le plus simple : relancer le seed (idempotent, remet chaque table à son état
canonique sans toucher au volume) :

```bash
docker compose -f docker-compose.demo.yml exec server-demo /app/seed-demo
# ou en option A : cd server && DEMO_MODE=true DB_PORT=55432 go run ./cmd/seed-demo
```

Pour repartir d'un état complètement vierge (volume compris) :

```bash
docker compose -f docker-compose.demo.yml down -v
docker compose -f docker-compose.demo.yml up -d --build
docker compose -f docker-compose.demo.yml exec server-demo /app/seed-demo
```

Si les hôtes seedés finissent par apparaître "hors ligne" après une longue
session (ils ne reçoivent aucun vrai heartbeat), relancer simplement le seed
pour rafraîchir leur `last_seen`.

### À savoir

- Les boutons "Tester la connexion" / "Poll now" (Proxmox, NPM) restent
  cliquables en mode démo et échoueront visiblement — la connexion Proxmox
  seedée est volontairement désactivée (`enabled=false`) et pointe vers une
  URL fictive. C'est attendu : seuls les pollers automatiques sont
  désactivés, pas les actions manuelles.
- N'active jamais `DEMO_MODE=true` sur un déploiement réel : `seed-demo`
  refuse de démarrer si `DEMO_MODE` n'est pas positionné, mais rien
  n'empêche techniquement de pointer la variable d'environnement vers une
  vraie base — cette variable n'est lue que depuis l'environnement, jamais
  depuis les réglages en base, précisément pour qu'elle ne puisse pas être
  activée par erreur depuis l'UI.
