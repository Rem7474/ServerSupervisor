# Superviser un cluster Proxmox VE

Ce guide détaille comment connecter ServerSupervisor à un ou plusieurs
clusters/nœuds [Proxmox VE](https://www.proxmox.com/en/proxmox-virtual-environment)
via son API REST — **sans agent installé sur l'hyperviseur**. La collecte
(nœuds, VMs, LXC, stockage, disques, tâches, sauvegardes) se fait entièrement
par polling HTTP authentifié par token API.

---

## 1. Prérequis côté Proxmox : créer un token API

Toute la collecte repose sur un rôle en lecture seule et un token API dédié
(ne réutilisez pas `root@pam` ni un compte personnel) :

```bash
# Rôle lecture seule (nœuds, VMs, LXC, stockage, disques)
pveum role add SSAuditor -privs "Datastore.Audit Sys.Audit VM.Audit"
pveum user add supervision@pve
pveum aclmod / -user supervision@pve -role SSAuditor
pveum user token add supervision@pve monitoring --privsep 0
```

Copiez le `token ID` affiché (ex : `supervision@pve!monitoring`) et le
`secret` — le secret n'est montré qu'une seule fois par Proxmox.

> **Important** : si le token a "Privilege Separation" activé (coché par
> défaut à la création avec `--privsep` omis ou `1`), les permissions
> doivent être assignées **au token lui-même**, pas seulement à
> l'utilisateur :
> ```bash
> pveum aclmod / -token supervision@pve!monitoring -role SSAuditor
> ```
> C'est la cause la plus fréquente d'un token qui s'authentifie mais dont
> tous les appels renvoient une erreur de permission.

### Étendre les droits pour les actions en écriture (optionnel)

Le rôle ci-dessus ne permet que la lecture. Certaines actions déclenchables
depuis le dashboard ServerSupervisor ont besoin de `Sys.Modify` sur le même
token :

```bash
pveum role modify SSAuditor -privs "Datastore.Audit Sys.Audit Sys.Modify VM.Audit"
# + si privilege separation activé :
pveum aclmod / -token supervision@pve!monitoring -role SSAuditor
```

Voir [§6](#6-actions-en-écriture--posture-de-permissions) pour savoir
précisément quelles actions ce privilège débloque, et qui dans
ServerSupervisor peut les déclencher — les deux ne sont **pas** alignés de
la même façon pour toutes les actions.

## 2. Ajouter la connexion dans ServerSupervisor

Dans **Réglages** → carte **Proxmox VE** → **Ajouter une connexion** :

| Champ | Valeur |
|---|---|
| Nom | Label interne libre (ex : `Cluster prod`) |
| URL API | `https://pve.example.com:8006/api2/json` |
| Token ID | `supervision@pve!monitoring` |
| Token secret | Le secret copié à l'étape 1 (masqué après création — laisser vide en édition = inchangé) |
| Intervalle de collecte (s) | Défaut `60`, minimum `10` |
| Ignorer TLS (self-signed) | À cocher uniquement si le certificat de l'API Proxmox est auto-signé |
| Activé | Décoché = connexion conservée en base mais jamais pollée |

Cliquez **Tester la connexion** (utilise directement les champs du
formulaire, sans rien sauvegarder) avant de cliquer **Créer**. Chaque
connexion existante a aussi ses propres boutons **Tester** et **Collecter
maintenant** dans la liste, pour revalider ou forcer un cycle sans attendre
le prochain poll.

## 3. Ce qui est collecté, et à quelle fréquence

Le poller interne tourne toutes les 30s et respecte, par connexion,
l'`poll_interval_sec` configuré (donc une connexion à `120` n'est repollée
que toutes les deux minutes, même si le poller global tourne plus souvent).
À chaque cycle réussi :

- **Nœuds** : CPU, RAM, uptime, version PVE, compteur de mises à jour apt en attente
- **Guests** : VMs QEMU et conteneurs LXC (statut, ressources allouées)
- **Stockage** : pools et leur usage
- **Disques physiques** : modèle, type (SSD/HDD/NVMe), santé S.M.A.R.T., usure SSD (nécessite `Sys.Audit`)
- **Tâches** : 50 dernières tâches par nœud (vzdump, migration, création VM…)
- **Sauvegardes** : jobs vzdump configurés + dernier résultat par VM

Tout est en **UPSERT** — une ressource qui disparaît de Proxmox (VM
supprimée, nœud retiré du cluster) est automatiquement nettoyée en base au
cycle suivant, pas de résidu à purger manuellement.

Deux vues consomment ces données :
- `/proxmox` : cartes de synthèse (connexions, nœuds, VMs, LXC, stockage) + alertes de santé + tableau des nœuds
- `/proxmox/nodes/:id` : stats du nœud + onglets **VMs / LXC / Stockage / Disques / Tâches / Sauvegardes / Mises à jour / Services / Journaux sécurité**

## 4. Lier un guest à un hôte supervisé par agent

Si une VM/LXC Proxmox est *aussi* supervisée classiquement (agent
ServerSupervisor installé dedans), les deux identités peuvent être liées :

- Détection automatique par correspondance de nom → statut `suggested`
- Confirmation ou rejet manuel dans l'UI → statut `confirmed` / `ignored`
- Sélection de la **source de métriques** par guest : `auto` / `agent` / `proxmox`. Quand la source est `proxmox` et que la donnée est fraîche, le serveur indique à l'agent de ne plus envoyer CPU/RAM (`skip_metrics: true`) pour éviter la double collecte — les hôtes qui fournissent une donnée que Proxmox n'a pas (température CPU, RPM ventilateurs) continuent d'envoyer ces métriques quoi qu'il arrive

### Source capteurs nœud (température / ventilateurs)

L'API Proxmox n'expose pas de façon fiable la température CPU ni le RPM des
ventilateurs physiques d'un nœud. Si ce même serveur physique est par
ailleurs supervisé par un agent ServerSupervisor (installé directement sur
l'hôte Proxmox ou sur une VM qui a accès aux capteurs), le sélecteur
**Source capteurs nœud (CPU + ventilateurs)** en haut de la page détail du
nœud permet de le désigner comme source — les graphiques Température CPU /
RPM Ventilateurs du nœud utilisent alors les métriques déjà collectées par
cet agent.

## 5. Mises à jour apt du nœud

L'onglet **Mises à jour** d'un nœud est **en lecture seule par design** —
« pour appliquer les mises à jour, connectez-vous directement au nœud » — il
affiche uniquement le compteur de paquets en attente (issu du cache apt de
Proxmox, rafraîchi à chaque poll) et un bouton `apt update` qui déclenche un
rafraîchissement de ce cache côté PVE (`POST
/api/v1/proxmox/nodes/:id/apt-refresh`), pas une mise à jour réelle des
paquets. Ce bouton nécessite `Sys.Modify` sur le token (voir [§1](#étendre-les-droits-pour-les-actions-en-écriture-optionnel)) ; sans ce privilège, Proxmox
renvoie une erreur de permission que ServerSupervisor remonte telle quelle.

## 6. Actions en écriture : posture de permissions

Toutes les actions ci-dessous nécessitent `Sys.Modify` **sur le token PVE**.
Ce qui diffère, c'est le contrôle **côté ServerSupervisor** :

| Action | Endpoint | Qui peut l'utiliser côté ServerSupervisor |
|---|---|---|
| Rafraîchir le cache apt d'un nœud | `POST /proxmox/nodes/:id/apt-refresh` | N'importe quel utilisateur authentifié — le token PVE est le seul garde-fou |
| Migrer un guest | `POST /proxmox/nodes/:id/guests/:vmid/migrate` | N'importe quel utilisateur authentifié — idem |
| Service systemd du nœud (start/stop/restart/reload) | `POST /proxmox/nodes/:id/services/:service/:action` | N'importe quel utilisateur authentifié — idem |
| Démarrer / arrêter (ACPI) / redémarrer un guest | `POST /proxmox/guests/:id/action` | **Admin uniquement**, en plus du token — cette action peut couper une VM en cours d'exécution directement, un blast radius jugé suffisant pour justifier un deuxième verrou applicatif |

C'est un choix délibéré, pas un oubli : les trois premières actions
supposent que si vous avez donné `Sys.Modify` à ce token, vous acceptez que
tout compte authentifié de l'app puisse s'en servir. L'arrêt/redémarrage de
guest est le seul cas où ServerSupervisor ajoute son propre contrôle
d'accès par-dessus celui de Proxmox. Notez aussi que l'arrêt "dur"
(power-off immédiat, sans ACPI) est volontairement absent des actions
proposées — seuls `start` / `shutdown` (ACPI) / `reboot` le sont.

## 7. Sécurité

- `token_secret` est stocké en base et **jamais renvoyé au frontend**, y
  compris pour un admin qui rouvre le formulaire d'édition (le champ reste
  vide ; laisser vide = ne pas changer le secret existant)
- `insecure_skip_verify` (ignorer les erreurs TLS) est **désactivé par
  défaut** — à n'activer que pour un certificat auto-signé connu, jamais en
  routine

## Dépannage

| Symptôme | Cause probable |
|---|---|
| **Tester la connexion** échoue avec une erreur réseau | URL API incorrecte (doit inclure `/api2/json`), pare-feu entre ServerSupervisor et Proxmox, ou certificat TLS invalide (voir `insecure_skip_verify`) |
| Connexion créée mais badge **Erreur** dans la liste | Survolez le badge pour voir `last_error` — le plus souvent un token qui s'authentifie mais dont le rôle n'est pas assigné **au token** (privilege separation, voir [§1](#1-prérequis-côté-proxmox--créer-un-token-api)) |
| Nœuds/VMs/LXC visibles sur Proxmox mais absents du dashboard | Attendre le prochain cycle de poll (`poll_interval_sec`), ou cliquer **Collecter maintenant** sur la connexion pour forcer un cycle immédiat |
| `apt update`, migration ou action sur un service de nœud échoue avec une erreur de permission | Le token n'a pas `Sys.Modify` (voir [§1](#étendre-les-droits-pour-les-actions-en-écriture-optionnel)) — contrairement aux erreurs de lecture, celle-ci n'apparaît qu'au moment du clic, pas sur le badge de connexion |
| Démarrer/arrêter/redémarrer une VM ou un LXC renvoie 403 alors que le token a `Sys.Modify` | Cette action est admin-only côté ServerSupervisor en plus du token (voir [§6](#6-actions-en-écriture--posture-de-permissions)) — vérifiez le rôle du compte connecté, pas seulement le token PVE |
| Pas de courbe Température CPU / RPM Ventilateurs sur un nœud | Aucune source capteurs configurée (voir [§4](#source-capteurs-nœud-température--ventilateurs)) — l'API Proxmox seule n'expose pas ces valeurs de façon fiable |
| Disques physiques ou usure SSD absents d'un nœud | Le rôle du token n'inclut pas `Sys.Audit`, ou le nœud n'expose pas encore de données S.M.A.R.T. au moment du poll |
| Sauvegardes : "Dernier résultat par VM" vide alors que des vzdump tournent | Aucune tâche vzdump n'a encore été vue par un cycle de poll depuis la création de la connexion — le résultat est dérivé des tâches PVE, pas d'une lecture directe du planning de sauvegarde |

## Pour aller plus loin

Voir aussi la section [Proxmox VE](../README.md#proxmox-ve-supervision-sans-agent)
du README pour la vue d'ensemble des fonctionnalités et le tableau complet
des routes API.
