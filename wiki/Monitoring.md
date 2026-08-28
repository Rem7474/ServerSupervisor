# Monitoring synthétique (uptime & certificats SSL)

Sondes actives indépendantes de l'agent : ServerSupervisor lui-même effectue le check, ce qui
couvre aussi bien un service exposé publiquement qu'un équipement qui ne peut pas recevoir
l'agent (switch, imprimante, caméra IP…). Vue d'ensemble sur `/monitoring`, détail par sonde/
certificat sur `/monitoring/host/:id` (entités liées à un proxy host NPM) ou
`/monitoring/probes/:id` / `/monitoring/ssl/:id` (entités créées manuellement).

## 1. Sondes uptime

Une sonde a un `type` : `http`, `tcp` ou `icmp`.

| Type | Ce qui est vérifié |
|---|---|
| `http` | Requête `GET` sur `target` (URL complète). `expected_status` (optionnel) valide le code HTTP reçu ; `expected_body_regex` (optionnel) valide le corps de la réponse (lu jusqu'à 256 Ko max) ; `verify_tls` contrôle la validation du certificat ; `follow_redirects` contrôle si les redirections sont suivies |
| `tcp` | Simple connexion TCP sur `target` (`host:port`) — succès si la connexion s'établit |
| `icmp` | Ping ICMP sur `target` (IPv4) — la seule option qui ne nécessite ni port TCP ni endpoint HTTP côté cible |

`interval_sec` (fréquence de check) a un minimum de 10 s côté serveur — toute valeur en dessous
retombe silencieusement à 60 s. `timeout_sec` défaut à 10 s si absent ou nul. Le worker
lui-même se réveille toutes les 10 s et ne relance que les sondes réellement dues, donc
l'intervalle réel observé peut avoir jusqu'à 10 s de gigue.

### Le check ICMP a besoin de `CAP_NET_RAW`

Un ping ICMP nécessite un socket raw. Le serveur essaie d'abord un socket "ping" non privilégié
(`udp4`, ne marche que si `net.ipv4.ping_group_range` est configuré côté hôte Docker — pas le
cas par défaut), puis retombe sur un socket raw (`ip4:icmp`), qui a besoin de la capacité Linux
`CAP_NET_RAW`. L'image officielle l'accorde déjà au binaire non-root via `setcap` dans
`server/Dockerfile` (`CAP_NET_RAW` fait partie de l'ensemble de capacités par défaut de Docker,
aucun `cap_add` requis en temps normal). Un déploiement durci avec `cap_drop: [ALL]` doit
ajouter explicitement `cap_add: [NET_RAW]` dans son `docker-compose.yml` — sans cette
capacité, un check ICMP échoue avec un message explicite ("CAP_NET_RAW manquant") plutôt que de
rapporter un faux "hors ligne". Le scan de sous-réseau (voir
[Host-Discovery](Host-Discovery.md)) réutilise exactement le même code et a donc besoin de la
même capacité.

## 2. Certificats SSL/TLS

Un certificat suivi (`host` + `port`, 443 par défaut) est vérifié par une poignée TLS complète
toutes les **6 heures** (le premier check a lieu ~30 s après le démarrage du serveur, pas
immédiatement). `server_name` (SNI) par défaut à `host` si non renseigné. Le check lit la
chaîne de certificats même si elle est expirée ou invalide (`InsecureSkipVerify` côté client
TLS ne veut pas dire "je ne vérifie rien" ici — c'est nécessaire pour pouvoir afficher "expiré
depuis N jours" au lieu d'un échec de connexion muet). Chaque nouveau numéro de série observé
est enregistré comme un événement (`GET /api/v1/ssl/certificates/:id/history`), ce qui permet
de voir les renouvellements passés.

## 3. Déclenchement immédiat

`POST /api/v1/uptime/probes/:id/check-now` et `POST /api/v1/ssl/certificates/:id/check-now`
forcent un check hors cycle, sans attendre le prochain tick — utile juste après la création
d'une sonde/d'un certificat, ou pour vérifier un correctif.

## 4. Intégration avec les alertes

Deux métriques globales (pas par hôte) exploitables dans une règle d'alerte :
`uptime_down_count` (nombre de sondes actuellement down) et `ssl_min_days_remaining` (le plus
petit nombre de jours avant expiration parmi tous les certificats suivis). Ce sont les deux
seules métriques exclues des [modèles de règles réutilisables](Alerting.md#7-modèles-de-règles)
puisqu'elles s'évaluent globalement, pas par hôte — voir [Alerting](Alerting.md).

## 5. Intégration NPM

Connecter une instance Nginx Proxy Manager crée automatiquement une sonde uptime HTTP et un
certificat SSL par proxy host (activables individuellement) — voir
[NPM](NPM.md#4-monitoring-automatique-par-proxy-host).
Une sonde/certificat créé ainsi est désactivé en cascade si le proxy host est supprimé côté NPM
ou si la connexion NPM elle-même est supprimée.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Sonde ICMP toujours en échec avec "CAP_NET_RAW manquant" | Le conteneur serveur n'a pas la capacité — vérifiez que `server/Dockerfile` n'a pas été modifié pour retirer le `setcap`, ou que votre `docker-compose.yml` durci n'a pas oublié `cap_add: [NET_RAW]` après un `cap_drop: [ALL]` |
| Intervalle de check plus lent que prévu | `interval_sec` réglé sous 10 s est silencieusement remonté à 60 s ; sinon, le worker se réveille toutes les 10 s, donc jusqu'à 10 s de gigue est normal |
| Certificat marqué "expiré" alors qu'il vient d'être renouvelé côté serveur cible | Le prochain check n'a lieu qu'après le cycle de 6 h — utilisez **Vérifier maintenant** pour forcer une relecture immédiate |
| Sonde HTTP en échec malgré un service qui répond bien dans un navigateur | Vérifiez `expected_status`/`expected_body_regex` — un critère trop strict (ex : code exact au lieu d'une plage) fait échouer un check dont la réponse a légèrement changé |

## Pour aller plus loin

Voir aussi la section [Monitoring](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#monitoring-sondes-uptime--certificats-ssl)
du README pour le tableau complet des routes API.
