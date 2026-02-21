#!/bin/bash
# Script pour appliquer manuellement les migrations de base de données

set -e

echo "========================================"
echo "Application des migrations"
echo "========================================"
echo ""

# Find PostgreSQL container
PG_CONTAINER=$(docker ps --filter "name=postgres" --format "{{.Names}}" | head -n 1)

if [ -z "$PG_CONTAINER" ]; then
    PG_CONTAINER=$(docker ps --filter "name=serversupervisor" --format "{{.Names}}" | grep -i postgres | head -n 1)
fi

if [ -z "$PG_CONTAINER" ]; then
    echo "❌ Conteneur PostgreSQL non trouvé"
    echo ""
    echo "Containers en cours d'exécution:"
    docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
    exit 1
fi

echo "✓ Conteneur PostgreSQL: $PG_CONTAINER"
echo ""

# Apply migrations
echo "[1/2] Migration: agent_version column..."
docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor <<-EOSQL
    ALTER TABLE hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT NULL;
    CREATE INDEX IF NOT EXISTS idx_hosts_agent_version ON hosts(agent_version);
EOSQL

if [ $? -eq 0 ]; then
    echo "✓ Migration agent_version appliquée"
else
    echo "❌ Erreur lors de la migration agent_version"
    exit 1
fi

echo ""
echo "[2/2] Migration: cve_list column..."
docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor <<-EOSQL
    ALTER TABLE apt_status ADD COLUMN IF NOT EXISTS cve_list TEXT DEFAULT '[]';
    UPDATE apt_status SET cve_list = '[]' WHERE cve_list IS NULL;
EOSQL

if [ $? -eq 0 ]; then
    echo "✓ Migration cve_list appliquée"
else
    echo "❌ Erreur lors de la migration cve_list"
    exit 1
fi

echo ""
echo "========================================"
echo "✓ Toutes les migrations sont appliquées"
echo "========================================"
echo ""
echo "Prochaines étapes:"
echo "1. Redémarrez les agents: sudo systemctl restart serversupervisor-agent"
echo "2. Attendez 30-60 secondes"
echo "3. Vérifiez avec: ./check-db.sh"
echo ""
