# Notifications (centre in-app & Web Push)

Deux mécanismes distincts, souvent confondus parce qu'ils se déclenchent ensemble depuis le
canal `browser` d'une règle d'alerte (voir [Alerting](Alerting.md#4-notifications)), mais aussi
depuis d'autres domaines (suivi de releases, webhooks Git) :

| Mécanisme | Où | Fonctionne app fermée ? |
|---|---|---|
| Centre in-app | `/notifications`, alimenté en temps réel par WebSocket | Non — seulement si un onglet est ouvert |
| Web Push (VAPID) | Notification native du navigateur/OS, via le Service Worker | Oui — c'est tout son intérêt |

## 1. Centre in-app

`GET /api/v1/notifications` retourne un flux **global, partagé par tous les comptes
authentifiés** — ce n'est pas une boîte de réception par utilisateur. Seul le marqueur "lu
jusqu'à" (`POST /api/v1/notifications/mark-read`) est personnel, stocké par nom d'utilisateur,
pour que le badge de non-lus reste cohérent d'un appareil à l'autre pour le même compte. Un
opérateur ou un viewer voit donc exactement les mêmes notifications qu'un admin sur cette page.

## 2. Web Push (VAPID)

### Activer côté navigateur

Le bouton d'activation (menu notifications) déclenche `Notification.requestPermission()` —
nécessite un contexte sécurisé (HTTPS, ou `localhost` en développement) et un navigateur qui
supporte `ServiceWorker`/`PushManager`. Sans l'un des deux, le bouton n'a silencieusement aucun
effet plutôt que d'afficher une erreur.

### Clés VAPID : rien à configurer

Contrairement à beaucoup d'intégrations Web Push, **il n'y a aucune variable d'environnement à
renseigner**. La paire de clés VAPID est générée automatiquement par le serveur au tout premier
appel (`GET /api/v1/push/vapid-public-key`) et persistée dans la table `settings`
(`vapid_private_key`/`vapid_public_key`) — elle est ensuite réutilisée pour tous les envois
suivants. Le frontend met en cache la clé publique utilisée pour l'abonnement courant
(`localStorage`) ; si jamais la clé change côté serveur (ex : `settings` restaurée depuis une
sauvegarde antérieure au premier abonnement), l'ancien abonnement est invalidé et l'utilisateur
doit se réabonner.

## 3. Portée : qui reçoit quoi

Le Web Push ne délivre **qu'aux abonnements d'un compte `admin`**, quel que soit le domaine
d'origine (alerte, suivi de releases, webhook Git) — un opérateur ou un viewer peut s'abonner
depuis son navigateur, l'abonnement est enregistré, mais aucune notification push ne lui sera
jamais envoyée. C'est un choix côté code (`push.Service.Send` interroge explicitement les
abonnements du rôle `admin`), pas une option configurable. Le centre in-app, lui (§1), reste
visible par tous les rôles. Un abonnement à un endpoint devenu invalide (utilisateur ayant
révoqué la permission navigateur) est purgé automatiquement à la prochaine tentative d'envoi
échouée (HTTP 410 Gone).

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Rien ne se passe en cliquant sur "Activer les notifications" | Contexte non sécurisé (HTTP simple sans `localhost`) ou navigateur sans support `PushManager` — vérifiez la console navigateur |
| Notification in-app reçue mais jamais de notification système (app fermée) | Compte non-admin — voir §3 ; ou abonnement expiré côté navigateur (réabonnez-vous) |
| Notifications système reçues en double après une restauration de sauvegarde DB | Les clés VAPID ont changé entre l'ancien et le nouvel état restauré — désabonnez-vous puis réabonnez-vous pour resynchroniser |
| Badge "non lu" incohérent entre deux appareils du même compte | Le marqueur de lecture est stocké côté serveur par compte (§1) — un délai de synchronisation WebSocket est normal, une incohérence persistante ne l'est pas |

## Pour aller plus loin

Voir aussi la section [Notifications & Push](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#notifications--push)
du README pour le tableau complet des routes API, et [Alerting](Alerting.md) pour la
configuration du canal `browser` au niveau d'une règle.
