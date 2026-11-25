# API REST et clés API

Le serveur expose une API REST sous `/api/v1`. Un script, un outil tiers ou une intégration
(Home Assistant, n8n, cron…) l'appelle avec une **clé API personnelle**, sans passer par une
session de navigateur.

## 1. Créer une clé

*Mon compte → Clés API → Nouvelle clé* :

| Champ | Valeurs |
|---|---|
| Nom | Libellé libre (1 à 100 caractères), pour reconnaître l'usage de la clé |
| Portée | `Lecture seule` (requêtes `GET`) ou `Lecture et écriture` |
| Validité | 30, 90 ou 365 jours, ou sans expiration |

La clé complète (`ssk_<identifiant>_<secret>`) s'affiche **une seule fois**, à la création :
la copier aussitôt. Le serveur n'en conserve qu'une empreinte SHA-256 et ne peut pas la
réafficher. Une clé perdue se révoque et se remplace.

La liste affiche pour chaque clé son préfixe (`ssk_<identifiant>`), sa portée, sa dernière
utilisation, son expiration et son état (active, expirée, révoquée). Un utilisateur a au plus
25 clés actives. La création et la révocation sont tracées dans le journal d'audit
(catégorie *auth*).

## 2. Utiliser une clé

```bash
curl -H "Authorization: Bearer $SS_API_KEY" https://supervisor.example.org/api/v1/hosts
```

Le serveur distingue la clé d'un jeton de session par son préfixe `ssk_`. Sans en-tête
`Accept-Language`, les messages d'erreur sont en anglais ; `Accept-Language: fr` les renvoie
en français. Format d'erreur : `{"error", "code", "i18nKey"}`, où `i18nKey` est un code stable.

## 3. Droits d'une clé

Une clé agit **au nom de l'utilisateur qui l'a créée**, avec son rôle et ses permissions par
hôte **actuels** : elle ne dépasse jamais ses droits, et un changement de rôle s'applique
immédiatement aux clés existantes. La portée restreint encore ces droits :

| Portée | Accès |
|---|---|
| `read` | `GET` et `HEAD` uniquement ; toute autre méthode répond `403` |
| `write` | Toutes les méthodes que le rôle du propriétaire autorise |

Certaines routes sont **refusées à toute clé** (`403`), quels que soient sa portée et le rôle
de son propriétaire :

- compte et session (`/auth/…`), dont la gestion des clés API elles-mêmes ;
- utilisateurs, paramètres et configuration (`/users`, `/settings`, `/config`) ;
- tout ce qui stocke ou renvoie des identifiants : connexions Proxmox et NPM, identifiants de
  registre, webhooks Git, abonnements Web Push ;
- enregistrement d'hôtes (`POST /hosts`, `/hosts/bulk`), rotation de clé d'agent et
  permissions par hôte.

Ces opérations restent disponibles depuis l'interface, avec une session.

## 4. Limitation de débit

Chaque clé a son propre quota, aux mêmes valeurs que la limite par IP (`RATE_LIMIT_RPS`,
`RATE_LIMIT_BURST`). Au-delà, le serveur répond `429`. La limite par IP s'applique en plus.

## 5. Spécification OpenAPI

`GET /api/v1/openapi.json` sert, sans authentification, une spécification OpenAPI 3
versionnée du sous-ensemble public de l'API : hôtes, métriques, conteneurs Docker, APT,
incidents d'alerte (dont l'acquittement), règles d'alerte, sondes uptime, certificats SSL,
Proxmox (lecture), fenêtres de maintenance et tableau de bord. Elle s'importe dans Postman,
Insomnia, Swagger UI ou un générateur de client.

Les routes documentées sont toutes accessibles avec une clé ; un test du serveur vérifie
qu'elles existent dans le routeur.

## 6. Révocation et expiration

*Mon compte → Clés API → icône corbeille* révoque une clé immédiatement : les requêtes
suivantes répondent `401`. Une clé expirée répond aussi `401`, avec un message distinct.
Supprimer un utilisateur supprime ses clés.

Un administrateur liste toutes les clés du serveur avec `GET /api/v1/api-keys` et révoque
n'importe laquelle avec `DELETE /api/v1/auth/api-keys/{id}`, depuis une session.

## Dépannage

| Réponse | `i18nKey` | Cause |
|---|---|---|
| `401` | `API_KEY_INVALID` | Clé mal copiée, inconnue ou supprimée |
| `401` | `API_KEY_EXPIRED`, `API_KEY_REVOKED` | Créer une nouvelle clé |
| `403` | `API_KEY_READ_ONLY` | Requête d'écriture avec une clé `read` |
| `403` | `API_KEY_ROUTE_FORBIDDEN` | Route réservée aux sessions (voir §3) |
| `403` | autre | Le rôle du propriétaire n'autorise pas l'action |
| `429` | `API_KEY_RATE_LIMITED` | Quota de la clé dépassé |
