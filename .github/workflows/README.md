# 🤖 GitHub Actions Workflows - ServerSupervisor

Ce dossier contient les workflows CI/CD automatisés pour ServerSupervisor avec **auto-fix intégré**.

## 📚 Table des matières

- [Vue d'ensemble](#vue-densemble)
- [Workflows disponibles](#workflows-disponibles)
- [Fonctionnalités auto-fix](#fonctionnalités-auto-fix)
- [Configuration](#configuration)
- [Déclenchement](#déclenchement)
- [Dépannage](#dépannage)

---

## 🎯 Vue d'ensemble

Les workflows GitHub Actions de ce projet :

✅ **Corrigent automatiquement** les problèmes courants  
✅ **Commitent les corrections** avec `[skip ci]` pour éviter les boucles  
✅ **Exécutent tests et linting** sur chaque push/PR  
✅ **Utilisent le cache** pour accélérer les builds  

---

## 🛠️ Workflows disponibles

### 1. **CI Agent** (`.github/workflows/ci-agent.yml`)

**Déclenchement** : Push/PR sur `agent/**`

**Étapes** :
1. ✅ Checkout du code
2. ⚙️ Setup Go 1.22 avec cache
3. 🔧 **Auto-fix** : `go mod tidy`
4. 💾 Commit auto si `go.mod`/`go.sum` modifiés
5. 🛠️ Build : `go build ./...`
6. 🧪 Tests : `go test -race -coverprofile=coverage.out`
7. 🔍 Lint : `golangci-lint run --fix`
8. 💾 Commit auto des corrections linter
9. 📈 Upload coverage vers Codecov

**Corrections automatiques** :
- Synchronisation `go.mod` et `go.sum`
- Ajout vérifications erreurs (`errcheck`)
- Suppression variables inutiles (`ineffassign`)
- Correction blocs vides (`staticcheck`)

---

### 2. **CI Server** (`.github/workflows/ci-server.yml`)

**Déclenchement** : Push/PR sur `server/**`

**Étapes** : Identiques à CI Agent

**Corrections automatiques** : Mêmes que CI Agent

---

### 3. **CI Frontend** (`.github/workflows/ci-frontend.yml`)

**Déclenchement** : Push/PR sur `frontend/**`

**Étapes** :
1. ✅ Checkout du code
2. ⚙️ Setup Node.js 20 avec cache npm
3. 🔧 **Auto-fix** : Régénération `package-lock.json` si corrompu
4. 💾 Commit auto si `package-lock.json` régénéré
5. 📦 Install : `npm ci --prefer-offline`
6. 🔍 Lint : `npm run lint -- --fix`
7. 💾 Commit auto des corrections ESLint
8. 🛠️ Build : `npm run build`
9. 🧪 Tests : `npm test` (si présents)

**Corrections automatiques** :
- Régénération `package-lock.json`
- Fixes ESLint (indentation, quotes, etc.)
- Ajout semicolons, suppression imports inutilisés

---

## ✨ Fonctionnalités auto-fix

### 🐛 Problèmes corrigés automatiquement

| Composant | Problème | Solution auto |
|-----------|----------|---------------|
| **Agent/Server** | `go.mod` désynchronisé | `go mod tidy` + commit |
| **Agent/Server** | Erreurs non vérifiées | `golangci-lint --fix` |
| **Agent/Server** | Variables inutiles | Suppression auto |
| **Frontend** | `package-lock.json` corrompu | Régénération + commit |
| **Frontend** | Style code inconsistant | `eslint --fix` |

### 🔒 Sécurité : `[skip ci]`

Tous les commits automatiques incluent `[skip ci]` pour **éviter les boucles infinies** :

```bash
# Exemple de commit auto
git commit -m "chore(agent): auto-fix go mod tidy [skip ci]"
```

➡️ Ce commit **ne déclenchera pas** un nouveau workflow.

---

## ⚙️ Configuration

### Linters Go

Fichiers de configuration :
- `agent/.golangci.yml`
- `server/.golangci.yml`

**Linters activés** :
```yaml
linters:
  enable:
    - errcheck       # Vérifie gestion erreurs
    - ineffassign    # Détecte assignments inutiles
    - staticcheck    # Analyse statique approfondie
    - govet          # Outil officiel Go
    - unused         # Code non utilisé
    - gosimple       # Simplifications possibles
```

**Auto-fix activé** :
```yaml
issues:
  fix: true  # Corrige automatiquement ce qui peut l'être
```

### Linter JavaScript/Vue

Fichier de configuration : `frontend/.eslintrc.cjs`

**Configuration** :
```javascript
module.exports = {
  extends: [
    'plugin:vue/vue3-essential',
    'eslint:recommended'
  ],
  rules: {
    'no-unused-vars': 'warn',
    'no-console': 'off',
    'vue/multi-word-component-names': 'off'
  }
}
```

**Scripts disponibles** :
```bash
npm run lint       # Vérifie le code
npm run lint:fix   # Corrige automatiquement
```

---

## 🚀 Déclenchement

### Push sur `main` ou `develop`

```bash
# Modification dans agent/
cd agent
echo "// test" >> main.go
git add main.go
git commit -m "feat: add feature"
git push origin main
```

➡️ Déclenche **CI Agent** uniquement

### Pull Request

```bash
git checkout -b feature/ma-feature
# ... modifications ...
git push origin feature/ma-feature
# Créer PR sur GitHub
```

➡️ Déclenche workflows selon fichiers modifiés  
⚠️ **Pas de commit auto sur PR** (pour éviter modifications non sollicitées)

### Déclenchement manuel

Via l'interface GitHub :
1. Aller dans **Actions**
2. Sélectionner un workflow
3. Cliquer **Run workflow**
4. Choisir la branche
5. Cliquer **Run workflow**

---

## 🔍 Surveillance

### Visualiser les exécutions

1. Aller sur [github.com/Rem7474/ServerSupervisor/actions](https://github.com/Rem7474/ServerSupervisor/actions)
2. Sélectionner un workflow
3. Voir l'historique et les logs

### Badges de statut

Ajouter dans le `README.md` principal :

```markdown
![CI Agent](https://github.com/Rem7474/ServerSupervisor/workflows/CI%20Agent/badge.svg)
![CI Server](https://github.com/Rem7474/ServerSupervisor/workflows/CI%20Server/badge.svg)
![CI Frontend](https://github.com/Rem7474/ServerSupervisor/workflows/CI%20Frontend/badge.svg)
```

---

## 🚫 Dépannage

### Problème : Workflow ne se déclenche pas

**Cause** : Fichiers modifiés hors du scope

**Solution** : Vérifier les `paths` dans le workflow

```yaml
on:
  push:
    paths:
      - 'agent/**'  # Ne se déclenche que si agent/ modifié
```

### Problème : Commit auto échoue

**Erreur** : `Permission denied`

**Solution** : Vérifier que `permissions: contents: write` est présent

```yaml
permissions:
  contents: write  # Nécessaire pour push auto
```

### Problème : Boucle infinie de commits

**Cause** : `[skip ci]` manquant dans le message de commit

**Solution** : Vérifier tous les commits auto incluent `[skip ci]` :

```bash
git commit -m "chore: auto-fix [skip ci]"  # ✅ Correct
git commit -m "chore: auto-fix"            # ❌ Boucle infinie
```

### Problème : Linter échoue malgré auto-fix

**Cause** : Certains problèmes ne peuvent pas être corrigés automatiquement

**Solution** : Corriger manuellement et commit

**Exemple** : Logique métier incorrecte détectée par `staticcheck`

```bash
# Voir les erreurs dans les logs Actions
# Corriger localement
npm run lint:fix  # ou golangci-lint run --fix
git add .
git commit -m "fix: correct linter issues"
git push
```

### Problème : Cache ne fonctionne pas

**Cause** : `go.sum` ou `package-lock.json` modifié

**Solution** : Le cache sera automatiquement recréé au prochain run

---

## 📚 Ressources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [golangci-lint](https://golangci-lint.run/)
- [ESLint](https://eslint.org/)
- [Vue ESLint Plugin](https://eslint.vuejs.org/)

---

## ✅ Checklist maintenance

- [ ] Vérifier workflows s'exécutent correctement chaque semaine
- [ ] Mettre à jour versions actions (setup-go, setup-node) tous les 3 mois
- [ ] Revoir règles linter tous les 6 mois
- [ ] Ajouter nouveaux tests au fur et à mesure
- [ ] Documenter nouvelles règles ajoutées

---

**🚀 Happy coding with auto-fix CI !**
