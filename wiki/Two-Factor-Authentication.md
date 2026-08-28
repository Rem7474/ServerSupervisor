# Authentification à deux facteurs (TOTP & clés de sécurité/passkeys)

Second facteur optionnel, par compte, géré depuis `/account/security`. Deux méthodes
indépendantes, cumulables : un compte peut avoir l'une, l'autre, les deux, ou aucune.
`Authenticate()` considère l'une ou l'autre comme suffisante — il n'exige pas les deux à la
fois même si les deux sont enregistrées.

## 1. TOTP (application d'authentification)

1. Depuis `/account/security`, lancer l'activation TOTP : le serveur génère un secret + un QR
   code (`data:image/png` encodé côté serveur, rien à installer) + **10 codes de secours** à usage
   unique (chaînes de 10 caractères).
2. Scanner le QR code avec une app TOTP standard (Google Authenticator, Aegis, 1Password…).
3. Saisir le code à 6 chiffres affiché pour confirmer — tant que cette étape n'est pas validée,
   le secret généré n'est pas activé (`VerifyMFA` est ce qui bascule réellement `mfa_enabled`
   en base).
4. **Conserver les 10 codes de secours hors ligne.** Chacun n'est utilisable qu'une fois et
   remplace un code TOTP en cas de perte de l'appareil — ils sont hashés (bcrypt) en base, donc
   impossibles à récupérer après coup si perdus.

Désactiver TOTP (`DisableMFA`) redemande le mot de passe du compte — un attaquant qui aurait
volé une session active ne peut pas désactiver le second facteur sans connaître aussi le mot de
passe.

## 2. Clés de sécurité / passkeys (WebAuthn)

Alternative ou complément au TOTP : une clé de sécurité physique (YubiKey…) ou un passkey
plateforme (Touch ID, Windows Hello, gestionnaire de mots de passe compatible WebAuthn).

- `GET /api/v1/auth/webauthn/credentials` liste les clés déjà enregistrées sur le compte.
- `POST /api/v1/auth/webauthn/register/begin` puis `.../finish` enregistrent une nouvelle clé
  (cérémonie WebAuthn standard, gérée par le navigateur).
- La connexion via clé de sécurité (`POST /api/auth/webauthn/login/begin`/`finish`) est une
  cérémonie **séparée** de celle du TOTP, pas un code saisi dans le même formulaire — elle
  revérifie le mot de passe elle-même avant d'émettre un challenge, puisqu'aucune session
  n'existe encore à ce stade (même posture anti-brute-force que la connexion standard).

### `WEBAUTHN_RP_ID` / `WEBAUTHN_RP_ORIGINS`

Par défaut, le Relying Party ID et les origines autorisées sont déduits de `BASE_URL` /
`ALLOWED_ORIGINS` — rien à renseigner dans le cas courant où le frontend est servi depuis
l'origine même de l'API. Ne définissez `WEBAUTHN_RP_ID`/`WEBAUTHN_RP_ORIGINS` explicitement que
si ce n'est *pas* le cas (frontend et API sur des domaines réellement différents) — voir
`.env.example`. Un déploiement multi-domaine mal configuré ici se traduit par un échec
silencieux de la cérémonie côté navigateur (le domaine du challenge ne correspond pas au
domaine visité), pas par une erreur explicite.

## Dépannage

| Symptôme | Cause probable |
|---|---|
| Code TOTP toujours refusé alors qu'il semble correct | Horloge de l'appareil ou du serveur désynchronisée — TOTP est sensible au décalage de temps (±quelques dizaines de secondes de tolérance seulement) |
| Perte de l'appareil TOTP et des codes de secours | Aucune procédure de contournement automatisée — un admin doit désactiver le MFA du compte concerné directement en base (`DisableMFA` exige le mot de passe, qui reste le prérequis même en cas de perte du second facteur) |
| Cérémonie WebAuthn échoue systématiquement en environnement multi-domaine | Vérifiez `WEBAUTHN_RP_ID`/`WEBAUTHN_RP_ORIGINS` (§2) — la déduction automatique depuis `BASE_URL` suppose frontend et API sur la même origine |
| Bouton clé de sécurité grisé ou absent | Navigateur sans support WebAuthn, ou contexte non sécurisé (HTTPS requis, comme pour le Web Push — voir [Notifications](Notifications.md#activer-côté-navigateur)) |

## Pour aller plus loin

Voir aussi la section [RBAC](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md#rbac)
du README pour les rôles/permissions, distincts du MFA (l'un authentifie *qui* vous êtes, l'autre
autorise *ce que* vous pouvez faire une fois connecté).
