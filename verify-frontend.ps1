# Script to verify frontend deployment
# Checks if the latest features are present in the deployed frontend

Write-Host "=== Frontend Verification Script ===" -ForegroundColor Cyan
Write-Host ""

$SERVER_CONTAINER = docker ps --format "{{.Names}}" | Where-Object { $_ -like "*server*" } | Select-Object -First 1

if (-not $SERVER_CONTAINER) {
    Write-Host "Error: Server container not found" -ForegroundColor Red
    Write-Host "Make sure your docker-compose services are running" -ForegroundColor Yellow
    exit 1
}

Write-Host "Found server container: $SERVER_CONTAINER" -ForegroundColor Green
Write-Host ""

# Check if frontend directory exists
Write-Host "1. Checking frontend structure..." -ForegroundColor Yellow
$frontendExists = docker exec $SERVER_CONTAINER test -d /app/frontend/dist && echo "exists" || echo "missing"

if ($frontendExists -eq "missing") {
    Write-Host "   ✗ Frontend directory is MISSING" -ForegroundColor Red
    Write-Host "   → Rebuild server: docker-compose build --no-cache server" -ForegroundColor Yellow
    exit 1
} else {
    Write-Host "   ✓ Frontend directory exists" -ForegroundColor Green
}

# Check for key files
Write-Host ""
Write-Host "2. Checking critical files..." -ForegroundColor Yellow

$files = @{
    "index.html" = "/app/frontend/dist/index.html"
    "AptView" = "/app/frontend/dist/assets/index-*.js"
}

foreach ($file in $files.GetEnumerator()) {
    $exists = docker exec $SERVER_CONTAINER sh -c "ls $($file.Value) 2>/dev/null"
    if ($exists) {
        Write-Host "   ✓ $($file.Key) found" -ForegroundColor Green
    } else {
        Write-Host "   ✗ $($file.Key) NOT FOUND" -ForegroundColor Red
    }
}

# Check for console live feature in JavaScript
Write-Host ""
Write-Host "3. Checking for APT console feature in JavaScript..." -ForegroundColor Yellow
$jsFiles = docker exec $SERVER_CONTAINER sh -c "ls /app/frontend/dist/assets/index-*.js 2>/dev/null"

if ($jsFiles) {
    $hasConsole = docker exec $SERVER_CONTAINER sh -c "grep -l 'Console Live' /app/frontend/dist/assets/index-*.js 2>/dev/null"
    $hasWatchCommand = docker exec $SERVER_CONTAINER sh -c "grep -l 'watchCommand' /app/frontend/dist/assets/index-*.js 2>/dev/null"
    
    if ($hasConsole) {
        Write-Host "   ✓ Console Live UI code found" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Console Live UI code NOT FOUND" -ForegroundColor Red
        Write-Host "   → Frontend needs to be rebuilt with latest code" -ForegroundColor Yellow
    }
    
    if ($hasWatchCommand) {
        Write-Host "   ✓ watchCommand function found" -ForegroundColor Green
    } else {
        Write-Host "   ✗ watchCommand function NOT FOUND" -ForegroundColor Red
    }
} else {
    Write-Host "   ✗ No JavaScript files found" -ForegroundColor Red
}

# Check for agent version display
Write-Host ""
Write-Host "4. Checking for agent version feature..." -ForegroundColor Yellow

if ($jsFiles) {
    $hasAgentVersion = docker exec $SERVER_CONTAINER sh -c "grep -l 'agent_version' /app/frontend/dist/assets/index-*.js 2>/dev/null"
    $hasLatestVersion = docker exec $SERVER_CONTAINER sh -c "grep -l 'LATEST_AGENT_VERSION' /app/frontend/dist/assets/index-*.js 2>/dev/null"
    
    if ($hasAgentVersion) {
        Write-Host "   ✓ Agent version display code found" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Agent version display code NOT FOUND" -ForegroundColor Red
    }
    
    if ($hasLatestVersion) {
        Write-Host "   ✓ Version comparison logic found" -ForegroundColor Green
    } else {
        Write-Host "   ✗ Version comparison logic NOT FOUND" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "=== Verification Complete ===" -ForegroundColor Cyan
Write-Host ""

# Recommendations
Write-Host "Recommendations:" -ForegroundColor Yellow
Write-Host "• If features are missing, rebuild frontend:" -ForegroundColor White
Write-Host "  cd frontend && npm install && npm run build" -ForegroundColor Cyan
Write-Host ""
Write-Host "• Then rebuild Docker image:" -ForegroundColor White
Write-Host "  docker-compose build --no-cache server" -ForegroundColor Cyan
Write-Host "  docker-compose up -d" -ForegroundColor Cyan
