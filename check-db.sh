#!/bin/bash
# Script de diagnostic de la base de données

set -e

echo "========================================"
echo "Diagnostic de la base de données"
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

# Check agent_version column
echo ""
echo "[1/3] Vérification de la colonne agent_version..."
AGENT_VERSION_CHECK=$(docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -t <<-EOSQL
    SELECT column_name, data_type, column_default 
    FROM information_schema.columns 
    WHERE table_name = 'hosts' AND column_name = 'agent_version';
EOSQL
)

if echo "$AGENT_VERSION_CHECK" | grep -q "agent_version"; then
    echo "✓ Colonne agent_version existe"
    
    HOSTS_WITH_VERSION=$(docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -t -c "SELECT COUNT(*) FROM hosts WHERE agent_version IS NOT NULL;")
    echo "  Hôtes avec version: $(echo $HOSTS_WITH_VERSION | tr -d ' ')"
    
    echo ""
    echo "  Détails:"
    docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -c "SELECT id, name, hostname, agent_version FROM hosts ORDER BY name;"
else
    echo "❌ Colonne agent_version n'existe PAS"
    echo ""
    echo "  >> SOLUTION: Exécutez:"
    echo "     ./apply-migrations.sh"
fi

# Check cve_list column
echo ""
echo "[2/3] Vérification de la colonne cve_list..."
CVE_LIST_CHECK=$(docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -t <<-EOSQL
    SELECT column_name, data_type, column_default 
    FROM information_schema.columns 
    WHERE table_name = 'apt_status' AND column_name = 'cve_list';
EOSQL
)

if echo "$CVE_LIST_CHECK" | grep -q "cve_list"; then
    echo "✓ Colonne cve_list existe"
    
    HOSTS_WITH_CVE=$(docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -t -c "SELECT COUNT(*) FROM apt_status WHERE cve_list IS NOT NULL AND cve_list != '[]';")
    echo "  Hôtes avec CVE: $(echo $HOSTS_WITH_CVE | tr -d ' ')"
    
    echo ""
    echo "  Échantillon de données CVE:"
    docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -c "SELECT host_id, security_updates, SUBSTRING(cve_list, 1, 100) as cve_preview FROM apt_status WHERE security_updates > 0 LIMIT 3;"
else
    echo "❌ Colonne cve_list n'existe PAS"
    echo ""
    echo "  >> SOLUTION: Exécutez:"
    echo "     ./apply-migrations.sh"
fi

# Check apt_status data
echo ""
echo "[3/3] Vérification des données apt_status..."
docker exec -i $PG_CONTAINER psql -U serversupervisor -d serversupervisor -c "SELECT host_id, pending_packages, security_updates, LENGTH(cve_list) as cve_length FROM apt_status LIMIT 5;"

# Summary
echo ""
echo "========================================"
echo "Résumé"
echo "========================================"

if echo "$AGENT_VERSION_CHECK" | grep -q "agent_version" && echo "$CVE_LIST_CHECK" | grep -q "cve_list"; then
    echo "✓ Toutes les colonnes existent"
    echo ""
    echo "Si les données ne s'affichent toujours pas:"
    echo "1. Vérifiez que les agents sont redémarrés:"
    echo "   sudo systemctl restart serversupervisor-agent"
    echo "2. Vérifiez les logs de l'agent:"
    echo "   journalctl -u serversupervisor-agent -f"
    echo "3. Attendez 30-60 secondes que les agents envoient un nouveau rapport"
else
    echo "❌ Migrations manquantes"
    echo ""
    echo "Exécutez: ./apply-migrations.sh"
    echo ""
    echo "Ou recréez complètement la base:"
    echo "  docker-compose down -v"
    echo "  docker-compose up -d"
fi

echo ""
echo "========================================"
echo ""
