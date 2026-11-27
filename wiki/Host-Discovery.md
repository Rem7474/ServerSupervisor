# Découverte réseau

Deux familles d'outils, pour inventorier un réseau privé et ajouter des hôtes sans connaître
chaque adresse à l'avance :

- le **ping sweep ICMP** (§1 à §3), exécuté par le serveur ;
- la **découverte par agent** (§4) : table ARP, scan de ports TCP borné et sonde SNMP,
  exécutés par un agent placé sur le réseau cible.

Aucun de ces outils n'enregistre d'hôte : ils listent des adresses candidates,
cross-référencées avec les hôtes déjà connus. Pas de fingerprinting d'OS, pas d'IPv6.

## 1. Lancer un scan

`POST /api/v1/hosts/discover` (admin) prend un CIDR IPv4 borné entre **`/24`** (254 adresses
utilisables — au-delà, une seule requête HTTP synchrone ping-sweeperait des milliers
d'adresses) et **`/30`** (2 adresses — en dessous, il n'y a rien de significatif à balayer). Le
scan s'exécute **de façon synchrone dans la requête HTTP** : la réponse n'arrive qu'une fois
tous les pings terminés, avec un maximum de 64 pings en parallèle (même logique de sémaphore que
le worker de sondes uptime — un `/24` entier ne doit pas forker 254 goroutines d'un coup).

Le résultat liste, par adresse : si elle a répondu, sa latence, et si elle correspond déjà à un
hôte enregistré (nom + ID affichés directement, pour ne pas ré-ajouter un doublon par erreur).

## 2. Ajout en masse

Les adresses répondantes et pas encore enregistrées peuvent être sélectionnées puis ajoutées en
un seul appel : `POST /api/v1/hosts/bulk` (admin), qui valide chaque entrée **indépendamment**
— l'échec d'une adresse (nom déjà pris, par exemple) ne bloque pas l'enregistrement des autres.
Chaque hôte créé reçoit sa propre clé API réelle, exactement comme un ajout manuel un par un.

## 3. Prérequis : `CAP_NET_RAW`

Le scan réutilise exactement le même code ICMP que les [sondes uptime de type `icmp`](Monitoring.md#le-check-icmp-a-besoin-de-cap_net_raw)
— même socket "ping" non privilégié en premier essai, même repli sur un socket raw nécessitant
`CAP_NET_RAW`, déjà accordé au binaire serveur non-root via `setcap` dans l'image officielle.
Un déploiement durci (`cap_drop: [ALL]`) doit ajouter `cap_add: [NET_RAW]`, sinon le scan
répond avec une erreur explicite plutôt que de rapporter silencieusement zéro réponse.

## 4. Découverte par agent (ARP, ports, SNMP)

Le serveur tourne souvent dans un autre réseau que les machines à inventorier (VPS, conteneur,
VLAN d'administration) : il ne voit ni leur table de voisinage, ni leurs adresses privées. Ces
trois sondes s'exécutent donc **sur un agent** choisi par l'administrateur, à travers le modèle
de commandes à distance habituel (module `discovery`) : file d'attente, prise en charge par
l'agent, progression en direct, résultat, journal d'audit (`discovery_arp`,
`discovery_port_scan`, `discovery_snmp`, `discovery_cancel`).

### Activation côté agent

| Sonde | Nature | Prérequis sur l'agent |
|---|---|---|
| Table ARP | Passive : lecture de `/proc/net/arp`, aucun paquet émis | Agent à jour (capacité `network_discovery`) |
| Scan de ports | Active : connexions TCP | `allow_network_scan: true` (capacité `network_scan`) |
| SNMP | Active : requêtes UDP GET | `allow_network_scan: true` (capacité `network_scan`) |

`allow_network_scan` (variable `SUPERVISOR_ALLOW_NETWORK_SCAN`) vaut `false` par défaut : le
serveur ne peut pas faire émettre de trafic actif à un agent sans l'accord local de son
administrateur. L'agent revérifie toutes les bornes ci-dessous de son côté, indépendamment du
serveur.

### Table ARP

Liste les voisins IPv4 résolus (entrées complètes uniquement) : IP, MAC, interface. Le serveur
ajoute une indication de fabricant pour une courte liste de préfixes MAC courants en homelab
(VMware, QEMU/KVM, Proxmox VE, VirtualBox, Hyper-V, Xen, Docker, Raspberry Pi) et signale les
MAC administrées localement (adresses aléatoires ou générées par logiciel). Ce n'est pas une
base OUI complète.

### Scan de ports TCP

- lancé explicitement par un administrateur, après confirmation dans l'interface ;
- cible : un bloc IPv4 privé (RFC 1918) entre `/24` et `/32`, jamais une adresse publique ;
  le paramètre `DISCOVERY_ALLOWED_SUBNETS` restreint en plus les plages autorisées ;
- ports : liste par défaut d'environ 85 ports d'infrastructure courants, ou liste explicite
  (`22,80,8000-8100`) plafonnée par `DISCOVERY_PORT_SCAN_MAX_PORTS` (1024 par défaut, 4096 au
  maximum) ;
- connexions TCP complètes (`connect`), sans socket raw ni privilège ;
- concurrence (64 par défaut, 256 au plus), débit (500 sondes/s par défaut, 2000 au plus),
  timeout par sonde (800 ms par défaut) et durée totale (20 minutes) bornés ;
- annulable : une tâche en attente est annulée immédiatement ; une tâche en cours s'arrête à son
  prochain rapport de progression (toutes les 2 secondes) et renvoie le résultat partiel.

### SNMP

GET SNMP v2c ou v3 (`noAuthNoPriv`, `authNoPriv`, `authPriv` ; SHA/SHA256, AES) sur `sysDescr`,
`sysObjectID`, `sysUpTime`, `sysContact`, `sysName`, `sysLocation`, plus un résumé optionnel
de `ifTable` (64 interfaces au plus). Même restriction de cible que le scan de ports.

La communauté et les phrases secrètes v3 ne sont **jamais** stockées en base, renvoyées par
l'API ni journalisées : le serveur les garde en mémoire (15 minutes au plus) sous une
référence à usage unique et ne les ajoute à la commande qu'au moment où l'agent la récupère.
Une tâche SNMP dont l'agent ne passe pas dans ce délai, ou survivant à un redémarrage du
serveur, échoue avec un message explicite et doit être relancée.

### API

| Méthode | Endpoint | Rôle |
|---|---|---|
| `GET` | `/api/v1/discovery/limits` | Bornes : ports par défaut, maximum de ports, préfixe minimal, sous-réseaux autorisés |
| `POST` | `/api/v1/discovery/arp` | `{host_id}` → `{job_id}` |
| `POST` | `/api/v1/discovery/port-scan` | `{host_id, cidr, ports}` → `{job_id}` |
| `POST` | `/api/v1/discovery/snmp` | `{host_id, target, version, community \| username…}` → `{job_id}` |
| `GET` | `/api/v1/discovery/jobs/:id` | État et résultat d'une tâche |
| `POST` | `/api/v1/discovery/jobs/:id/cancel` | Annuler une tâche |

Tous ces endpoints sont réservés aux administrateurs.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Scan refusé avec une erreur de validation sur le CIDR | Préfixe hors bornes — un `/23` ou plus large est refusé (trop d'adresses), un `/31` ou plus étroit aussi (rien à balayer) |
| Aucune adresse ne répond alors que le réseau est actif | `CAP_NET_RAW` manquant côté conteneur (§3), ou pare-feu/segmentation réseau qui bloque l'ICMP entre le conteneur serveur et le sous-réseau cible |
| Le scan met plusieurs secondes à répondre | Normal et volontaire — il est synchrone, la réponse HTTP attend la fin des 254 pings (bornés à 64 en parallèle) d'un `/24` complet |
| « L'agent de cet hôte ne prend pas en charge la découverte réseau » | Agent antérieur au module `discovery` : le mettre à jour |
| « Le scan actif est désactivé sur cet agent » | `allow_network_scan` absent ou à `false` dans la configuration de l'agent (§4) |
| Plage refusée « hors des sous-réseaux autorisés » | `DISCOVERY_ALLOWED_SUBNETS` est renseigné et ne contient pas la cible |
| Tâche SNMP en échec « community is missing » | Identifiants expirés (tâche restée plus de 15 minutes en attente) ou serveur redémarré entre le lancement et la prise en charge : relancer |
| Une adresse répondante n'apparaît pas comme "déjà enregistrée" alors qu'un hôte existe avec ce nom | Le rapprochement se fait par **adresse IP**, pas par nom — un hôte enregistré avec une IP différente (DHCP ayant changé) apparaîtra comme une nouvelle adresse |

## Pour aller plus loin

Voir aussi [Monitoring](Monitoring.md) pour le détail du mécanisme ICMP partagé, et la section
[Hôtes & Métriques](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#hôtes--métriques)
du README pour le tableau complet des routes API liées aux hôtes.
