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

## 6. Objectifs de disponibilité (SLA/SLO)

Chaque sonde porte un objectif de disponibilité `slo_target` en pourcentage (défaut `99.9`,
borné à `]0 ; 100]`), réglable à la création et dans le formulaire d'édition. Le panneau
**SLA** de `/monitoring` (route `GET /api/v1/uptime/sla`) le compare à la disponibilité mesurée
sur trois fenêtres glissantes : 7, 30 et 90 jours.

- **Calcul** : disponibilité = checks réussis / checks effectués sur la fenêtre. Les fenêtres
  sont des jours calendaires UTC, jour courant inclus.
- **Budget d'erreur** : nombre de checks en échec autorisés = `(1 - objectif/100) × checks`.
  Le budget restant est la part de ce quota non consommée, bornée entre 0 et 100 %. Un objectif
  de 100 % n'autorise aucun échec : le premier check en échec épuise le budget.
- **Statut** : `breached` si la disponibilité est sous l'objectif, `at_risk` si moins de 25 %
  du budget reste, `met` sinon, `no_data` en l'absence de checks sur la fenêtre.
- **Équivalent en minutes** : la durée d'indisponibilité tolérée sur la fenêtre est affichée à
  titre indicatif (`allowed_downtime_minutes`). Le calcul reste fondé sur le nombre de checks,
  pas sur la durée.

### Rétention et couverture réelle

Les résultats bruts des sondes sont conservés 30 jours. Pour couvrir 90 jours, le worker
agrège chaque jour UTC dans la table `uptime_probe_daily` (checks effectués et réussis par
sonde), conservée 400 jours. L'agrégation s'exécute au démarrage sur les 29 derniers jours puis
toutes les heures sur les deux derniers jours complets. Elle est idempotente. Le calcul lit les
agrégats pour les jours antérieurs à la veille et les résultats bruts pour la veille et le jour
courant.

Une fenêtre n'est jamais extrapolée : chaque fenêtre indique `covered_days`, le nombre de jours
qui ont réellement des données, et la sonde indique `data_since`. Sur une instance mise à jour,
l'historique disponible commence à la plus ancienne donnée brute encore présente (30 jours au
plus) : la fenêtre de 90 jours affiche alors par exemple « 30 j sur 90 ».

## 7. Page de statut publique

Une page de statut en lecture seule, accessible sans authentification sur `/status`, présente
l'état des services choisis par un administrateur. Elle est **désactivée par défaut** : tant
qu'elle l'est, `/status` et `GET /api/public/status` répondent `404`.

L'administrateur la configure depuis `/monitoring` → **Page de statut** (routes admin
`GET`/`PUT /api/v1/status-page`) :

- activation, titre (120 caractères max) et description (500 caractères max) ;
- liste explicite des sondes publiées (50 au plus), chacune avec un **nom public** obligatoire
  (80 caractères max), jamais prérempli à partir du nom interne de la sonde.

Chaque modification est tracée dans le journal d'audit (`status_page_update`). La suppression
d'une sonde la retire de la page.

### Ce qui est exposé

La réponse publique est construite champ par champ à partir du nom public et des compteurs de
checks. Elle ne contient jamais l'identifiant, la cible (URL, hôte, IP, port), le type de sonde
ni le message d'erreur d'un check. Pour chaque service publié :

- le statut courant : `operational` (dernier check réussi), `major_outage` (dernier check en
  échec) ou `unknown` (sonde désactivée ou jamais vérifiée) ;
- la disponibilité sur 90 jours et une barre par jour ;
- les incidents récents : périodes d'échec consécutives des 14 derniers jours (20 au plus),
  décrites uniquement par le nom public et les heures de début et de fin.

Le statut global vaut `major_outage` si tous les services connus sont en échec,
`partial_outage` si une partie seulement l'est, `operational` si tous les services connus
répondent, `unknown` si aucun état n'est connu.

### Cache et limitation de débit

La réponse est mise en cache 30 s en mémoire (l'état « désactivée » aussi) et servie avec
`Cache-Control: public, max-age=30`. Une modification de la configuration vide le cache.
`GET /api/public/status` et `/status` sont limités par adresse IP (2 requêtes/s, rafale de 10).
La page se rafraîchit toutes les 60 s et conserve le dernier état affiché en cas d'erreur
réseau.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Sonde ICMP toujours en échec avec "CAP_NET_RAW manquant" | Le conteneur serveur n'a pas la capacité — vérifiez que `server/Dockerfile` n'a pas été modifié pour retirer le `setcap`, ou que votre `docker-compose.yml` durci n'a pas oublié `cap_add: [NET_RAW]` après un `cap_drop: [ALL]` |
| Intervalle de check plus lent que prévu | `interval_sec` réglé sous 10 s est silencieusement remonté à 60 s ; sinon, le worker se réveille toutes les 10 s, donc jusqu'à 10 s de gigue est normal |
| Certificat marqué "expiré" alors qu'il vient d'être renouvelé côté serveur cible | Le prochain check n'a lieu qu'après le cycle de 6 h — utilisez **Vérifier maintenant** pour forcer une relecture immédiate |
| Fenêtre SLA de 90 jours affichée « 30 j sur 90 » | L'historique antérieur à l'agrégation journalière a été purgé par la rétention de 30 jours des résultats bruts ; la couverture augmente d'un jour par jour |
| `/status` répond 404 | La page de statut est désactivée (état par défaut) — activez-la dans `/monitoring` → **Page de statut**. Après activation, le cache public peut servir l'ancien état jusqu'à 30 s |
| Sonde HTTP en échec malgré un service qui répond bien dans un navigateur | Vérifiez `expected_status`/`expected_body_regex` — un critère trop strict (ex : code exact au lieu d'une plage) fait échouer un check dont la réponse a légèrement changé |

## Pour aller plus loin

Voir aussi la section [Monitoring](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#monitoring-sondes-uptime--certificats-ssl)
du README pour le tableau complet des routes API.
