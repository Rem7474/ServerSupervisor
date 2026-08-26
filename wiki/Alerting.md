# Moteur d'alertes

Ce guide détaille comment une règle d'alerte est évaluée, quand un incident
s'ouvre et se referme, et quels mécanismes existent pour que le bruit reste
gérable en production (hystérésis, cooldown, acquittement, escalade,
corrélation, fenêtres de maintenance).

Le README donne la vue d'ensemble et la liste des endpoints ; ce document
explique le comportement.

---

## 1. Anatomie d'une règle

Une règle relie **une métrique** à **un ou deux seuils**, et décrit ce qui se
passe quand ils sont franchis.

| Champ | Rôle |
|---|---|
| `source_type` | `agent` (métriques d'un hôte), `proxmox` (cluster/nœud/guest) ou `synthetic` (sondes uptime / certificats SSL) |
| `host_id` / `proxmox_scope` / `docker_scope` | La cible, selon la source. Une règle Docker est forcément rattachée à un hôte ; une règle Proxmox est au niveau cluster |
| `metric` + `operator` | Ce qu'on mesure (`cpu_usage`, `memory_usage`, `status_offline`, …) et la comparaison (`>`, `>=`, `<`, `<=`) |
| `threshold_warn` / `threshold_crit` | Les deux niveaux de sévérité. Les deux sont optionnels, mais au moins un doit être renseigné |
| `threshold_clear_warn` / `threshold_clear_crit` | Seuils de **sortie** (hystérésis) — voir [§2](#2-hystérésis--pourquoi-deux-jeux-de-seuils) |
| `duration_seconds` | Durée pendant laquelle la condition doit tenir avant de déclencher |
| `baseline_window_seconds` | Uniquement pour `bandwidth_vs_rolling_avg` : fenêtre de moyenne glissante (1h / 6h / 24h) |
| `actions` | Canaux de notification, cooldown, escalade, commande à déclencher — voir [§4](#4-notifications) |
| `enabled` | Une règle désactivée n'est pas évaluée du tout |

Les métriques réellement disponibles ne sont pas une liste figée dans la doc :
elles dépendent de ce que la source sait produire, et sont exposées par
`GET /api/v1/alert-rules/capabilities/{agent,proxmox,synthetic,docker}` —
c'est ce que le formulaire de création utilise pour peupler ses menus.

**Cadence.** Le job `alert-eval` évalue toutes les règles **toutes les 60
secondes**, plus une passe immédiate au démarrage du serveur (pour que des
incidents restés ouverts après un redémarrage se referment sans attendre le
premier tick).

## 2. Hystérésis : pourquoi deux jeux de seuils

Une règle « CPU > 80 % » sur une machine qui oscille autour de 80 % ouvrirait
et refermerait un incident à chaque cycle. Les seuils de sortie évitent ça :
l'incident **s'ouvre** au franchissement de `threshold_warn`/`threshold_crit`,
mais ne **se referme** qu'au franchissement de `threshold_clear_*`, réglé plus
bas.

- `threshold_clear_crit` renseigné → un incident critique se résout quand la
  valeur repasse ce seuil.
- Non renseigné → il se résout dès que la sévérité redescend sous `crit`
  (c'est-à-dire dès le retour en `warn` ou en dessous).
- Même logique pour `threshold_clear_warn`, avec résolution quand plus aucune
  sévérité n'est active.

La sévérité est toujours calculée « le plus grave d'abord » : `crit` est testé
avant `warn`, et un incident déjà ouvert conserve sa sévérité tant que sa
propre condition de sortie n'est pas remplie.

`status_offline` est le seul cas particulier : pas de seuil du tout, la
sévérité est `crit` si l'hôte est hors ligne, `none` sinon.

## 3. Cycle de vie d'un incident

```
        seuil franchi (pendant duration_seconds)
                        │
                        ▼
                  ┌───────────┐   ack (admin)    ┌──────────────────────┐
                  │  ouvert   │ ───────────────► │ ouvert + acquitté    │
                  └───────────┘                  └──────────────────────┘
                        │                                   │
      seuil de sortie franchi, ou résolution manuelle       │
                        │◄──────────────────────────────────┘
                        ▼
                    ┌────────┐
                    │ résolu │
                    └────────┘
```

- **Acquitter** (`POST /alerts/incidents/:id/ack`, admin) marque l'incident
  « en cours de traitement ». Il reste ouvert — l'acquittement ne le referme
  pas, il **stoppe l'escalade**.
- **Résoudre** (`POST /alerts/incidents/:id/resolve`, admin) le referme à la
  main, sans attendre que la métrique redescende.
- L'acquittement n'a plus de sens une fois l'incident résolu : les deux
  dimensions sont indépendantes, un incident peut être
  ouvert/non-acquitté, ouvert/acquitté, ou résolu.

L'onglet **Vue active** de `/alerts` (onglet par défaut) est la vue « war
room » : uniquement les incidents ouverts, groupés par sévérité et triés du
plus ancien au plus récent.

## 4. Notifications

`actions.channels` liste les canaux à activer pour cette règle :

| Canal | Effet | Configuration requise |
|---|---|---|
| `smtp` | Email | `SMTP_HOST`/`SMTP_FROM` globaux + `actions.smtp_to` sur la règle |
| `ntfy` | Push ntfy | `NOTIFY_URL` global fournit le serveur (schéma + hôte) ; `actions.ntfy_topic` surcharge le topic par règle |
| `browser` | Notification in-app (WebSocket) **et** Web Push si le navigateur est abonné | Aucune — les clés VAPID sont générées et persistées automatiquement au premier envoi (voir [Notifications](Notifications.md)) |
| `notify` | **Déprécié.** POST JSON brut vers `NOTIFY_URL` | `NOTIFY_URL` |

Un canal dont la destination n'est pas configurée est **journalisé et ignoré**,
il ne fait pas échouer les autres canaux de la même notification. Le Web Push ne délivre
qu'aux comptes **admin** abonnés, quel que soit le rôle ciblé par la règle — voir
[Notifications](Notifications.md#3-portée--qui-reçoit-quoi).

> Le canal `notify` est la seule forme de « webhook générique » existante, et
> il est marqué déprécié dans le code. Un vrai canal
> Slack/Teams/Discord/webhook reste un item ouvert de la
> [roadmap](https://github.com/Rem7474/ServerSupervisor/blob/main/ROADMAP.md) (#6) — ne comptez pas dessus pour une intégration
> neuve.

### Cooldown et escalade

Deux réglages distincts, souvent confondus :

- **`cooldown`** (secondes) — délai minimum entre deux notifications pour la
  même règle. Empêche le spam quand une condition bat de l'aile. `0` = pas de
  cooldown.
- **`escalate_after_minutes`** — si > 0, **relance** la notification d'un
  incident toujours ouvert et **non acquitté**, toutes les N minutes. `0`
  (défaut) désactive l'escalade.

Contrairement au cooldown, l'escalade ne supprime jamais la *première*
notification : elle ne fait que répéter une notification non acquittée. Et
elle ne redéclenche **jamais** `command_trigger` — répéter une commande de
remédiation en boucle est une action bien plus risquée que répéter un message.

### Commande déclenchée (`command_trigger`)

Une règle peut dispatcher une commande agent quand elle se déclenche. Le
vocabulaire (module / action / cible) est le même que celui des runbooks et
est validé côté serveur contre une whitelist — voir
[Runbooks-and-Scheduled-Tasks.md](Runbooks-and-Scheduled-Tasks.md).

## 5. Corrélation : éviter la cascade « hôte down »

Quand un hôte tombe, chaque conteneur Docker et chaque guest Proxmox qui en
dépend franchit ses propres seuils. Sans garde-fou, c'est une notification par
règle affectée.

Un incident qui s'ouvre alors qu'un incident `status_offline` /
`heartbeat_timeout` est déjà ouvert sur le même hôte est donc **rattaché** à
celui-ci (`alert_incidents.correlated_with`) au lieu d'émettre sa propre
notification.

Ce qu'il faut en retenir :

- L'incident corrélé est **quand même enregistré** et visible dans l'UI (avec
  une icône de lien) — rien n'est masqué, seule la notification bruyante et le
  `command_trigger` sont supprimés.
- Un incident corrélé n'escalade jamais non plus.
- La corrélation est calculée **une fois, à la création**. Si l'incident
  hôte-down se résout avant, le rattachement n'est pas recalculé.
- Portée volontairement étroite : seules les cibles ramenables à un hôte réel
  (`docker:container:`, `docker:compose:`, guest Proxmox lié et confirmé) sont
  corrélées. Une sonde synthétique ou un scope Proxmox non-guest n'a pas
  d'hôte propriétaire unique et n'est jamais corrélé.

## 6. Fenêtres de maintenance

Une fenêtre de maintenance suspend les notifications sur une plage horaire —
onglet **Maintenance** de `/alerts`.

| Portée | Qui peut la créer |
|---|---|
| Un hôte (`host_id` renseigné) | Operator+ **sur cet hôte** |
| Tous les hôtes (`host_id` NULL) | **Admin uniquement** — elle rend tout le système silencieux |

Pendant la fenêtre, une cible couverte est **entièrement sautée** : aucun
nouvel incident n'est créé, et un incident déjà ouvert est résolu
silencieusement. La liste des incidents se rafraîchit toujours, mais aucun
canal bruyant (smtp/ntfy/push/navigateur) ni `command_trigger` ne part — une
intervention planifiée produit zéro bruit, dans les deux sens.

> **Limite connue** : une cible qui n'est pas un hôte agent (guest Proxmox,
> conteneur Docker, sonde synthétique) n'existe pas dans la table `hosts` et
> ne peut donc matcher qu'une fenêtre **globale**, jamais une fenêtre par
> hôte. C'est le même compromis que pour les permissions par hôte, pas un
> oubli.

## 7. Modèles de règles

Un modèle (`alert_rule_templates`, onglet **Modèles**) est une recette sans
hôte : nom, métrique, opérateur, seuils, actions. `POST
/alert-rule-templates/:id/apply` avec une liste d'`host_ids` crée **une règle
indépendante par hôte**, par le même chemin de validation qu'une création
manuelle.

Ce n'est **pas** un lien vivant :

- Modifier ou supprimer un modèle ne touche jamais les règles déjà créées.
- L'application n'est pas atomique : l'échec sur un hôte n'empêche pas les
  autres (les erreurs sont renvoyées par hôte).
- Volontairement limité aux métriques agent : une règle Docker exige un
  `host_id` par règle, une règle Proxmox est déjà au niveau cluster, et les
  deux métriques synthétiques s'évaluent globalement une fois par règle — aucun
  des trois ne correspond à « appliquer la même recette à N hôtes ».

Le filtrage du sélecteur d'hôtes accepte le nom **ou** un tag d'hôte.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Une règle ne déclenche jamais | `enabled` à faux, ou `duration_seconds` plus long que l'intervalle pendant lequel la condition tient réellement (l'évaluation a lieu toutes les 60s) |
| L'incident s'ouvre et se referme en boucle | Pas de seuil de sortie — renseignez `threshold_clear_warn`/`threshold_clear_crit` nettement en dessous du seuil de déclenchement (voir [§2](#2-hystérésis--pourquoi-deux-jeux-de-seuils)) |
| Incident visible dans l'UI mais aucune notification reçue | Canal non configuré (destination SMTP/ntfy manquante — c'est journalisé côté serveur), cooldown encore actif, incident corrélé à un hôte down, ou fenêtre de maintenance en cours |
| L'escalade ne se déclenche pas | L'incident est acquitté (c'est le but), ou corrélé, ou `escalate_after_minutes` est à 0 |
| Un hôte tombe et je ne reçois qu'une alerte au lieu de dix | Comportement voulu — les incidents fils sont corrélés (voir [§5](#5-corrélation--éviter-la-cascade--hôte-down-)). Ils sont bien enregistrés, marqués d'une icône de lien |
| Modifier un modèle n'a rien changé sur les règles existantes | Voulu — un modèle est un moule, pas un lien vivant (voir [§7](#7-modèles-de-règles)) |
| Une règle sur un guest Proxmox n'est pas couverte par ma fenêtre de maintenance d'hôte | Seule une fenêtre **globale** couvre une cible non-agent (voir [§6](#6-fenêtres-de-maintenance)) |

## Pour aller plus loin

- [README — Alertes](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#alertes) : liste des endpoints
- [Runbooks-and-Scheduled-Tasks.md](Runbooks-and-Scheduled-Tasks.md) : le
  vocabulaire module/action/cible partagé par `command_trigger`
- [ROADMAP.md](https://github.com/Rem7474/ServerSupervisor/blob/main/ROADMAP.md) : canaux de notification supplémentaires (#6),
  groupes dynamiques d'hôtes et règles par groupe (#7)
