# GitHub Actions Workflows — ServerSupervisor

CI/CD automatisé pour les trois modules (server, agent, frontend) plus la
sécurité et les releases.

## Principe

- **Vérifier, pas réparer en douce.** Les workflows ne committent plus
  automatiquement sur la branche. Quand une dérive est détectée (go.mod non
  tidy, types générés obsolètes), le job **échoue** et l'auteur committe le
  correctif. L'arbre committé reste la seule source de vérité — y compris sur
  les pull requests.
- **Bloquant par défaut.** Lint, tests et vérifications font échouer le CI.
- **Reproductible.** Les versions d'outils (golangci-lint, govulncheck) sont
  épinglées ; rien ne dépend de `latest`.
- **Économe.** Chaque workflow a un `concurrency:` qui annule les runs
  superflus sur la même réf (sauf les releases, jamais annulées).

---

## Workflows

### CI Server (`ci-server.yml`) — push/PR sur `server/**`

1. Setup Go (version lue depuis `server/go.mod`) + cache
2. **Vérifie** `go mod tidy` (échoue si dérive)
3. **Vérifie** la synchro des types frontend générés (`tygo`) avec les modèles Go
4. `go mod verify`
5. Build : `go build ./...`
6. Tests : `go test -race -coverprofile=coverage.out ./...` (inclut les tests
   d'intégration testcontainers + Postgres/TimescaleDB)
7. Upload coverage → Codecov (`server`)
8. Lint **bloquant** : `golangci-lint` épinglé, config `server/.golangci.yml`

### CI Agent (`ci-agent.yml`) — push/PR sur `agent/**`

Identique au server (sans la régén de types) : tidy, build, tests + coverage,
lint bloquant (`agent/.golangci.yml`).

### CI Frontend (`ci-frontend.yml`) — push/PR sur `frontend/**`

- Job `quality` : `npm ci` → lint → typecheck → **tests unitaires/composants
  (`npm run test`, Vitest)** → build de production
- Job `browser-tests` : installe Chromium via Playwright puis exécute les tests
  navigateur (`npm run test:browser`, rendu Chart.js / D3)

### Security (`security.yml`) — lundi 6h UTC + push sur les manifests + manuel

- `govulncheck` (server + agent), version épinglée — **bloquant**
- `nancy` (CVE des deps Go, server + agent) — **bloquant**
- `npm audit --audit-level=moderate` (frontend) — **bloquant**
- Trivy sur l'image server (SARIF → onglet Security) — informatif
- CodeQL (`go`, `javascript`, `security-extended`)

### Release (`release.yml`) — tags `vX.Y.Z[-pre]`

- Métadonnées de version + changelog généré depuis les commits
- Binaires agent multi-arch (amd64 / arm64 / armv7 / armv6) + sha256
- Image Docker server multi-arch (amd64 / arm64) → GHCR, avec **attestations
  SBOM + provenance**
- GitHub Release avec tous les assets

### PR checks (`pr-checks.yml`) — sur chaque PR

- Titre au format Conventional Commits (bloquant)
- Auto-labeling selon les fichiers modifiés
- Avertissement si la PR dépasse 1000 lignes

### Stale (`stale.yml`) — mercredi 8h UTC

Marque/ferme les issues et PRs inactives (exemptions : `pinned`, `security`, …).

---

## Configuration

| Outil | Fichier |
|-------|---------|
| golangci-lint (server) | `server/.golangci.yml` |
| golangci-lint (agent)  | `agent/.golangci.yml` |
| ESLint (frontend)      | `frontend/eslint.config.js` (flat config, ESLint 10) |
| Dependabot             | `.github/dependabot.yml` |

Les configs golangci partent du jeu `standard` (errcheck, govet, ineffassign,
staticcheck, unused) + `bodyclose` et `unconvert`, avec les presets d'exclusion
`std-error-handling` et `common-false-positives`.

### Versions runtime (alignées)

- **Go** : `1.25` (go.mod, Dockerfile, CI)
- **Node** : `22` (Dockerfile, CI, audit)

---

## Reproduire en local

```bash
# Server / Agent
cd server   && go build ./... && go test -race ./...
cd agent    && go build ./... && go test -race ./...
golangci-lint run            # depuis server/ ou agent/

# Frontend
cd frontend && npm ci && npm run lint && npm run typecheck && npm run test && npm run build
npm run test:browser         # nécessite: npx playwright install --with-deps chromium
```
