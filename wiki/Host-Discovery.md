# Découverte réseau (scan de sous-réseau)

Ping-sweep ICMP d'un bloc CIDR IPv4, pour ajouter plusieurs hôtes sans connaître chaque
adresse à l'avance. Onglet **Scanner un sous-réseau** de la page **Ajouter un hôte**.
Volontairement limité à "quelles adresses répondent à un ping" — pas de scan ARP, pas de scan
de ports, pas de fingerprinting d'OS. Le scan lui-même n'enregistre aucun hôte : il ne fait que
lister des adresses candidates, cross-référencées avec les hôtes déjà connus.

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

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Scan refusé avec une erreur de validation sur le CIDR | Préfixe hors bornes — un `/23` ou plus large est refusé (trop d'adresses), un `/31` ou plus étroit aussi (rien à balayer) |
| Aucune adresse ne répond alors que le réseau est actif | `CAP_NET_RAW` manquant côté conteneur (§3), ou pare-feu/segmentation réseau qui bloque l'ICMP entre le conteneur serveur et le sous-réseau cible |
| Le scan met plusieurs secondes à répondre | Normal et volontaire — il est synchrone, la réponse HTTP attend la fin des 254 pings (bornés à 64 en parallèle) d'un `/24` complet |
| Une adresse répondante n'apparaît pas comme "déjà enregistrée" alors qu'un hôte existe avec ce nom | Le rapprochement se fait par **adresse IP**, pas par nom — un hôte enregistré avec une IP différente (DHCP ayant changé) apparaîtra comme une nouvelle adresse |

## Pour aller plus loin

Voir aussi [Monitoring](Monitoring.md) pour le détail du mécanisme ICMP partagé, et la section
[Hôtes & Métriques](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#hôtes--métriques)
du README pour le tableau complet des routes API liées aux hôtes.
