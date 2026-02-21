# Diagnostic script for ServerSupervisor
# Checks if everything is properly configured

Write-Host "=== ServerSupervisor Diagnostic Tool ===" -ForegroundColor Cyan
Write-Host ""

# Check Docker
Write-Host "1. Checking Docker..." -ForegroundColor Yellow
try {
    $dockerVersion = docker --version
    Write-Host "   ✓ Docker is running: $dockerVersion" -ForegroundColor Green
} catch {
    Write-Host "   ✗ Docker is not accessible" -ForegroundColor Red
    exit 1
}

# Check running containers
Write-Host ""
Write-Host "2. Checking containers..." -ForegroundColor Yellow
$containers = docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
if ($containers) {
    Write-Host $containers
} else {
    Write-Host "   ✗ No containers running. Start with: docker-compose up -d" -ForegroundColor Red
}

# Check PostgreSQL
Write-Host ""
Write-Host "3. Checking PostgreSQL database..." -ForegroundColor Yellow
$POSTGRES_CONTAINER = docker ps --format "{{.Names}}" | Where-Object { $_ -like "*postgres*" } | Select-Object -First 1

if ($POSTGRES_CONTAINER) {
    Write-Host "   ✓ PostgreSQL container found: $POSTGRES_CONTAINER" -ForegroundColor Green
    
    # Check if agent_version column exists
    $checkColumn = @"
SELECT EXISTS (
    SELECT 1 FROM information_schema.columns 
    WHERE table_name='hosts' AND column_name='agent_version'
);
"@
    
    $result = $checkColumn | docker exec -i $POSTGRES_CONTAINER psql -U supervisor -d serversupervisor -t -A
    
    if ($result -match "t") {
        Write-Host "   ✓ agent_version column exists" -ForegroundColor Green
    } else {
        Write-Host "   ✗ agent_version column is MISSING" -ForegroundColor Red
        Write-Host "   → Run .\migrate.ps1 to add it" -ForegroundColor Yellow
    }
    
    # Check hosts table
    $hostsQuery = @"
SELECT COUNT(*) as total,
       COUNT(agent_version) as with_version,
       COUNT(*) - COUNT(agent_version) as without_version
FROM hosts;
"@
    
    Write-Host ""
    Write-Host "   Hosts status:" -ForegroundColor Cyan
    $hostsQuery | docker exec -i $POSTGRES_CONTAINER psql -U supervisor -d serversupervisor
    
} else {
    Write-Host "   ✗ PostgreSQL container not found" -ForegroundColor Red
}

# Check Server container
Write-Host ""
Write-Host "4. Checking Server container..." -ForegroundColor Yellow
$SERVER_CONTAINER = docker ps --format "{{.Names}}" | Where-Object { $_ -like "*server*" } | Select-Object -First 1

if ($SERVER_CONTAINER) {
    Write-Host "   ✓ Server container found: $SERVER_CONTAINER" -ForegroundColor Green
    
    # Check if frontend is built
    Write-Host "   Checking frontend files..." -ForegroundColor Cyan
    $frontendCheck = docker exec $SERVER_CONTAINER ls -la /app/frontend/dist/index.html 2>&1
    if ($frontendCheck -notmatch "No such file") {
        Write-Host "   ✓ Frontend files are present" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Frontend files are MISSING" -ForegroundColor Red
        Write-Host "   → Rebuild with: docker-compose build --no-cache server" -ForegroundColor Yellow
    }
    
    # Check server version
    Write-Host "   Checking server binary..." -ForegroundColor Cyan
    docker exec $SERVER_CONTAINER ./serversupervisor --version 2>&1 | Out-Null
    
} else {
    Write-Host "   ✗ Server container not found" -ForegroundColor Red
}

# Check Server URL
Write-Host ""
Write-Host "5. Checking Server accessibility..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/api/health" -UseBasicParsing -TimeoutSec 5
    if ($response.StatusCode -eq 200) {
        Write-Host "   ✓ Server is accessible at http://localhost:8080" -ForegroundColor Green
    }
} catch {
    Write-Host "   ✗ Server is not accessible at http://localhost:8080" -ForegroundColor Red
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
Write-Host "=== Diagnostic Complete ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Common fixes:" -ForegroundColor Yellow
Write-Host "1. Missing agent_version: Run .\migrate.ps1" -ForegroundColor White
Write-Host "2. Frontend issues: docker-compose build --no-cache server" -ForegroundColor White
Write-Host "3. Not seeing versions: Restart agents to send updated reports" -ForegroundColor White
Write-Host "4. APT logs not working: Check browser console (F12) for JavaScript errors" -ForegroundColor White
