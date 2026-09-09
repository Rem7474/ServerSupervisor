# Architecture et Interface d'Administration de la Configuration (Docker & Système)

Ce document décrit l'architecture, le fonctionnement et l'utilisation du système de configuration centralisé de **ServerSupervisor**. Il permet de configurer l'ensemble des paramètres applicatifs depuis une interface d'administration dédiée dans l'UI web, tout en garantissant une compatibilité totale avec les variables d'environnement Docker.

---

## 1. Vue d'Ensemble de l'Architecture

Le système résout la configuration selon un modèle hiérarchique à trois sources :

```
┌─────────────────────────────────────────────────────────┐
│                    PRIORITÉ 1 (ENV)                     │
│         Variables d'environnement du conteneur          │
│          (docker-compose.yml / Docker run -e)           │
└────────────────────────────┬────────────────────────────┘
                             │ (Si non défini)
                             ▼
┌─────────────────────────────────────────────────────────┐
│                   PRIORITÉ 2 (UI / DB)                  │
│       Valeurs personnalisées enregistrées en base       │
│           (Page admin UI -> table settings)             │
└────────────────────────────┬────────────────────────────┘
                             │ (Si non défini)
                             ▼
┌─────────────────────────────────────────────────────────┐
│                   PRIORITÉ 3 (DÉFAUT)                   │
│          Valeurs de repli codées en dur                 │
│         (Registre applicatif internal/config)           │
└─────────────────────────────────────────────────────────┘
```

### Règles de Priorité et Comportement en Cas de Conflit

1. **Priorité 1 — Variable d'Environnement Docker (`ENV`)** :
   - Lorsque l'opérateur Docker définit explicitement une variable d'environnement (ex: `SMTP_PORT=587`), celle-ci est **autoritaire et prioritaire**.
   - Le système applique cette valeur effective pour tous les services applicatifs.
   - Dans l'interface utilisateur, le paramètre arbore le badge **`ENV Docker`**.

2. **Priorité 2 — Interface Utilisateur (`UI / DB`)** :
   - Si aucune variable d'environnement n'est injectée dans le conteneur, toute valeur configurée et sauvegardée par un administrateur dans la table `settings` prend effet immédiatement.
   - Dans l'interface, le paramètre affiche le badge **`Base de données (UI)`**.

3. **Priorité 3 — Valeur par Défaut (`Défaut`)** :
   - En l'absence de variable d'environnement et d'enregistrement en base, la valeur par défaut définie dans le registre applicatif est utilisée.
   - Dans l'interface, le paramètre affiche le badge neutre **`Défaut`**.

4. **Détection et Signalement des Conflits** :
   - Un **conflit** survient lorsqu'un administrateur a enregistré une valeur via l'UI en base, mais que l'opérateur a également défini la variable d'environnement dans Docker avec une valeur distincte.
   - Dans ce cas :
     - La variable d'environnement Docker **continue de primer** (garantie d'inviolabilité pour l'opérateur d'infrastructure).
     - La valeur saisie par l'admin reste conservée en base pour mémoire.
     - L'UI affiche un badge rouge **`Conflit ENV vs UI`** et une alerte explicite :  
       *« Conflit détecté : La valeur UI en base (X) est différente de la variable d'environnement active (Y). »*
     - Le compteur de conflits de la vue admin s'incrémente pour attirer l'attention de l'administrateur.

---

## 2. Tableau Récapitulatif des Paramètres Configurables

| Clé Technique | Variable d'Environnement | Catégorie | Type | Défaut | Secret ? | Redémarrage requis ? | Description |
| :--- | :--- | :--- | :--- | :--- | :---: | :---: | :--- |
| `SERVER_PORT` | `SERVER_PORT` | Serveur | int | `8080` | Non | Oui | Port TCP d'écoute HTTP du serveur |
| `BASE_URL` | `BASE_URL` | Serveur | string | `http://localhost:8080` | Non | Non | URL racine pour liens et redirections SSO |
| `TLS_ENABLED` | `TLS_ENABLED` | Serveur | bool | `false` | Non | Non | Active le mode HTTPS natif |
| `DEMO_MODE` | `DEMO_MODE` | Serveur | bool | `false` | Non | Oui | Coupe les appels réseau sortants (ENV-only) |
| `TZ` | `TZ` | Serveur | string | `UTC` | Non | Oui | Fuseau horaire pour crons et affichage |
| `LOG_LEVEL` | `LOG_LEVEL` | Logging | select | `info` | Non | Non | Niveau des logs (debug, info, warn, error) |
| `LOG_FORMAT` | `LOG_FORMAT` | Logging | select | `json` | Non | Oui | Format des logs (json, text) |
| `TRUSTED_PROXIES` | `TRUSTED_PROXIES` | Réseau | csv | `""` | Non | Non | CIDRs des reverse proxies autorisés |
| `ALLOWED_ORIGINS` | `ALLOWED_ORIGINS` | Réseau | csv | `""` | Non | Non | Origines autorisées pour WebSocket CORS |
| `DB_HOST` | `DB_HOST` | Base | string | `localhost` | Non | Oui | Hôte PostgreSQL / TimescaleDB |
| `DB_PORT` | `DB_PORT` | Base | int | `5432` | Non | Oui | Port TCP PostgreSQL |
| `DB_USER` | `DB_USER` | Base | string | `supervisor` | Non | Oui | Utilisateur PostgreSQL |
| `DB_PASSWORD` | `DB_PASSWORD` | Base | string | `supervisor` | Oui | Oui | Mot de passe PostgreSQL |
| `DB_NAME` | `DB_NAME` | Base | string | `serversupervisor` | Non | Oui | Nom de la base de données |
| `DB_SSLMODE` | `DB_SSLMODE` | Base | select | `disable` | Non | Oui | Mode SSL PostgreSQL (disable, require...) |
| `JWT_SECRET` | `JWT_SECRET` | Auth | string | `""` (auto-gen) | Oui | Non | Clé secrète de signature des tokens |
| `JWT_EXPIRATION` | `JWT_EXPIRATION` | Auth | duration | `24h` | Non | Non | Durée de validité d'une session JWT |
| `REFRESH_TOKEN_EXPIRATION` | `REFRESH_TOKEN_EXPIRATION` | Auth | duration | `168h` | Non | Non | Durée max du token de rafraîchissement |
| `ADMIN_USER` | `ADMIN_USER` | Auth | string | `admin` | Non | Oui | Utilisateur admin initial |
| `ADMIN_PASSWORD` | `ADMIN_PASSWORD` | Auth | string | `""` | Oui | Oui | Mot de passe admin initial (bootstrap) |
| `RATE_LIMIT_RPS` | `RATE_LIMIT_RPS` | Auth | int | `100` | Non | Oui | Requêtes par seconde max par IP |
| `RATE_LIMIT_BURST` | `RATE_LIMIT_BURST` | Auth | int | `200` | Non | Oui | Capacité de rafale de requêtes |
| `OIDC_ENABLED` | `OIDC_ENABLED` | OIDC | bool | `false` | Non | Non | Activation d'OpenID Connect / SSO |
| `OIDC_DISPLAY_NAME` | `OIDC_DISPLAY_NAME` | OIDC | string | `SSO / OpenID Connect` | Non | Non | Libellé sur le bouton de connexion |
| `OIDC_ISSUER_URL` | `OIDC_ISSUER_URL` | OIDC | string | `""` | Non | Non | URL du fournisseur OIDC |
| `OIDC_CLIENT_ID` | `OIDC_CLIENT_ID` | OIDC | string | `""` | Non | Non | Client ID OIDC |
| `OIDC_CLIENT_SECRET` | `OIDC_CLIENT_SECRET` | OIDC | string | `""` | Oui | Non | Client Secret OIDC |
| `OIDC_REDIRECT_URL` | `OIDC_REDIRECT_URL` | OIDC | string | `""` | Non | Non | URL de callback OIDC |
| `OIDC_SCOPES` | `OIDC_SCOPES` | OIDC | csv | `openid, profile...` | Non | Non | Portées demandées au fournisseur |
| `OIDC_DEFAULT_ROLE` | `OIDC_DEFAULT_ROLE` | OIDC | select | `viewer` | Non | Non | Rôle par défaut (viewer, operator, admin) |
| `OIDC_AUTO_CREATE_USER` | `OIDC_AUTO_CREATE_USER` | OIDC | bool | `true` | Non | Non | Création auto des comptes à la connexion |
| `OIDC_ALLOW_LOCAL_LOGIN`| `OIDC_ALLOW_LOCAL_LOGIN`| OIDC | bool | `true` | Non | Non | Maintien du formulaire de login local |
| `OIDC_INSECURE_SKIP_VERIFY` | `OIDC_INSECURE_SKIP_VERIFY` | OIDC | bool | `false` | Non | Non | Désactive la vérification TLS OIDC |
| `NOTIFY_URL` | `NOTIFY_URL` | Notifications | string | `""` | Non | Non | URL du topic ntfy.sh |
| `NTFY_AUTH_TOKEN` | `NTFY_AUTH_TOKEN` | Notifications | string | `""` | Oui | Non | Token d'accès topic ntfy privé |
| `SMTP_HOST` | `SMTP_HOST` | Notifications | string | `""` | Non | Non | Hôte du serveur SMTP sortant |
| `SMTP_PORT` | `SMTP_PORT` | Notifications | int | `587` | Non | Non | Port SMTP (587 STARTTLS, 465 SMTPS) |
| `SMTP_USER` | `SMTP_USER` | Notifications | string | `""` | Non | Non | Identifiant SMTP |
| `SMTP_PASS` | `SMTP_PASS` | Notifications | string | `""` | Oui | Non | Mot de passe SMTP |
| `SMTP_FROM` | `SMTP_FROM` | Notifications | string | `""` | Non | Non | Adresse expéditeur des emails |
| `SMTP_TO` | `SMTP_TO` | Notifications | string | `""` | Non | Non | Destinataire par défaut des alertes |
| `SMTP_TLS` | `SMTP_TLS` | Notifications | bool | `true` | Non | Non | Activation du chiffrement TLS |
| `GITHUB_TOKEN` | `GITHUB_TOKEN` | Intégrations | string | `""` | Oui | Non | Jeton d'accès GitHub pour les releases |
| `GITHUB_POLL_INTERVAL` | `GITHUB_POLL_INTERVAL` | Intégrations | duration | `15m` | Non | Non | Fréquence de vérification GitHub |
| `DOCKER_IMAGE_POLL_INTERVAL` | `DOCKER_IMAGE_POLL_INTERVAL` | Intégrations | duration | `6h` | Non | Non | Fréquence de vérification images Docker |
| `METRICS_RETENTION_DAYS` | `METRICS_RETENTION_DAYS` | Rétention | int | `30` | Non | Non | Rétention des métriques (jours) |
| `AUDIT_RETENTION_DAYS` | `AUDIT_RETENTION_DAYS` | Rétention | int | `90` | Non | Non | Rétention des logs d'audit (jours) |
| `WEB_LOGS_RETENTION_DAYS`| `WEB_LOGS_RETENTION_DAYS`| Rétention | int | `30` | Non | Non | Rétention des logs HTTP (jours) |
| `NETWORK_FLOWS_RETENTION_DAYS` | `NETWORK_FLOWS_RETENTION_DAYS` | Rétention | int | `14` | Non | Non | Rétention des flux réseau (jours) |
| `THREAT_WEIGHT_*` | `THREAT_WEIGHT_*` | Menaces | float | *variables* | Non | Non | Coefficients de score de menace web |
| `THREAT_THRESHOLD_*` | `THREAT_THRESHOLD_*` | Menaces | float | *variables* | Non | Non | Seuils d'alerte (Medium, High, Critical) |

---

## 3. Endpoints de l'API Backend (Go)

Tous les endpoints sont sécurisés et strictement réservés au rôle `admin` :

### `GET /api/v1/config` (ou `/api/config`)
- **Query Params** : `reveal=true` (optionnel, pour renvoyer les secrets en clair au lieu du caviardage `••••••••`).
- **Réponse** :
  ```json
  {
    "entries": [
      {
        "key": "SMTP_PORT",
        "setting_key": "smtp_port",
        "env_var": "SMTP_PORT",
        "label": "Port SMTP",
        "description": "Port de connexion SMTP",
        "category": "notifications",
        "type": "int",
        "default_value": "587",
        "effective_value": "587",
        "source": "env",
        "is_secret": false,
        "is_editable": true,
        "requires_restart": false,
        "has_env_override": true,
        "has_conflict": false
      }
    ],
    "total_params": 52,
    "env_count": 12,
    "ui_count": 4,
    "default_count": 36,
    "conflict_count": 0,
    "categories": ["server", "logging", "network", "database", "auth", "oidc", "notifications", "integrations", "retention", "threats"]
  }
  ```

### `PUT /api/v1/config/:key` (ou `/api/config/:key`)
- **Body** :
  ```json
  { "value": "465" }
  ```
- **Validation** : Vérification de type, options, ports (1-65535), durées Go (`15m`, `24h`), URLs valides.
- **Réponse** :
  ```json
  {
    "success": true,
    "entry": { ... },
    "warning": "La valeur a été enregistrée en base, mais la variable d'environnement Docker SMTP_PORT reste active et prioritaire.",
    "message": "Paramètre mis à jour"
  }
  ```

### `DELETE /api/v1/config/:key` (ou `/api/config/:key`)
- Supprime la surcharge UI stockée dans la table `settings`. Le paramètre retourne instantanément à sa valeur issue de l'ENV Docker (s'il existe) ou à sa valeur par défaut.

### `PUT /api/v1/config` (Mise à jour groupée)
- **Body** :
  ```json
  {
    "settings": {
      "SMTP_HOST": "mail.example.com",
      "SMTP_PORT": "587",
      "SMTP_TLS": "true"
    }
  }
  ```

---

## 4. Sécurité & Traçabilité (Audit Logs)

1. **Masquage des Secrets** :
   - Par défaut, les champs secrets (`IsSecret: true`) tels que `SMTP_PASS`, `DB_PASSWORD`, `JWT_SECRET`, `OIDC_CLIENT_SECRET`, `GITHUB_TOKEN` sont renvoyés sous forme de masque `••••••••`.
   - L'interface d'administration propose un commutateur sécurisé pour afficher/masquer les secrets à la demande.
2. **Protection du Journal d'Audit** :
   - Lors de la modification d'un secret, le journal d'audit enregistre automatiquement la mention caviardée `[REDACTED]`. Aucun mot de passe ou jeton en clair n'est écrit dans la table `audit_logs`.
3. **Contrôle d'Accès** :
   - Seuls les utilisateurs disposant du rôle `admin` peuvent accéder à l'API ou à la route UI `/admin/configuration`. Les rôles `operator` ou `viewer` reçoivent une réponse HTTP `403 Forbidden`.

---

## 5. Guide Développeur : Comment Ajouter un Nouveau Paramètre Configurable

L'architecture est entièrement déclarative et extensible. Pour ajouter un nouveau paramètre (ex: `SLACK_WEBHOOK_URL`) :

1. Ouvrez [`server/internal/config/registry.go`](file:///d:/GitHub/ServerSupervisor/server/internal/config/registry.go).
2. Ajoutez l'entrée dans le tableau `AllParams` :
   ```go
   {
       Key:             "SLACK_WEBHOOK_URL",
       SettingKey:      "slack_webhook_url",
       EnvVar:          "SLACK_WEBHOOK_URL",
       Type:            TypeString,
       DefaultValue:    "",
       Category:        "notifications",
       Label:           "URL du Webhook Slack",
       Description:     "URL entrante du webhook Slack pour les alertes.",
       IsSecret:        true,
       IsEditable:      true,
       RequiresRestart: false,
       Validate:        validateURL,
   },
   ```
3. Si le paramètre doit être mappé directement sur le struct `Config` de l'application :
   - Ajoutez le champ dans le struct `Config` (`server/internal/config/config.go`).
   - Dans `Load()`, lisez la variable : `SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", "")`.
   - Dans `OverrideFromDB()`, appliquez la surcharge DB :
     ```go
     if v, ok := settings["slack_webhook_url"]; ok && v != "" {
         c.SlackWebhookURL = v
     }
     ```
4. C'est terminé ! Le paramètre apparaît automatiquement dans :
   - L'API `GET /api/v1/config`.
   - L'interface web dans la catégorie sélectionnée avec champ adapté, validation et gestion des secrets.

---

## 6. Exemple `docker-compose.yml`

Voici un exemple montrant comment un opérateur peut forcer certains paramètres d'infrastructure via variables d'environnement, tout en laissant les paramètres fonctionnels (SMTP, notifications, rétention) être gérés par les administrateurs via l'UI :

```yaml
services:
  server:
    image: ghcr.io/rem7474/serversupervisor:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      # Paramètres d'infrastructure forcés par l'opérateur (Priorité 1 - ENV)
      SERVER_PORT: "8080"
      BASE_URL: "https://supervisor.example.com"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_NAME: "serversupervisor"
      DB_USER: "supervisor"
      DB_PASSWORD: "StrongDbPasswordHere!"
      TZ: "Europe/Paris"
      
      # Tous les autres paramètres (SMTP, OIDC, GitHub Token, seuils d'alertes)
      # sont laissés non définis ici afin d'être configurables directement
      # depuis l'interface d'administration UI / DB !
    depends_on:
      postgres:
        condition: service_healthy
```

---

## 7. Description Visuelle de l'Interface UI Admin (`/admin/configuration`)

```
+-----------------------------------------------------------------------------------------------+
| Dashboard / Configuration Docker & Système                                                    |
| [Icon] Configuration Docker & Système                       [Afficher secrets]  [Actualiser]  |
| Gestion centralisée des paramètres conteneurisés et variables d'environnement                 |
+-----------------------------------------------------------------------------------------------+
| [ 52 Paramètres ]   [ 12 via ENV ]   [ 4 via UI (DB) ]   [ 36 Défaut ]   [ 0 Conflit(s) ]     |
+-----------------------------------------------------------------------------------------------+
| (i) Règle de priorité : 1. ENV Docker (Prioritaire) > 2. UI / Base > 3. Valeur par défaut    |
+-----------------------------------------------------------------------------------------------+
| [ Rechercher paramètre ou variable... ]    [ Catégorie: Toutes v ]    [ Source: Toutes v ]    |
+-----------------------------------------------------------------------------------------------+
|                                                                                               |
| [Icon] Notifications & Alertes (7 éléments)                              [Enregistrer section]|
| +-------------------------+--------------------+-------------------------+------------------+ |
| | Paramètre               | Source             | Valeur                  | Actions          | |
| +-------------------------+--------------------+-------------------------+------------------+ |
| | Hôte SMTP               | [UI (vert)]        | [ mail.entreprise.fr  ] | [Enregistrer]    | |
| | SMTP_HOST               |                    |                         | [Réinitialiser]  | |
| +-------------------------+--------------------+-------------------------+------------------+ |
| | Port SMTP               | [ENV Docker (bleu)]| [ 587                 ] | [Enregistrer]    | |
| | SMTP_PORT               |                    | (i) Forcé via Docker    |                  | |
| +-------------------------+--------------------+-------------------------+------------------+ |
| | Mot de passe SMTP       | [Défaut (gris)]    | [ ••••••••        [Eye] ] | [Enregistrer]  | |
| | SMTP_PASS [Secret]      |                    |                         |                  | |
| +-------------------------+--------------------+-------------------------+------------------+ |
```
