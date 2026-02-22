# Audit v6 — Changements appliqués

Date: 2026-02-22

## 🔴 CORRECTIONS CRITIQUES

### 1. RBAC sur endpoints Settings destructifs
**Fichier:** `server/internal/api/settings.go`  
**Implémentation:** Validation rôle `admin` sur :
- `CleanupMetrics()` — requiert rôle admin
- `CleanupAuditLogs()` — requiert rôle admin

Avant: N'importe quel utilisateur `viewer` pouvait déclencher suppression massive  
Après: Seuls les administrateurs peuvent exécuter ces opérations

---

### 2. Suppression code mort: `HasOpenIncident()`
**Fichier:** `server/internal/database/db.go` (ligne ~1175)  
**Raison:** Fonction déclarée mais jamais appelée → confusion potentielle  
**Action:** Complètement supprimée

---

### 3. Configuration `.env.example` complétée
**Fichier:** `.env.example`  
**Ajouts:**
```env
TLS_ENABLED=false
# TLS_CERT_FILE=/etc/ssl/certs/server.pem
# TLS_KEY_FILE=/etc/ssl/private/server.key

ALLOWED_ORIGINS=
```
Permet aux utilisateurs de configurer TLS et CORS correctement

---

## 🟠 PROBLÈMES SÉRIEUX — PERSISTANCE RÉSEAU

### 3. Configuration réseau : localStorage → Base de données

Avant toutes les modifications d'architecture réseau (nom proxy, IP, ports exclus, services, overrides) était en localStorage client seulement.

**Problèmes:**
- Config se perd en changeant de navigateur/appareil
- Non partageable entre administrateurs
- Disparaît si navigateur vide le storage

**Solution implémentée:** Persistance en DB + WebSocket async

---

## 🆕 NOUVELLE ARCHITECTURE — TOPOLOGIE RÉSEAU AUTOMATIQUE

### Phase 1 : Modèles et Migrations DB

**Fichier:** `server/internal/models/models.go`  
**Structs ajoutées:**
```go
type DockerNetwork {
    ID, HostID, NetworkID, Name, Driver, Scope
    ContainerIDs []string  // JSONB dans DB
}

type TopologyLink {
    SourceContainerName, TargetContainerName
    LinkType: "network" | "env_ref" | "proxy"
    Confidence: 0-100
}

type NetworkTopologyConfig {
    RootLabel, RootIP, ExcludedPorts, ServiceMap
    ShowProxyLinks, HostOverrides, ManualServices
    (tous persistés en DB)
}

type TopologySnapshot {
    Hosts, Containers, Networks, Links, Config
}
```

**Fichier:** `server/internal/database/db.go`  
**Migrations:**
- Table `docker_networks` — stocke réseaux Docker par hôte
- Table `network_topology_config` — config persistée (une seule ligne)
- Index sur `(host_id)` pour requêtes rapides

**Fonctions CRUD:**
- `UpsertDockerNetworks(hostID, networks)` — mise à jour des réseaux découverts
- `GetDockerNetworks*()` — lecture par hôte ou globalet
- `Get/SaveNetworkTopologyConfig()` — persistent config

---

### Phase 2 : Collecteur Agent — Docker Networks + Env Vars

**Fichier:** `agent/internal/collector/docker.go`

**Fonction `CollectDockerNetworks()`:**
```go
// Découvre réseaux Docker et conteneurs connectés
// Exclut les réseaux système (bridge, host, none)
// Utilise: docker network ls + docker network inspect
```

**Fonction `CollectContainerEnvVars()`:**
```go
// Récupère variables d'environnement des conteneurs
// Filtre sensibles: password, secret, token, key, auth, salt, etc.
// Important: évite fuites de données
```

**Types:**
```go
type DockerNetwork struct {
    NetworkID    string   // SHA256 truncated to 12 chars
    Name         string
    Driver       string   // bridge, overlay, etc.
    Scope        string   // local, swarm
    ContainerIDs []string // Membres du réseau
}

type ContainerEnv struct {
    ContainerName string
    EnvVars       map[string]string // Sans secrets
}
```

**Fichier:** `agent/internal/sender/sender.go`  
**Changement:** Rapport agent inclut maintenant:
```go
type Report struct {
    // ... existing fields ...
    DockerNetworks interface{} // []DockerNetwork
    ContainerEnvs  interface{} // []ContainerEnv
}
```

**Fichier:** `agent/cmd/agent/main.go`  
**Intégration:** Dans chaque rapport, collecte et envoie:
- Réseaux Docker détectés
- Env vars des conteneurs (filtrées pour sécurité)

---

### Phase 3 : API Réseau - Inference des Liens + Config

**Fichier:** `server/internal/api/network.go`

**Nouveaux endpoints:**
```
GET  /v1/network/topology          — Snapshot complet avec liens inférés
GET  /v1/network/config            — Configuration persistée actuelle
PUT  /v1/network/config            — Sauvegarder nouvelle configuration
```

**Logique d'inférence (3 règles):**

1. **Réseau Docker partagé** → Lien `network` (confiance 70%)
   - Si A et B sont sur le même réseau Docker (non-système)
   - Ils peuvent communiquer directement

2. **Référence variable d'environnement** → Lien `env_ref` (confiance 90%)
   - Si container A a `DATABASE_HOST=postgres`
   - Et existe container nommé `postgres`
   - A dépend de postgres

3. **Traefik/proxy** → Lien `proxy` (confiance 95%)
   - Si container a label `traefik.http.routers.X.rule=Host(immich.domain.com)`
   - Et existe container nginx/traefik/npm
   - Proxy → service (avec domaine stocké)

**Déduplication:** Gardé lien avec confiance la plus élevée

---

### Phase 4 : WebSocket enrichi

**Fichier:** `server/internal/api/ws.go`

**Avant:** WebSocket envoyait hosts + containers seulement  
**Après:** WebSocket envoie snapshot complet:
```json
{
  "type": "network",
  "hosts": [...],
  "containers": [...],
  "networks": [...],      // NOUVEAU: réseaux Docker
  "config": {...},        // NOUVEAU: config persistée
  "updated_at": "2026-02-22T..."
}
```

Permet sync automatique config à travers multiple clients

---

### Phase 5 : Frontend — Persistance DB

**Fichier:** `frontend/src/api/index.js`  
**Nouveaux clients:**
```javascript
getTopologyConfig()          // GET /v1/network/config
saveTopologyConfig(config)   // PUT /v1/network/config
getTopologySnapshot()        // GET /v1/network/topology
```

**Fichier:** `frontend/src/views/NetworkView.vue`

**Avant:**
- Tous les states chargés depuis `localStorage.getItem()`
- Chaque changement sauvegardé immédiatement en localStorage
- Config se perd entre navigateurs

**Après:**
- Au mount: `loadTopologyConfig()` depuis DB
- Watches déclenchent `debouncedSave()` (500ms debounce)
- Debounce évite rafales d'appels API pendant édition intensive
- WebSocket reçoit config + networks automatiquement

```javascript
// Nouveau lifecycle
onMounted(async () => {
  await loadTopologyConfig()      // Charge depuis DB
  await fetchSnapshot()            // Puis données temps réel
})

// Debounce 500ms sur changes
watch([rootNodeName, servicePortMapText, ...], () => {
  debouncedSave()
})

// WebSocket reçoit mises à jour auto
useWebSocket('/api/v1/ws/network', (payload) => {
  networks.value = payload.networks || []
  if (payload.config) {
    rootNodeName.value = payload.config.root_label
    // ... sync config depuis serveur
  }
})
```

**Résultat:**
- ✅ Config persistante entre onglets/appareils
- ✅ Partageable entre admin (via DB)
- ✅ Pas de perte si localStorage vide
- ✅ Sync temps réel via WebSocket

---

## 📦 FICHIERS MODIFIÉS

| Fichier | Type | Raison |
|---------|------|--------|
| `server/internal/api/settings.go` | Fix | RBAC sur cleanup |
| `server/internal/database/db.go` | Feature | Migrations + CRUD réseau |
| `server/internal/models/models.go` | Feature | Types topologie |
| `server/internal/api/network.go` | Feature | Endpoints + inférence |
| `server/internal/api/ws.go` | Enhancement | Enrichir payload WS |
| `server/internal/api/router.go` | Update | Ajouter routes réseau |
| `agent/internal/collector/docker.go` | Feature | Collector réseaux + env |
| `agent/internal/sender/sender.go` | Update | Inclure données réseau |
| `agent/cmd/agent/main.go` | Update | Appeler new collectors |
| `frontend/src/api/index.js` | Feature | Clients API réseau |
| `frontend/src/views/NetworkView.vue` | Refactor | localStorage → DB |
| `.env.example` | Docs | Ajouter configs manquantes |

---

## 🔒 Sécurité

1. **RBAC appliqué** — Cleanup ops réservées admin
2. **Secrets filtrés** — Variables d'env sensibles jamais envoyées
3. **Debounce** — Évite bombardement API
4. **DB persistance** — Config sécurisée vs client storage

---

## 🚀 Testing

```bash
# Vérifier compilation Go
cd server && go build ./cmd/server

# Vérifier endpoints
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/network/topology

curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/network/config

# Modifier config (test PUT)
curl -X PUT \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"root_label":"MyInfra","root_ip":"192.168.1.1"}' \
  http://localhost:8080/api/v1/network/config
```

---

## ✅ Résumé

**Avant:** Config réseau statique, client-side, localhost seulement  
**Après:** Topologie réseau **automatiquement découverte**, config **persistante**, **partageable**, avec **liens inférés intelligents**

L'infrastructure réseau n'est plus une configuration manuelle — elle se construit elle-même à partir des données réelles collectées par les agents!
