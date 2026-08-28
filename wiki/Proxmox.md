# Superviser un cluster Proxmox VE

Ce guide détaille comment connecter ServerSupervisor à un ou plusieurs
clusters/nœuds [Proxmox VE](https://www.proxmox.com/en/proxmox-virtual-environment)
via son API REST — **sans agent installé sur l'hyperviseur**. La collecte
(nœuds, VMs, LXC, stockage, disques, tâches, sauvegardes) se fait entièrement
par polling HTTP authentifié par token API.

Une seule fonctionnalité échappe à ce modèle : la **console interactive LXC**
([§8](#8-console-interactive-lxc)), pour laquelle l'API Proxmox n'accepte pas
l'authentification par token et exige un vrai compte utilisateur PVE.

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

### Identifiants console PVE (optionnel, pour la console LXC)

Le token API ne suffit **pas** pour ouvrir une console interactive sur un
conteneur : l'endpoint d'upgrade WebSocket de Proxmox (`vncwebsocket`,
partagé par termproxy et VNC) refuse l'authentification `PVEAPIToken` et
n'accepte qu'un ticket utilisateur. Si vous voulez la console
([§8](#8-console-interactive-lxc)), il faut donc fournir en plus un couple
**utilisateur PVE + mot de passe** ayant le privilège `VM.Console` :

```bash
# Option 1 : réutiliser le compte de supervision créé plus haut
pveum role modify SSAuditor -privs "Datastore.Audit Sys.Audit VM.Audit VM.Console"
pveum passwd supervision@pve      # le compte doit avoir un mot de passe utilisable
```

Le privilège s'assigne ici à **l'utilisateur**, pas au token — c'est
l'utilisateur qui ouvre la session console. Un compte `root@pam` fonctionne
aussi, mais lui donne évidemment bien plus que `VM.Console`.

Ces identifiants sont facultatifs : sans eux, toute la collecte et toutes les
autres actions fonctionnent normalement, seul le bouton **Console** reste
indisponible.

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
| Utilisateur PVE | *Optionnel* — uniquement pour la console LXC (ex : `supervision@pve`, `root@pam`). Voir [§1](#identifiants-console-pve-optionnel-pour-la-console-lxc) |
| Mot de passe PVE | *Optionnel* — idem (masqué après création — laisser vide en édition = inchangé, même règle que le token secret) |

La colonne **Console** de la liste des connexions indique si ces identifiants
sont renseignés. **Tester la connexion** vérifie les deux couches séparément et
le dit explicitement : « Console non configurée », « identifiants console
valides », ou l'erreur de login PVE rencontrée. Le test ne peut pas confirmer
le privilège `VM.Console` lui-même — il n'est vérifiable qu'à l'ouverture
d'une console sur un conteneur précis.

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

Les quatre premières actions ci-dessous nécessitent `Sys.Modify` **sur le
token PVE** ; la console est le cas à part (pas de token du tout, un compte
utilisateur PVE avec `VM.Console` — voir [§8](#8-console-interactive-lxc)).
Ce qui diffère ensuite, c'est le contrôle **côté ServerSupervisor** :

| Action | Endpoint | Qui peut l'utiliser côté ServerSupervisor |
|---|---|---|
| Rafraîchir le cache apt d'un nœud | `POST /proxmox/nodes/:id/apt-refresh` | N'importe quel utilisateur authentifié — le token PVE est le seul garde-fou |
| Migrer un guest | `POST /proxmox/nodes/:id/guests/:vmid/migrate` | N'importe quel utilisateur authentifié — idem |
| Service systemd du nœud (start/stop/restart/reload) | `POST /proxmox/nodes/:id/services/:service/:action` | N'importe quel utilisateur authentifié — idem |
| Démarrer / arrêter (ACPI) / redémarrer un guest | `POST /proxmox/guests/:id/action` | **Admin uniquement**, en plus du token — cette action peut couper une VM en cours d'exécution directement, un blast radius jugé suffisant pour justifier un deuxième verrou applicatif |
| Ouvrir une console LXC | `GET /ws/proxmox/console/:guest_id` (WebSocket) | **Admin uniquement** — même posture que l'action guest ci-dessus, un shell interactif ayant au moins le même blast radius. Ne dépend pas de `Sys.Modify` mais du privilège `VM.Console` du compte PVE |

C'est un choix délibéré, pas un oubli : les trois premières actions
supposent que si vous avez donné `Sys.Modify` à ce token, vous acceptez que
tout compte authentifié de l'app puisse s'en servir. L'arrêt/redémarrage de
guest et l'ouverture de console sont les deux seuls cas où ServerSupervisor
ajoute son propre contrôle d'accès par-dessus celui de Proxmox. Notez aussi
que l'arrêt "dur" (power-off immédiat, sans ACPI) est volontairement absent
des actions proposées — seuls `start` / `shutdown` (ACPI) / `reboot` le sont.

L'ouverture d'une console est par ailleurs la **seule** action Proxmox qui
écrit une ligne dans `audit_logs` (action `proxmox_console_open`, avec
l'utilisateur, l'IP et le guest visé). Les autres actions en écriture
(`GuestAction` / migration / service de nœud / `apt-refresh`) ne sont pas
auditées à ce jour — ne le déduisez pas de la présence de cette ligne.

## 7. Sécurité

- `token_secret` est stocké en base et **jamais renvoyé au frontend**, y
  compris pour un admin qui rouvre le formulaire d'édition (le champ reste
  vide ; laisser vide = ne pas changer le secret existant)
- `pve_password` (identifiant console, [§1](#identifiants-console-pve-optionnel-pour-la-console-lxc))
  suit la même règle : jamais renvoyé au frontend, qui ne reçoit qu'un
  booléen « console configurée ». Il est en revanche stocké **en clair** en
  base, comme `token_secret` — ServerSupervisor n'a pas de coffre applicatif,
  la protection repose entièrement sur l'accès à PostgreSQL. Donnez à ce
  compte le strict `VM.Console` plutôt que de réutiliser `root@pam` si votre
  posture le demande
- `insecure_skip_verify` (ignorer les erreurs TLS) est **désactivé par
  défaut** — à n'activer que pour un certificat auto-signé connu, jamais en
  routine
- Aucune session console n'est persistée : le relais est un simple tuyau
  bidirectionnel, fermer le panneau termine le shell distant côté PVE

## 8. Console interactive LXC

Un shell interactif dans un conteneur LXC, directement depuis la fiche guest
(`/proxmox/guests/:id`, bouton **Console**), sans passer par l'interface web
de Proxmox ni par SSH.

### Portée

- **Conteneurs LXC uniquement.** Le bouton est grisé sur une VM QEMU
  (« VM QEMU : bientôt disponible ») : QEMU expose du VNC/RFB, un protocole
  entièrement différent de `termproxy`, non couvert par cette V1.
- **Admin uniquement**, et uniquement si les identifiants console PVE sont
  renseignés sur la connexion ([§1](#identifiants-console-pve-optionnel-pour-la-console-lxc)).
  Un triangle d'avertissement s'affiche à côté du bouton quand ils manquent.

### Fonctionnement

Le navigateur ouvre un WebSocket vers ServerSupervisor
(`GET /api/v1/ws/proxmox/console/:guest_id`), qui relaie vers `termproxy` sur
le nœud PVE. **Aucun ticket PVE n'est jamais exposé au navigateur** : le
serveur obtient lui-même un `PVEAuthCookie` via `POST /access/ticket` juste
avant d'ouvrir la socket, avec les identifiants de la connexion.

- Pas de reconnexion automatique : une session interrompue ne se rebranche
  jamais toute seule (un shell qui se rebranche silencieusement au milieu
  d'une commande serait pire que l'inverse). Un bouton **Rouvrir** ouvre une
  session neuve.
- Fermer le panneau (**✕**) termine réellement le shell distant, il ne se
  contente pas de masquer l'affichage. Le tampon d'écran reste visible à la
  réouverture, mais avec un nouveau shell dessous — même comportement que la
  console web de Proxmox.
- Le redimensionnement de la fenêtre est propagé au PTY distant.
- L'ouverture est tracée dans `audit_logs` (`proxmox_console_open`).

### Sur mobile

Le panneau passe en plein écran et une barre de touches apparaît sous le
terminal (`Ctrl`, `^C`, `Échap`, `Tab`, flèches) : un clavier virtuel ne
produit aucune de ces touches, sans quoi il serait impossible d'interrompre
une commande ou d'utiliser la complétion. `Ctrl` est un modificateur collant,
appliqué au caractère tapé juste après.

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
| Bouton **Console** grisé sur une VM | Normal : la V1 ne couvre que les conteneurs LXC (voir [§8](#8-console-interactive-lxc)) |
| Bouton **Console** actif mais triangle d'avertissement à côté | Identifiants console PVE absents sur cette connexion — Paramètres → Proxmox VE (voir [§1](#identifiants-console-pve-optionnel-pour-la-console-lxc)) |
| La console renvoie « admin only » | L'ouverture de console est admin-only côté ServerSupervisor, indépendamment des droits PVE (voir [§6](#6-actions-en-écriture--posture-de-permissions)) |
| La console se connecte puis se ferme immédiatement | Le compte PVE s'authentifie mais n'a pas `VM.Console` sur ce conteneur — **Tester la connexion** ne peut pas le détecter, il ne valide que le login |
| Sauvegardes : "Dernier résultat par VM" vide alors que des vzdump tournent | Aucune tâche vzdump n'a encore été vue par un cycle de poll depuis la création de la connexion — le résultat est dérivé des tâches PVE, pas d'une lecture directe du planning de sauvegarde |

## Pour aller plus loin

Voir aussi la section [Proxmox VE](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#proxmox-ve-supervision-sans-agent)
du README pour la vue d'ensemble des fonctionnalités et le tableau complet
des routes API.
