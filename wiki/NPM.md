# Intégration NPM (Nginx Proxy Manager)

Ce guide détaille comment connecter ServerSupervisor à une ou plusieurs
instances [Nginx Proxy Manager](https://nginxproxymanager.com/) pour importer
ses proxy hosts et générer automatiquement leur supervision (sonde uptime +
certificat SSL suivi).

> **Avant de commencer** : contrairement à Proxmox ou Restic, NPM ne propose
> pas de jeton API à portée restreinte — l'authentification se fait avec un
> identifiant/mot de passe NPM classique (voir [§1](#1-authentification)).
> Créez si possible un utilisateur NPM dédié (**Users → Add User** dans NPM)
> plutôt que de réutiliser le compte admin principal, pour pouvoir révoquer
> l'accès de ServerSupervisor indépendamment.

---

## 1. Authentification

ServerSupervisor s'authentifie auprès de l'API NPM avec `POST
{api_url}/api/tokens` (identité + secret), et
**ne met jamais ce token en cache** : chaque synchronisation, bascule ou
test de connexion refait un login complet. Il n'y a donc pas de session
longue durée à gérer de votre côté — juste un identifiant/mot de passe NPM
valides en permanence.

## 2. Ajouter la connexion dans ServerSupervisor

Dans **Réglages** → onglet **Intégrations** → carte **NPM** →
**Ajouter une connexion** :

| Champ | Valeur |
|---|---|
| Nom | Label interne libre (ex : `NPM prod`) |
| URL API | `http://192.168.1.10:81` (port admin NPM par défaut) |
| Identifiant (email) | Le compte NPM créé à l'étape 1 |
| Mot de passe | Le mot de passe de ce compte (laisser vide en édition = inchangé) |
| Intervalle de rafraîchissement (s) | Défaut `3600` (1h), minimum `60` |
| Activé | Décoché = connexion conservée en base mais jamais synchronisée |

**Tester la connexion** authentifie et liste les proxy hosts sans rien
écrire en base — pratique pour valider les identifiants avant de créer la
connexion. Notez que ce test répond toujours en HTTP 200 : le succès ou
l'échec se lit dans le message affiché, pas dans le code de statut.

## 3. Synchronisation : ce qui est importé, et à quelle fréquence

> **Important — il n'y a pas d'import sélectif.** Une fois la connexion
> créée, **tous** les proxy hosts existants dans NPM apparaissent
> automatiquement sur `/npm` après le premier cycle de synchronisation —
> il n'y a pas de fenêtre de sélection pour n'en importer qu'une partie.
> Chaque host importé démarre avec sa supervision **activée par défaut**
> (uptime + SSL) ; désactivez-la individuellement sur les hosts que vous ne
> voulez pas surveiller depuis `/npm`.

Cadence : un cycle de vérification tourne toutes les 30 secondes en
interne, mais ne contacte réellement l'API NPM d'une connexion donnée que
si son `poll_interval_sec` (défaut 1h) est écoulé depuis le dernier succès.
Le bouton **Rafraîchir maintenant** (carte de connexion, Réglages) force un
cycle immédiat pour une connexion — c'est un déclenchement "fire-and-forget" :
la réponse confirme juste que la synchronisation a démarré, pas son résultat ;
rechargez la liste quelques secondes après pour voir `Dernier contact` mis à
jour (ou `last_error` en cas d'échec).

## 4. Monitoring automatique par proxy host

Sur `/npm`, chaque host importé expose deux interrupteurs — **Uptime** et
**SSL** — pas d'interrupteur maître unique dans cette vue : le "master"
(`monitoring_enabled`) est en réalité recalculé côté serveur à partir des
deux sous-interrupteurs (`= uptime OR ssl`), pas piloté directement depuis
l'UI.

- Activer **Uptime** crée une sonde HTTP(S) à la première activation
  (cible = domaine principal du host, intervalle 60s, timeout 10s, statut
  attendu 200, TLS vérifié).
- Activer **SSL** crée un certificat suivi (port 443) — **uniquement si le
  proxy host a SSL activé dans NPM** (`ssl_forced` ou un certificat déjà
  attaché). Un host en HTTP pur ne peut pas avoir de suivi SSL, le
  toggle est grisé avec une infobulle explicite dans ce cas.
- Un host sans surveillance uptime alors qu'il est actif dans NPM est mis
  en évidence (ligne surlignée, icône d'avertissement) — une panne sur ce
  host ne serait sinon jamais détectée.

## 5. Basculer un host actif/inactif directement dans NPM

Le switch **Actif NPM** ne pilote pas seulement l'affichage côté
ServerSupervisor : il appelle l'API NPM elle-même
(`enable`/`disable` du proxy host), donc **coupe réellement le routage** si
vous le désactivez. C'est le seul des trois interrupteurs de la ligne à
afficher une confirmation avant d'agir — les deux toggles de monitoring
s'appliquent instantanément, sans confirmation.

## 6. Nettoyage automatique (cascade de désactivation)

Trois situations désactivent automatiquement la supervision d'un host sans
intervention manuelle :

| Situation | Effet |
|---|---|
| Suppression de la connexion NPM | Sonde uptime et certificat SSL liés sont **désactivés**, pas supprimés — ils restent visibles ailleurs (Monitoring) mais n'émettent plus de vérifications |
| Un host est désactivé côté NPM lui-même (hors ServerSupervisor) | Détecté au prochain sync, monitoring désactivé en cascade |
| Un host est supprimé côté NPM (plus du tout dans la réponse API) | Idem — détecté par comparaison avec le cycle précédent, monitoring désactivé en cascade |

Dans les trois cas, **le proxy host, la sonde et le certificat ne sont
jamais supprimés automatiquement** — seule leur activation est coupée,
pour éviter des lignes fantômes qui continueraient à vérifier une cible qui
n'existe plus.

## 7. Sécurité

- Les identifiants NPM (`identity`/`secret`) sont stockés **en clair** en
  base — contrairement au `token_secret` Proxmox ou aux credentials Restic
  (qui ne quittent jamais l'hôte), il n'y a pas de couche de chiffrement
  dédiée côté ServerSupervisor aujourd'hui. Utilisez un compte NPM dédié
  à portée limitée (voir la note en tête de guide) plutôt que le compte
  admin principal.
- Toutes les routes NPM exigent une session authentifiée ; les routes
  d'écriture (créer/modifier/supprimer une connexion, tester, rafraîchir,
  modifier un toggle) sont **admin uniquement**. Un compte `viewer` peut
  consulter `/npm` mais tout clic sur un toggle échouera côté API (403) —
  l'UI actuelle ne masque pas ces contrôles pour un non-admin.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| **Tester la connexion** échoue | URL API incorrecte (vérifiez le port admin, souvent `:81`), identifiants invalides, ou instance NPM injoignable — le timeout HTTP est fixé à 15s |
| Aucun proxy host n'apparaît sur `/npm` après création de la connexion | Attendez le premier cycle (jusqu'à `poll_interval_sec`), ou cliquez **Rafraîchir maintenant** sur la connexion dans Réglages |
| Tous les proxy hosts NPM apparaissent alors que je n'en voulais qu'une partie | Comportement normal — il n'y a pas de sélection à l'import (voir [§3](#3-synchronisation--ce-qui-est-importé-et-à-quelle-fréquence)) ; désactivez la supervision host par host après import |
| Toggle SSL grisé sur un host que je sais servir en HTTPS | Vérifiez côté NPM que "Force SSL" est coché ou qu'un certificat est bien attaché — ServerSupervisor se fie à ces deux signaux, pas à un ping HTTPS réel |
| Un host désactivé côté ServerSupervisor réapparaît actif après un moment | Il a probablement été réactivé directement dans NPM — le prochain sync reflète l'état réel de NPM, qui est toujours la source de vérité |
| Clic sur un toggle sans effet, pas d'erreur visible | Compte connecté non-admin — vérifiez le rôle, l'UI ne masque pas encore ces contrôles pour un `viewer` |
| Connexion supprimée mais des sondes/certificats NPM traînent encore dans Monitoring | Normal et volontaire — ils sont désactivés, pas supprimés (voir [§6](#6-nettoyage-automatique-cascade-de-désactivation)) ; supprimez-les manuellement dans `/monitoring` si vous n'en avez plus besoin |

## Pour aller plus loin

Voir aussi la section [NPM (Nginx Proxy Manager)](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#npm-nginx-proxy-manager)
du README pour le tableau complet des routes API, et le host-exposure
(`GET /api/v1/hosts/:id/exposure`, documenté dans le README principal du
dépôt) qui corrèle les domaines NPM avec le trafic web observé sur un hôte.
