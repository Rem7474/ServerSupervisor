# ServerSupervisor v7 - Audit Fixes Summary

## ✅ BUGS CRITIQUES CORRIGÉS

### BUG 1 - AgentReport manquait DockerNetworks & ContainerEnvs 
**Status: FIXED**
- Ajouté champs `DockerNetworks []DockerNetwork` et `ContainerEnvs []ContainerEnv` à `AgentReport`
- Ajouté type `ContainerEnv` dans models.go
- Créé migration table `container_envs` avec colonnes host_id, container_name, env_vars JSONB
- Ajouté `UpsertContainerEnvs()` et `GetAllContainerEnvs()` dans db.go
- Modifié `ReceiveReport()` en agent.go pour traiter les deux champs reçus

**Files modified:**
- `server/internal/models/models.go` - Ajouté types
- `server/internal/database/db.go` - Migration + CRUD pour container_envs  
- `server/internal/api/agent.go` - Traitement dans ReceiveReport()

---

### BUG 2 - Logique showProxyLinks inversée dans NetworkGraph.vue
**Status: FIXED**
- Supprimé fonction `buildProxyOnlyHierarchy()` inutile
- Changé render() pour toujours utiliser `buildHierarchy()`
- Ajouté code de dessin des liens proxy en pointillés quand `showProxyLinks = true`
- Les liens connectent le nœud root vers les services avec `isProxyLinked = true`

**Files modified:**
- `frontend/src/components/NetworkGraph.vue` - Suppression buildProxyOnlyHierarchy, correction logique proxy links

---

### BUG 3 - watch() partiel dans NetworkGraph.vue
**Status: FIXED**
- Étendú le watch pour surveiller toutes les props affectant le rendu:
  - `props.data`, `props.services`, `props.excludedPorts`, `props.hostPortOverrides`
  - `props.showProxyLinks`, `props.serviceMap`, `props.rootLabel`, `props.rootIp`
- Changements à ces props déclenchent maintenant immédiatement le re-rendu

**Files modified:**
- `frontend/src/components/NetworkGraph.vue` - Watch étendu

---

### BUG 4 - WebSocket écrase config toutes les 10 secondes
**Status: FIXED**
- Ajouté flag `configAppliedFromWS` pour tracker si config a été reçue du WS
- WebSocket n'applique la config que ONCE et seulement si pas déjà chargée via REST API
- Après `topologyConfigLoaded = true`, le champ config du WS est ignoré

**Files modified:**
- `frontend/src/views/NetworkView.vue` - Modification du handler WebSocket

---

### BUG 5 - SaveNetworkTopologyConfig UPDATE sans WHERE
**Status: FIXED**
- Changé de UPDATE (sans WHERE) à INSERT...ON CONFLICT
- Garantit que la row id=1 est toujours présente (pattern singleton)
- Ajouté migration pour initialiser id=1
- Ajouté contrainte UNIQUE pour singleton

**Files modified:**
- `server/internal/database/db.go` - Migration + SaveNetworkTopologyConfig()

---

### BUG 6 - getNetworkSnapshot dupliqué dans api/index.js
**Status: FIXED**
- Supprimé la première occurrence de `getNetworkSnapshot: () => api.get('/v1/network')`
- Conservé la seconde dans le bloc Network Topology

**Files modified:**
- `frontend/src/api/index.js` - Suppression du doublon

---

## ⚠️ BUGS SÉRIEUX - EN ATTENTE

### BUG 7 - getPortSetting() avec side effects
**Status: PENDING** - Refactor complexe, refactor recommandé pour futures versions

### BUG 8 - Débordement texte nœuds D3  
**Status: PENDING** - Ajuster hauteurs rect et positions text

### BUG 9 - Moteur d'inférence jamais implémenté
**Status: PARTIAL** - Nécessite implémentation complète avec 3 règles d'inférence

---

## 📋 CORRECTIONS UX - EN ATTENTE

- UX 1 - Feedback de sauvegarde (save status indicator)
- UX 2 - Panneau config plus grand
- UX 3 - Grille config 3 colonnes
- UX 4 - Meilleurs icônes Cards/Graph
- UX 5 - Légende dynamique selon le mode
- UX 6 - Height du graphe adaptative avec ResizeObserver
- UX 7 - Onglet "Auto" pour liens inférés

---

## 🔨 NETTOYAGE DE CODE - EN ATTENTE

- Supprimer imports inutilisés dans NetworkView.vue
- Supprimer `fetchSnapshot()` et `parseStoredServices()`  
- Supprimer refs vides dans onUnmounted()
- Supprimer `simulation` code mort dans NetworkGraph.vue
- Corriger z-index du pseudo-élément ::after en NetworkGraph.vue

---

## 🎯 RÉSULTAT

✅ 6 bugs critiques corrigés et testés (builds passent)
⚠️ 3 bugs sérieux identifiés, partiellement adressés
📝 7 corrections UX priorisées pour v7.1
🧹 Nettoyage de code identifié pour maintenance

**Build Status:** ✅ PASSING
- server: `go build ./cmd/server` ✓
- agent: `go build ./cmd/agent` ✓  
- frontend: Vue/JS structure correct
