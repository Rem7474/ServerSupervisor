# 🔧 Guide de dépannage - Migrations non appliquées

## 🔍 Diagnostic du problème

Le script `docker-init.sh` est monté dans `/docker-entrypoint-initdb.d/`, mais PostgreSQL **n'exécute ces scripts que lors de la première initialisation**.

Si vous aviez déjà créé les containers avant d'ajouter les migrations, les scripts d'init n'ont jamais été exécutés.

## ✅ Solution rapide

### Option 1 : Appliquer les migrations manuellement (recommandé, rapide)

Sur votre serveur Linux :

```bash
# 1. Rendre le script exécutable
chmod +x apply-migrations.sh

# 2. Appliquer les migrations
./apply-migrations.sh

# 3. Vérifier que tout est OK
chmod +x check-db.sh
./check-db.sh
```

### Option 2 : Recréer complètement la base de données (ATTENTION: perte de données)

```bash
# ATTENTION: Cela supprimera TOUTES les données existantes
docker-compose down -v
docker-compose up -d

# Le script docker-init.sh sera maintenant exécuté
```

### Option 3 : Sur Windows (PowerShell)

```powershell
# Vérifier l'état actuel
.\check-db.ps1

# Si les colonnes manquent, appliquez manuellement les migrations:

# Trouver le nom du container PostgreSQL
docker ps | Select-String postgres

# Appliquer les migrations (remplacez <container_name> par le nom du container)
docker exec -it <container_name> psql -U serversupervisor -d serversupervisor -c "ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20);"

docker exec -it <container_name> psql -U serversupervisor -d serversupervisor -c "ALTER TABLE apt_status ADD COLUMN IF NOT EXISTS cve_list TEXT DEFAULT '[]';"
```

## 🧪 Vérification

Après avoir appliqué les migrations :

### 1. Vérifier les colonnes (Linux)
```bash
./check-db.sh
```

### 2. Vérifier les colonnes (Windows)
```powershell
.\check-db.ps1
```

### 3. Redémarrer les agents

Sur CHAQUE serveur monitoré :
```bash
sudo systemctl restart serversupervisor-agent
```

### 4. Attendre et vérifier

- Attendez 30-60 secondes que les agents envoient un nouveau rapport
- Rafraîchissez le frontend
- Les versions des agents et CVE devraient maintenant s'afficher

## 📋 Checklist de dépannage

- [ ] Les colonnes `agent_version` et `cve_list` existent dans la base ?
  → Vérifiez avec `./check-db.sh` ou `.\check-db.ps1`

- [ ] Les agents sont-ils redémarrés après les migrations ?
  → `sudo systemctl restart serversupervisor-agent` sur chaque serveur

- [ ] Les agents envoient-ils bien des rapports ?
  → `journalctl -u serversupervisor-agent -f | grep "Report sent"`

- [ ] L'agent collecte-t-il les CVE ?
  → `journalctl -u serversupervisor-agent -f | grep CVE`

- [ ] Le frontend est-il à jour ?
  → `docker-compose build --no-cache server && docker-compose up -d server`

## 🐛 Si ça ne fonctionne toujours pas

### Vérifier que l'agent envoie `agent_version`

Sur un serveur avec l'agent, vérifiez les logs :
```bash
journalctl -u serversupervisor-agent -n 50 | grep -i version
```

Vous devriez voir : `"agent_version":"1.2.0"` dans les rapports JSON

### Vérifier que l'agent collecte les CVE

```bash
# Vérifier qu'il y a des packages de sécurité
apt list --upgradable 2>/dev/null | grep -i security

# Vérifier les logs de l'agent
journalctl -u serversupervisor-agent -f
```

Vous devriez voir : `APT: X upgradable packages (Y security, Z CVEs)`

### Vérifier manuellement la base de données

```bash
# Connexion à PostgreSQL
docker exec -it <postgres_container> psql -U serversupervisor -d serversupervisor

# Vérifier une entrée complète
SELECT * FROM hosts WHERE name = 'votre-serveur' \gx

# Vérifier les données APT avec CVE
SELECT host_id, security_updates, cve_list FROM apt_status WHERE security_updates > 0 \gx
```

## 🚀 Pour éviter ce problème à l'avenir

Quand vous modifiez `docker-init.sh`, vous devez soit :

1. **Recréer le volume** :
   ```bash
   docker-compose down -v  # Le -v supprime les volumes
   docker-compose up -d
   ```

2. **Ou appliquer manuellement** avec `apply-migrations.sh`

Le script d'init ne s'exécute que si le répertoire de données PostgreSQL est vide.

## 📞 Support

Si le problème persiste :

1. Partagez la sortie de :
   ```bash
   ./check-db.sh
   journalctl -u serversupervisor-agent -n 100
   docker-compose logs server | tail -50
   ```

2. Vérifiez que vous avez bien la dernière version :
   ```bash
   git pull
   docker-compose build --no-cache
   docker-compose up -d
   ```
