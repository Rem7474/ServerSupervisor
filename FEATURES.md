# ServerSupervisor - Nouvelles fonctionnalités

## 1. Tooltips interactifs sur les graphiques

### Fonctionnalité
Les graphiques de métriques (CPU et RAM) affichent maintenant des tooltips au survol de la souris avec :
- Le timestamp précis du point de données
- La valeur exacte (CPU % ou RAM %)
- Un point de survol agrandi pour faciliter l'interaction

### Implémentation
- **Frontend** : Configuration Chart.js enrichie dans `DashboardView.vue` et `HostDetailView.vue`
- **Configuration** : 
  - `hitRadius: 10` pour faciliter le survol
  - `hoverRadius: 5` pour afficher un point au survol
  - Callbacks personnalisés pour formater les tooltips

### Fichiers modifiés
- `frontend/src/views/DashboardView.vue` : Graphique global des métriques
- `frontend/src/views/HostDetailView.vue` : Graphiques par hôte

---

## 2. Suivi de version des agents

### Fonctionnalité
Affichage de la version de l'agent sur chaque hôte avec indication visuelle si l'agent est à jour ou obsolète.

### Fonctionnement
- **Version actuelle** : Hardcodée dans l'agent (`AgentVersion = "1.2.0"`)
- **Envoi au serveur** : Incluse dans chaque rapport d'agent
- **Stockage** : Colonne `agent_version` dans la table `hosts`
- **Affichage** : Badge vert (à jour) ou jaune (obsolète) dans le dashboard et la page hôte

### Architecture

#### Agent (Go)
1. **Constante de version** (`agent/cmd/agent/main.go`)
   - `const AgentVersion = "1.2.0"`
   - Facile à maintenir (un seul endroit)

2. **Inclusion dans les rapports** (`agent/internal/sender/sender.go`)
   - Champ `AgentVersion` dans le struct `Report`
   - Envoyé avec chaque rapport au serveur

#### Backend (Go)
1. **Modèle de données** (`server/internal/models/models.go`)
   - `AgentVersion` ajouté au struct `Host`
   - `AgentVersion` ajouté au struct `AgentReport`
   - `AgentVersion` ajouté au struct `HostUpdate`

2. **Stockage** (`server/internal/database/db.go`)
   - Colonne `agent_version VARCHAR(20)` dans la table `hosts`
   - Mise à jour automatique lors de chaque rapport agent

3. **Migration SQL** (`server/migrations/001_add_agent_version.sql`)
   ```sql
   ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;
   CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);
   ```

#### Frontend (Vue.js)
1. **Dashboard** (`frontend/src/views/DashboardView.vue`)
   - Nouvelle colonne "Agent" dans le tableau des hôtes
   - Badge vert si version = 1.2.0, jaune sinon
   - Affichage "v1.2.0" ou "-" si version inconnue

2. **Host Detail** (`frontend/src/views/HostDetailView.vue`)
   - Badge "Agent v1.2.0" dans le header
   - Affichage dans la description textuelle
   - Couleur verte (à jour) ou jaune (obsolète)

3. **Logique de comparaison**
   ```javascript
   const LATEST_AGENT_VERSION = '1.2.0'
   function isAgentUpToDate(version) {
     return version === LATEST_AGENT_VERSION
   }
   ```

### Bénéfices

- **Visibilité** : Savoir instantanément quels agents sont obsolètes
- **Maintenance** : Prioriser les mises à jour d'agents
- **Audit** : Historique des versions déployées
- **Sécurité** : Identifier rapidement les agents non patchés

### Utilisation

1. **Voir les versions** : Colonne "Agent" dans le dashboard principal
2. **Identifier les obsolètes** : Badge jaune = besoin de mise à jour
3. **Détail par hôte** : Info complète dans la page de détail de l'hôte

### Mise à jour de version

Pour mettre à jour la version de référence :

1. **Agent** : Modifier `AgentVersion` dans `agent/cmd/agent/main.go`
2. **Frontend** : Modifier `LATEST_AGENT_VERSION` dans `DashboardView.vue` et `HostDetailView.vue`
3. **Recompiler** : `go build` pour l'agent, `npm run build` pour le frontend
4. **Déployer** : Les nouveaux agents se signaleront automatiquement

### Migration SQL requise

```sql
-- Exécuter manuellement sur la base de données
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);
```

---

## 3. Console de logs en temps réel pour les commandes APT

### Fonctionnalité
Affichage en temps réel de la sortie des commandes APT (update, upgrade, dist-upgrade) directement dans l'interface web via une console live.

### Architecture

#### Backend
1. **Hub de streaming** (`server/internal/api/apt_stream.go`)
   - Gestion des connexions WebSocket par commande
   - Broadcasting des chunks de logs à tous les clients connectés
   - Nettoyage automatique des connexions fermées

2. **Endpoint WebSocket** (`/api/v1/ws/apt/stream/:command_id`)
   - Authentification JWT via query string
   - Envoi de l'état initial (sortie existante)
   - Streaming en temps réel des nouveaux chunks

3. **Endpoint agent** (`/api/agent/command/stream`)
   - Réception des chunks de sortie depuis l'agent
   - Broadcast immédiat aux clients WebSocket

#### Agent
1. **Streaming d'exécution** (`agent/internal/collector/apt.go`)
   - `ExecuteAptCommandWithStreaming()` : Nouvelle fonction
   - Capture stdout et stderr en temps réel
   - Callback pour envoyer chaque chunk au serveur

2. **Envoi des chunks** (`agent/internal/sender/sender.go`)
   - `StreamCommandChunk()` : POST vers `/api/agent/command/stream`
   - Envoi asynchrone pour ne pas bloquer l'exécution
   - Tolérance aux erreurs de streaming

3. **Intégration** (`agent/cmd/agent/main.go`)
   - `processCommands()` utilise la nouvelle fonction de streaming
   - Callback inline pour envoyer les chunks
   - Sortie complète toujours envoyée à la fin

#### Frontend
1. **Interface console** (`frontend/src/views/AptView.vue`)
   - Carte dépliante avec console noire style terminal
   - Bouton "Voir les logs" sur les commandes en cours (`running`, `pending`)
   - Auto-scroll vers le bas à chaque nouveau chunk
   - Fermeture manuelle de la console

2. **Connexion WebSocket**
   - `connectStreamWebSocket(commandId)` : Établit la connexion
   - Gestion des messages `apt_stream_init` et `apt_stream`
   - Concaténation des chunks entrants
   - Nettoyage à la fermeture

### Flux de données

```
Agent (laptop)          →  Server                    →  Browser
──────────────             ────────                     ─────────
1. Execute apt upgrade
2. Stdout chunk         →  POST /api/agent/command/stream
                           3. streamHub.Broadcast()
                                                       →  WS message
                                                          4. Display chunk
5. Stderr chunk         →  POST /api/agent/command/stream
                           6. streamHub.Broadcast()
                                                       →  WS message
                                                          7. Display chunk
8. Exit                 →  POST /api/agent/command/result
                           9. Update DB (status: completed)
```

### Fichiers créés
- `server/internal/api/apt_stream.go` : Hub de streaming WebSocket

### Fichiers modifiés

**Backend**
- `server/internal/api/ws.go` : Nouvelle méthode `AptStream()`, ajout de `streamHub`
- `server/internal/api/agent.go` : Nouvelle méthode `StreamCommandOutput()`
- `server/internal/api/router.go` : Route `/api/v1/ws/apt/stream/:command_id` et `/api/agent/command/stream`

**Agent**
- `agent/internal/collector/apt.go` : 
  - `ExecuteAptCommandWithStreaming()` : Nouvelle fonction
  - `runCommandWithStreaming()` : Capture et streaming de stdout/stderr
- `agent/internal/sender/sender.go` : 
  - `StreamCommandChunk()` : Envoi des chunks au serveur
- `agent/cmd/agent/main.go` : 
  - `processCommands()` : Utilisation du streaming avec callback

**Frontend**
- `frontend/src/views/AptView.vue` : 
  - Console live avec carte dépliante
  - Connexion WebSocket pour le streaming
  - Boutons "Voir les logs" sur les commandes actives
  - Auto-scroll et fermeture manuelle

### Utilisation

1. **Depuis le dashboard ou APT view** : Lancer une commande APT (update, upgrade, dist-upgrade)
2. **Dans APT view** : Cliquer sur "Voir les logs" pour une commande `running` ou `pending`
3. **Console live** : Les logs s'affichent en temps réel dans une fenêtre type terminal
4. **Fermeture** : Bouton "Fermer" en haut à droite de la console

### Bénéfices

- **Visibilité** : Voir la progression des commandes APT en temps réel
- **Debugging** : Identifier rapidement les erreurs ou blocages
- **UX** : Ne plus attendre "dans le vide" pendant les upgrades longs
- **Audit** : Logs complets même si la connexion WebSocket échoue (fallback sur sortie finale)

---

## Résumé des changements

### Backend
- **24 fichiers modifiés** : API streaming, WebSocket, agent handlers, version tracking
- **2 fichiers créés** : `apt_stream.go` (hub de streaming), `001_add_agent_version.sql` (migration)
- **Dépendances** : Aucune nouvelle dépendance (gorilla/websocket déjà présent)

### Frontend
- **3 fichiers modifiés** : `DashboardView.vue`, `HostDetailView.vue`, `AptView.vue`
- **Dépendances** : Aucune nouvelle dépendance

### Agent
- **4 fichiers modifiés** : `apt.go`, `sender.go`, `main.go`, version constante
- **Dépendances** : Aucune nouvelle dépendance

### Base de données
- **Migration SQL requise** : Ajouter colonne `agent_version` à la table `hosts`

### Compilation
✅ **Server** : Compilation réussie (`go build`)
✅ **Agent** : Compilation réussie (`go build`)
⚠️ **Frontend** : Non testé (npm non disponible sur ce système)

---

## Tests recommandés

1. **Tooltips** :
   - Survoler les graphiques CPU/RAM sur le dashboard
   - Survoler les graphiques d'un hôte spécifique
   - Vérifier que les valeurs et timestamps s'affichent correctement

2. **Version agent** :
   - Vérifier l'affichage de la version dans la colonne "Agent" du dashboard
   - Badge vert pour version 1.2.0, jaune pour autres versions
   - Affichage dans la page de détail d'un hôte
   - Exécuter la migration SQL avant le test

3. **Console APT** :
   - Lancer `apt update` sur un hôte
   - Cliquer sur "Voir les logs" dans APT view
   - Vérifier que les logs s'affichent progressivement
   - Tester `apt upgrade` (plus long, meilleur test de streaming)
   - Fermer la console et la rouvrir pendant l'exécution
   - Vérifier le comportement si le WebSocket se déconnecte

3. **Compatibilité** :
   - Tester avec plusieurs clients simultanés sur la même commande
   - Vérifier que les anciens agents (sans streaming) fonctionnent toujours
   - Tester avec un agent en version récente (avec streaming)

---

## Notes de sécurité

- **WebSocket auth** : JWT via query string (déjà sécurisé)
- **Agent auth** : API Key (inchangé)
- **RBAC** : Console accessible uniquement aux admin/operator (lecture seule pour viewer)
- **Injection** : Aucun input utilisateur dans les commandes shell (paramètres hardcodés)

---

## Prochaines étapes suggérées

1. **Amélioration console** :
   - Coloration syntaxique (ANSI escape codes)
   - Bouton "Télécharger les logs"
   - Historique des 10 dernières commandes avec accès rapide

2. **Monitoring avancé** :
   - Alertes en temps réel (CPU > 95%, Disk > 90%)
   - Graphiques interactifs avec zoom/pan
   - Export CSV des métriques

3. **APT enhancements** :
   - Dry-run mode (apt upgrade --dry-run)
   - Approbation manuelle des upgrades package par package
   - Rollback automatique en cas d'échec

4. **Version management** :
   - Comparaison sémantique avancée (1.2.3 vs 1.2.10)
   - Notifications automatiques de nouvelle version
   - Update agent à distance depuis le dashboard

---

**Date de mise à jour** : 2026-02-21
**Version** : ServerSupervisor v1.2.0
**Agent version** : v1.2.0
