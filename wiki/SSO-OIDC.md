# SSO OpenID Connect (OIDC)

Connexion unique via un fournisseur d'identité (Keycloak, Authentik, Authelia, Zitadel, Google…)
compatible OpenID Connect. Le flux utilise le code d'autorisation avec **PKCE (S256)** ; le
serveur découvre le fournisseur via `OIDC_ISSUER_URL` (`/.well-known/openid-configuration`).

## 1. Côté fournisseur d'identité

Créer un client **confidentiel** et déclarer l'URL de redirection :

```
<BASE_URL>/api/auth/oidc/callback
```

Récupérer le *client ID*, le *client secret* et l'URL de l'émetteur (*issuer*). Pour le
mapping de rôles, exposer un claim `groups` (ou le nom choisi dans `OIDC_GROUPS_CLAIM`) dans
l'ID token, avec les scopes `openid profile email groups`.

## 2. Côté ServerSupervisor

Les paramètres se saisissent dans **Paramètres → Configuration avancée → OIDC**, ou par
variables d'environnement (une variable définie explicitement reste prioritaire sur la valeur
saisie dans l'UI). Référence complète : [docs/admin-configuration.md](https://github.com/Rem7474/ServerSupervisor/blob/main/docs/admin-configuration.md).

| Variable | Défaut | Rôle |
|---|---|---|
| `OIDC_ENABLED` | `false` | Active le SSO |
| `OIDC_DISPLAY_NAME` | `SSO / OpenID Connect` | Libellé du bouton de connexion |
| `OIDC_ISSUER_URL` | — | URL de l'émetteur |
| `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | — | Identifiants du client |
| `OIDC_REDIRECT_URL` | — | URL de callback (voir ci-dessus) |
| `OIDC_SCOPES` | `openid, profile, email, groups` | Scopes demandés |
| `OIDC_USERNAME_CLAIM` / `OIDC_EMAIL_CLAIM` / `OIDC_GROUPS_CLAIM` | `preferred_username` / `email` / `groups` | Noms des claims lus |
| `OIDC_ADMIN_GROUP` / `OIDC_OPERATOR_GROUP` / `OIDC_VIEWER_GROUP` | `serversupervisor-admins` / `-operators` / `-viewers` | Groupes mappés aux rôles |
| `OIDC_DEFAULT_ROLE` | `viewer` | Rôle si aucun groupe ne correspond ; `none` ou `deny` refuse la connexion |
| `OIDC_AUTO_CREATE_USER` | `true` | Crée le compte à la première connexion |
| `OIDC_ALLOW_LOCAL_LOGIN` | `true` | Conserve le formulaire login/mot de passe |
| `OIDC_INSECURE_SKIP_VERIFY` | `false` | Désactive la vérification TLS de l'émetteur (déconseillé ; lu au démarrage, redémarrage requis) |

## 3. Rôles

À chaque connexion, le rôle est recalculé : `admin` si l'utilisateur appartient au groupe
admin, sinon `operator`, sinon `viewer`, sinon `OIDC_DEFAULT_ROLE`. Le rôle d'un compte existant
est mis à jour pour suivre les groupes du fournisseur.

## 4. Rattachement aux comptes locaux

Dans l'ordre :

1. un compte déjà lié au même identifiant (`sub`) OIDC ;
2. un compte local ayant la même adresse email, **uniquement si le fournisseur la déclare
   vérifiée** (`email_verified`) ;
3. un compte local de même nom d'utilisateur, **uniquement si** l'email vérifié du fournisseur
   est égal à celui du compte (un nom d'utilisateur identique seul ne suffit jamais) ;
4. sinon, création d'un compte (suffixe aléatoire si le nom est pris) si
   `OIDC_AUTO_CREATE_USER=true`, refus dans le cas contraire.

Un compte déjà lié à un autre `sub` n'est jamais repris.

## 5. Dépannage

| Symptôme | Cause probable |
|---|---|
| Le bouton SSO n'apparaît pas | `OIDC_ENABLED` à `false` ou émetteur injoignable |
| Erreur `redirect_uri` chez le fournisseur | L'URL déclarée ne correspond pas exactement à `<BASE_URL>/api/auth/oidc/callback` |
| Connexion refusée pour un compte inconnu | `OIDC_AUTO_CREATE_USER=false`, ou `OIDC_DEFAULT_ROLE=none` sans groupe correspondant |
| Compte local non rattaché, nouveau compte `nom_xxxx` créé | Email absent ou non vérifié dans l'ID token |
| Tous les utilisateurs sont `viewer` | Le claim de groupes est absent de l'ID token (vérifier le scope `groups` et `OIDC_GROUPS_CLAIM`) |
