# Audit qualité/sécurité — SonarCloud (août 2026)

Analyse du projet SonarCloud `Rem7474_ServerSupervisor` (org `rem7474`, scan des 3 modules
server/agent/frontend), complétée par une revue manuelle de chaque signal à sévérité
BLOCKER/CRITICAL et par un sondage représentatif des règles à fort volume, plus une vérification
des alertes Dependabot ouvertes. Ce document répond à « qu'est-ce qui est vraiment un risque »
et pas seulement « que dit l'outil » — chaque signal ci-dessous a été vérifié en lisant le code
réel, pas supposé depuis le libellé de la règle.

## 0. Chiffres bruts (au 2026-08-28)

| Métrique | Valeur |
|---|---|
| Issues ouvertes | 871 (585 MAJOR, 212 CRITICAL, 71 MINOR, 3 BLOCKER) |
| Bugs | 247 (reliability rating **E**) |
| Vulnerabilities | 151 (security rating **E**) |
| Code smells | 473 (maintainability rating **A**) |
| Security hotspots restants | 0 |
| Duplication | 1.5 % |
| Dette technique totale | ~140h (8406 min) |
| Lignes analysées | 120 158 |

Les ratings E sur reliability/security sont **trompeurs** : ils sont dominés par deux règles à
très haut volume dont la quasi-totalité des occurrences sont des faux positifs pour ce code
précis (détail §2, §3.1). Le rating A en maintainability est plus représentatif de l'état réel.

## 1. Corrections livrées (branche `fix/sonarqube-audit-findings`, commit `0455631`)

Les 8 signaux BLOCKER/CRITICAL de type BUG/VULNERABILITY ont tous été revus individuellement :

| Règle | Fichier | Verdict | Action |
|---|---|---|---|
| `go:S4830`/`go:S5527` | `server/internal/synthetic/ssl.go:105` | **Vrai manque** | Corrigé — voir §1.1 |
| `javascript:S2819` | `frontend/public/service-worker.js:258` | **Vrai manque (mineur)** | Corrigé — voir §1.2 |
| `typescript:S2871` | `useGlobalScheduledTasks.ts:176` | **Faux positif bénin** | Corrigé par clarté — voir §1.3 |
| `go:S6437` + `secrets:S8215` | `db_hosts.go:108` | **Faux positif** | `NOSONAR` + justification |
| `go:S4790` | `db_web_logs.go:179` | **Faux positif** | `NOSONAR` + justification |
| `plsql:DeleteOrUpdateWithoutWhereCheck` | `migrations/089_...sql:12` | **Faux positif** | Non modifié (voir §1.4) |

### 1.1 SSL/TLS — vérification hostname/chaîne absente (réel)

`checkCertificate` (sonde SSL) utilise `InsecureSkipVerify: true`, volontairement, pour pouvoir
lire un certificat même expiré/invalide plutôt que voir le handshake TLS échouer. Mais avant le
correctif, le seul contrôle réellement rapporté dans `last_error` était l'expiration : un
certificat avec un **mauvais hostname** ou signé par une **CA non fiable** repassait comme
« aucune erreur ». Pour un outil dont c'est justement le métier (alerter sur un problème SSL),
c'est un vrai trou fonctionnel, pas juste un signal bruyant.

Correctif : après le handshake, `leaf.Verify()` est appelé indépendamment (hostname + chaîne),
et toute erreur autre que l'expiration déjà couverte est ajoutée à `last_error`. Aucun champ de
modèle ajouté, donc aucune régénération de types/migration nécessaire.

### 1.2 Service worker — message sans vérification d'origine (réel, risque faible)

Le listener `message` du service worker exécutait `SKIP_WAITING`/`CLEAR_CACHE` sans vérifier
`event.origin`. Le risque concret est faible (un service worker n'est de toute façon contrôlable
que par des clients same-origin), mais c'est une défense en profondeur peu coûteuse — ajoutée.

### 1.3 Tri alphabétique par défaut (faux positif bénin, corrigé par clarté)

`hostList` trie des `string` (`host_name`), donc `Array.sort()` par défaut est déjà correct
(la règle S2871 cible le bug classique `[3,1,10].sort()`, pas ce cas). Remplacé par un
`localeCompare` explicite : correct fonctionnellement à l'identique, en plus robuste aux accents
et à la casse, et ça éteint le signal proprement plutôt que par suppression.

### 1.4 Faux positifs documentés

- **`db_hosts.go:108`** — le « secret » détecté est un hash bcrypt d'une valeur arbitraire,
  utilisé uniquement pour normaliser le temps de réponse (empêcher un attaquant de distinguer
  "host inconnu" de "mauvais secret" par timing). Rien n'est réellement hashé pour le produire,
  il n'y a donc rien à « révoquer ». `NOSONAR` + commentaire ajoutés.
- **`db_web_logs.go:179`** — MD5 utilisé comme clé de déduplication non-cryptographique
  (fingerprint de requête web), jamais dans une décision de sécurité. `NOSONAR` + commentaire
  ajoutés.
- **`migrations/089_audit_logs_category.sql:12`** — `UPDATE audit_logs SET category = CASE ...`
  sans `WHERE` est volontaire : c'est un backfill de la colonne qu'on vient d'ajouter sur
  **toutes** les lignes existantes. **Non modifié** : une migration déjà appliquée ne doit
  jamais être éditée (`server/CLAUDE.md`) — le signal reste affiché mais est sans objet.

## 2. Le volume MAJOR est dominé par deux patterns systématiques, pas 600 bugs distincts

### 2.1 `go:S2077` — 67 occurrences « requête SQL formatée dynamiquement »

Échantillon couvrant tous les fichiers distincts touchés (`db_docker_image_versions.go`,
`db_network_flows.go`, `db_backup.go`, `db_web_logs_summary.go`, `db_web_logs_detail.go`,
`db_commands.go`, `testutil/postgres.go`) : dans chaque cas, `fmt.Sprintf`/`+` ne sert qu'à
assembler un **fragment de requête fixe** (clause `WHERE`/`ORDER BY` conditionnelle) — les
valeurs elles-mêmes passent toujours par des placeholders `$N` paramétrés. Les deux points où on
pourrait craindre une injection réelle (colonne de tri, direction de tri dans
`db_web_logs_detail.go:243-248`, bucket temporel dans `db_web_logs_summary.go:811`) sont
allowlistés (map statique ou comparaison stricte) avant interpolation. **Aucune injection SQL
trouvée** dans l'échantillon couvert — c'est un faux positif systématique de la règle sur ce
pattern de construction de requêtes, pas une dette réelle.

**Recommandation** : ne pas corriger fichier par fichier. Soit exclure `go:S2077` du profil
Sonar pour `server/internal/database` avec une justification versionnée, soit — mieux à terme —
centraliser la construction des clauses `WHERE`/`ORDER BY` dynamiques dans un petit helper qui
rend le pattern visible en un seul endroit pour un futur audit.

### 2.2 `Web:InputWithoutLabelCheck` (234) + `Web:S6853` (216) — 450 issues, accessibilité

Plus de la moitié de toutes les issues ouvertes. Ce sont des `<input>`/`<select>` du frontend
sans `<label for>`/`aria-label` associé — un vrai gap d'accessibilité (lecteurs d'écran), mais
qui se répète très probablement sur un nombre restreint de *patterns* de composants (filtres de
table, champs de formulaire modal, toggles) plutôt que 450 cas uniques à traiter un par un.

**Recommandation** : avant de corriger, faire un passage de *tri par composant* (facette
`directories`/`files` de l'API Sonar) pour identifier les 5-10 composants qui concentrent le
plus d'occurrences (probablement des composants de filtre/formulaire réutilisés). Corriger ces
composants partagés en premier réduit le compteur bien plus vite que traiter vue par vue. Le
frontend a déjà la convention `form-label` (`frontend/CLAUDE.md`) — le gap est l'application
systématique, pas l'absence de convention.

### 2.3 `go:S3776`/`typescript:S3776` (94) — complexité cognitive

Fonctions trop complexes (souvent les handlers/composables les plus anciens du projet). Pas un
risque de sécurité, un risque de maintenabilité. Contrairement aux deux catégories ci-dessus, un
refactor de complexité cognitive change le flot de contrôle réel — **à traiter fonction par
fonction avec les tests existants comme filet**, pas en masse. Ne pas lancer un refactor
automatisé dessus.

### 2.4 `go:S4036` (34, MINOR) — recherche de binaire via `$PATH`

Concerne l'agent (`update.go`, `apt.go`, `disk.go`, `systemd.go`, `restic.go`, `system.go`) :
`exec.Command("nom", ...)` sans chemin absolu. Risque réel mais mineur et cohérent avec le
modèle de menace de l'agent (déjà privilégié, déjà root ou quasi — voir `agent/CLAUDE.md`) : un
`$PATH` déjà compromis sur la machine cible implique une compromission plus large que ce que ce
correctif éviterait. Sévérité MINOR confirmée cohérente. Pas prioritaire.

## 3. Dépendances (Dependabot, hors SonarCloud)

3 alertes ouvertes à haute sévérité sur `main`, vérifiées une par une :

| Package | Manifest | Plage vulnérable | Version résolue actuelle |
|---|---|---|---|
| `github.com/moby/go-archive` | `server/go.mod` | `< 0.3.0` | `0.3.3` — **déjà patché** |
| `github.com/moby/go-archive` | `agent/go.mod` | `< 0.3.0` | `0.3.3` — **déjà patché** |
| `nanoid` | `frontend/package-lock.json` | `< 3.3.18` | `3.3.18` — **déjà patché** |

Les 3 alertes sont **obsolètes** : les versions réellement résolues dans `go.sum`/
`package-lock.json` sont déjà au-delà du correctif. Aucune action de code nécessaire — à
dismiss côté GitHub (ou attendre le prochain rescan Dependabot). Accessoirement : côté agent,
`go-archive` vient de `go-dockerclient` (donc potentiellement en chemin de prod via le
collecteur Docker), côté server uniquement de `testcontainers-go` (donc test-only,
`internal/testutil`) — la distinction n'a plus d'importance ici puisque déjà patché des deux
côtés, mais utile à noter pour une prochaine alerte sur ce package.

## 4. Plan de suite recommandé

1. **Fait** (`fix/sonarqube-audit-findings`) : les 8 signaux BLOCKER/CRITICAL BUG/VULNERABILITY.
2. **Prochain, faible effort** : configurer une exclusion Sonar justifiée pour `go:S2077` sur
   `server/internal/database/**` (voir §2.1) — fait baisser le compteur MAJOR de ~65 sans toucher
   au code.
3. **Prochain, effort moyen, fort impact utilisateur** : accessibilité — identifier les
   composants concentrant le plus d'occurrences de `Web:InputWithoutLabelCheck`/`S6853` (§2.2)
   et les corriger en priorité.
4. **En continu, pas en rafale** : complexité cognitive (§2.3) — un refactor à la fois, avec
   tests, quand on touche de toute façon à la fonction concernée pour une autre raison.
5. **Non prioritaire** : `go:S4036` (§2.4), `go:S1192` (duplication de chaînes littérales —
   maintenabilité pure, sans risque).
