# Endpoint Prometheus `/metrics`

Le serveur peut exposer ses propres indicateurs de santé au format texte Prometheus
(trafic HTTP, runtime Go, pool de connexions DB, agents connectés, commandes, notifications).
Ce sont des métriques **du serveur ServerSupervisor**, pas des hôtes supervisés.

## 1. Activation

L'endpoint est désactivé par défaut (`GET /metrics` répond `404`). Il n'est jamais public :
il exige un jeton Bearer dédié, et reste désactivé tant qu'aucun jeton n'est défini.

| Variable | Description | Défaut |
|---|---|---|
| `METRICS_ENABLED` | Active `GET /metrics` | `false` |
| `METRICS_TOKEN` | Jeton exigé dans `Authorization: Bearer <jeton>` | *(vide)* |

Les deux variables se définissent uniquement par l'environnement du conteneur (redémarrage
requis). Générer un jeton : `openssl rand -hex 32`.

| Cas | Réponse |
|---|---|
| `METRICS_ENABLED=false`, ou jeton vide | `404` |
| Jeton absent ou incorrect | `401` |
| Jeton correct | `200`, format texte Prometheus |

Le chemin `/metrics` ne doit pas être redirigé publiquement par le reverse proxy ; le scrape se
fait de préférence sur le réseau interne.

## 2. Configuration du scrape

```yaml
scrape_configs:
  - job_name: serversupervisor
    metrics_path: /metrics
    scheme: http
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/serversupervisor.token
    static_configs:
      - targets: ["serversupervisor:8080"]
```

## 3. Métriques exposées

| Métrique | Type | Labels | Description |
|---|---|---|---|
| `serversupervisor_http_requests_total` | counter | `method`, `route`, `status_class` | Requêtes HTTP traitées |
| `serversupervisor_http_request_duration_seconds` | histogram | `method`, `route` | Latence des requêtes |
| `serversupervisor_http_requests_in_flight` | gauge | | Requêtes en cours |
| `serversupervisor_agents_connected` | gauge | | Agents avec une connexion WebSocket de push active |
| `serversupervisor_websocket_clients` | gauge | | Clients navigateur sur le flux de notifications |
| `serversupervisor_db_connections_open` / `_in_use` / `_idle` / `_max_open` | gauge | | État du pool de connexions DB |
| `serversupervisor_db_connection_wait_total` / `_wait_seconds_total` | counter | | Attentes d'une connexion DB |
| `serversupervisor_commands_dispatched_total` | counter | `module`, `outcome` | Commandes créées pour les agents (`queued` ou `error`) |
| `serversupervisor_notifications_total` | counter | `channel`, `outcome` | Envois de notifications (`smtp`, `ntfy`, `notify` ; `success` ou `error`) |
| `go_*`, `process_*` | | | Collecteurs runtime Go et processus |

Le label `route` est le gabarit de la route (`/api/v1/hosts/:id`), jamais le chemin brut ; les
requêtes qui ne correspondent à aucune route sont regroupées sous `unmatched`. La cardinalité
reste donc bornée.
