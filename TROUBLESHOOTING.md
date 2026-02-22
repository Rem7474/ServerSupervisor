# Guide de dépannage - Version agent & Logs APT

## Problèmes rencontrés

1. ❌ Version de l'agent affiche "-" dans le dashboard
2. ❌ Console de logs APT en direct ne s'affiche pas
3. ❌ Bouton "Voir les logs" ne fait rien

## Solutions pas à pas

### Étape 1 : Diagnostic

Exécutez le script de diagnostic pour identifier les problèmes :

```powershell
.\diagnostic.ps1
```

Ce script va vérifier :
- Docker est en cours d'exécution
- Les conteneurs sont démarrés
- La colonne `agent_version` existe dans la base de données
- Le frontend est correctement déployé

### Étape 2 : Exécuter la migration SQL

**Si le diagnostic indique que la colonne `agent_version` est manquante :**

```powershell
.\migrate.ps1
```

Ou manuellement :

```powershell
docker exec -it serversupervisor-postgres-1 psql -U supervisor -d serversupervisor

# Puis dans psql:
ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);
\q
```

### Étape 3 : Rebuild complet si nécessaire

**Si le diagnostic indique des problèmes avec le frontend :**

```powershell
# Arrêter les conteneurs
docker-compose down

# Rebuild sans cache
docker-compose build --no-cache

# Redémarrer
docker-compose up -d
```

### Étape 4 : Redémarrer les agents

Pour que les agents envoient leur version, ils doivent être redémarrés :

```bash
# Sur chaque machine avec un agent
sudo systemctl restart serversupervisor-agent

# Ou si lancé manuellement
pkill agent
./agent -config /path/to/agent.yaml
```

### Étape 5 : Vérifier dans le navigateur

1. Ouvrez le dashboard : http://localhost:8080
2. Connectez-vous
3. Vérifiez la colonne "Agent" dans le tableau des hôtes
4. Allez dans la page "APT" 
5. Cliquez sur "Voir l'historique" pour un hôte
6. Si une commande est en cours (running), le bouton "Voir les logs" devrait apparaître

### Étape 6 : Debug navigateur

Si la console APT ne s'affiche toujours pas :

1. Ouvrez les outils développeur (F12)
2. Onglet "Console" - vérifiez les erreurs JavaScript
3. Onglet "Network" - vérifiez que les WebSocket se connectent
   - Recherchez `/api/v1/ws/apt/stream/`
   - Le status devrait être "101 Switching Protocols"

## Vérification rapide

### Version agent fonctionne si :
- ✅ La colonne `agent_version` existe dans la table `hosts`
- ✅ Les agents ont été redémarrés après rebuild
- ✅ Au moins un rapport a été envoyé par l'agent

### Console APT fonctionne si :
- ✅ Le frontend a été rebuild et déployé
- ✅ Le serveur a les routes WebSocket `/api/v1/ws/apt/stream/:command_id`
- ✅ Une commande APT est en état "running" ou "pending"

## Commandes utiles

```powershell
# Voir les logs du serveur
docker logs -f serversupervisor-server-1

# Voir les logs de PostgreSQL
docker logs -f serversupervisor-postgres-1

# Redémarrer le serveur sans rebuild
docker-compose restart server

# Rebuild complet propre
docker-compose down -v  # ⚠️ ATTENTION: Supprime les données !
docker-compose build --no-cache
docker-compose up -d
```

## Si rien ne fonctionne

1. **Vérifier que le code source est à jour** :
   ```powershell
   git status
   git log --oneline -5
   ```

2. **Vérifier les fichiers modifiés** :
   - `agent/cmd/agent/main.go` : doit contenir `const AgentVersion = "1.2.0"`
   - `frontend/src/views/AptView.vue` : doit contenir `<div v-if="liveCommand">`
   - `server/internal/api/apt_stream.go` : doit exister

3. **Rebuild from scratch** :
   ```powershell
   # ATTENTION: Ceci supprime toutes les données !
   docker-compose down -v
   docker volume prune -f
   docker-compose build --no-cache
   docker-compose up -d
   
   # Puis recréer l'utilisateur admin
   docker exec -it serversupervisor-server-1 ./serversupervisor --create-admin
   ```

## Support

Si les problèmes persistent :
1. Exécutez `.\diagnostic.ps1` et sauvegardez le résultat
2. Vérifiez les logs : `docker logs serversupervisor-server-1 > server-logs.txt`
3. Vérifiez la console navigateur (F12) et sauvegardez les erreurs

---

## Migrations de base de données non appliquées

### Diagnostic du problème

Le script `docker-init.sh` est monté dans `/docker-entrypoint-initdb.d/`, mais PostgreSQL **n'exécute ces scripts que lors de la première initialisation**.

Si vous aviez déjà créé les containers avant d'ajouter les migrations, les scripts d'init n'ont jamais été exécutés.

### Solutions

#### Option 1 : Appliquer les migrations manuellement (recommandé)

Sur Linux :
```bash
# 1. Rendre le script exécutable
chmod +x apply-migrations.sh

# 2. Appliquer les migrations
./apply-migrations.sh

# 3. Vérifier
chmod +x check-db.sh
./check-db.sh
```

Sur Windows (PowerShell) :
```powershell
# Vérifier l'état actuel
.\check-db.ps1

# Trouver le nom du container PostgreSQL
docker ps | Select-String postgres

# Appliquer les migrations (remplacer <container_name>)
docker exec -it <container_name> psql -U supervisor -d serversupervisor -c "ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20);"
docker exec -it <container_name> psql -U supervisor -d serversupervisor -c "ALTER TABLE apt_status ADD COLUMN IF NOT EXISTS cve_list JSONB DEFAULT '[]'::jsonb;"
```

#### Option 2 : Recréer complètement (⚠️ perte de données)

```bash
# ATTENTION: supprime TOUTES les données
docker-compose down -v
docker-compose up -d
```

### Vérification post-migration

1. **Redémarrer les agents** sur tous les serveurs surveillés :
   ```bash
   sudo systemctl restart serversupervisor-agent
   ```

2. **Vérifier les logs agents** :
   ```bash
   journalctl -u serversupervisor-agent -f | grep "Report sent"
   ```

3. **Attendre 30-60s** pour que les agents envoient un rapport, puis rafraîchir le dashboard

### Pour éviter ce problème à l'avenir

Quand vous modifiez `docker-init.sh`, vous devez soit :
- **Recréer le volume** : `docker-compose down -v && docker-compose up -d`
- **Ou appliquer manuellement** avec `apply-migrations.sh`

Le script d'init ne s'exécute que si le répertoire de données PostgreSQL est vide.
