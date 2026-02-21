# Script de diagnostic de la base de données
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Diagnostic de la base de données" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Check if Docker is running
$dockerRunning = docker ps 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Docker n'est pas en cours d'exécution" -ForegroundColor Red
    exit 1
}

# Find PostgreSQL container
$pgContainer = docker ps --filter "name=postgres" --format "{{.Names}}" 2>$null | Select-Object -First 1
if (-not $pgContainer) {
    $pgContainer = docker ps --filter "name=serversupervisor" --format "{{.Names}}" 2>$null | Where-Object { $_ -match "postgres" } | Select-Object -First 1
}

if (-not $pgContainer) {
    Write-Host "❌ Conteneur PostgreSQL non trouvé" -ForegroundColor Red
    Write-Host "`nContainers en cours d'exécution:" -ForegroundColor Yellow
    docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
    exit 1
}

Write-Host "✓ Conteneur PostgreSQL: $pgContainer" -ForegroundColor Green

# Check agent_version column in hosts table
Write-Host "`n[1/3] Vérification de la colonne agent_version..." -ForegroundColor Yellow
$agentVersionCheck = @"
SELECT column_name, data_type, column_default 
FROM information_schema.columns 
WHERE table_name = 'hosts' AND column_name = 'agent_version';
"@ | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor -t 2>&1

if ($agentVersionCheck -match "agent_version") {
    Write-Host "✓ Colonne agent_version existe" -ForegroundColor Green
    
    # Check if any hosts have agent_version
    $hostsWithVersion = "SELECT COUNT(*) FROM hosts WHERE agent_version IS NOT NULL;" | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor -t 2>&1
    Write-Host "  Hôtes avec version: $($hostsWithVersion.Trim())" -ForegroundColor Cyan
    
    # Show all hosts and their versions
    $allHosts = "SELECT id, name, hostname, agent_version FROM hosts ORDER BY name;" | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor 2>&1
    Write-Host "`n  Détails:" -ForegroundColor Cyan
    Write-Host $allHosts
} else {
    Write-Host "❌ Colonne agent_version n'existe PAS" -ForegroundColor Red
    Write-Host "`n  >> SOLUTION: Exécutez la migration manuellement:" -ForegroundColor Yellow
    Write-Host "     docker exec -it $pgContainer psql -U serversupervisor -d serversupervisor -c 'ALTER TABLE hosts ADD COLUMN agent_version VARCHAR(20);'" -ForegroundColor White
}

# Check cve_list column in apt_status table
Write-Host "`n[2/3] Vérification de la colonne cve_list..." -ForegroundColor Yellow
$cveListCheck = @"
SELECT column_name, data_type, column_default 
FROM information_schema.columns 
WHERE table_name = 'apt_status' AND column_name = 'cve_list';
"@ | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor -t 2>&1

if ($cveListCheck -match "cve_list") {
    Write-Host "✓ Colonne cve_list existe" -ForegroundColor Green
    
    # Check if any hosts have CVE data
    $hostsWithCVE = "SELECT COUNT(*) FROM apt_status WHERE cve_list IS NOT NULL AND cve_list != '[]';" | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor -t 2>&1
    Write-Host "  Hôtes avec CVE: $($hostsWithCVE.Trim())" -ForegroundColor Cyan
    
    # Show sample CVE data
    $cveData = "SELECT host_id, security_updates, SUBSTRING(cve_list, 1, 100) as cve_preview FROM apt_status WHERE security_updates > 0 LIMIT 3;" | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor 2>&1
    Write-Host "`n  Échantillon de données CVE:" -ForegroundColor Cyan
    Write-Host $cveData
} else {
    Write-Host "❌ Colonne cve_list n'existe PAS" -ForegroundColor Red
    Write-Host "`n  >> SOLUTION: Exécutez la migration manuellement:" -ForegroundColor Yellow
    Write-Host "     docker exec -it $pgContainer psql -U serversupervisor -d serversupervisor -c ""ALTER TABLE apt_status ADD COLUMN cve_list TEXT DEFAULT '[]';""`n" -ForegroundColor White
}

# Check apt_status data
Write-Host "`n[3/3] Vérification des données apt_status..." -ForegroundColor Yellow
$aptStatus = "SELECT host_id, pending_packages, security_updates, LENGTH(cve_list) as cve_length FROM apt_status LIMIT 5;" | docker exec -i $pgContainer psql -U serversupervisor -d serversupervisor 2>&1
Write-Host $aptStatus

# Summary
Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Résumé" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

if ($agentVersionCheck -match "agent_version" -and $cveListCheck -match "cve_list") {
    Write-Host "✓ Toutes les colonnes existent" -ForegroundColor Green
    Write-Host "`nSi les données ne s'affichent toujours pas:" -ForegroundColor Yellow
    Write-Host "1. Vérifiez que les agents sont redémarrés: sudo systemctl restart serversupervisor-agent" -ForegroundColor White
    Write-Host "2. Vérifiez les logs de l'agent: journalctl -u serversupervisor-agent -f" -ForegroundColor White
    Write-Host "3. Attendez 30-60 secondes que les agents envoient un nouveau rapport" -ForegroundColor White
} else {
    Write-Host "❌ Migrations manquantes - Exécutez les commandes SQL ci-dessus" -ForegroundColor Red
    Write-Host "`nOu recréez complètement la base:" -ForegroundColor Yellow
    Write-Host "docker-compose down -v" -ForegroundColor White
    Write-Host "docker-compose up -d" -ForegroundColor White
}

Write-Host "`n========================================`n" -ForegroundColor Cyan
