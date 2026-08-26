# Exemple resticprofile : Immich

Quatre profils regroupés sous un groupe unique (`full-immich-backup`), pour
sauvegarder bibliothèque, uploads, profils et dump de base en un seul appel.
Bibliothèque et uploads basculent Immich en mode maintenance le temps du
backup ; le dump de base est supposé déjà généré en amont (ex : par un
`run-before` séparé ou un cron dédié) puisqu'il source directement le
dossier `backups`.

```yaml
version: "1"

global:
  status-file: /home/user/restic-backups/backup-status.json

groups:
  full-immich-backup:
    - immich-library
    - immich-upload
    - immich-profile
    - immich-db

immich-library:
  backup:
    run-before:
      - "docker exec immich_server immich-admin enable-maintenance-mode"
    source:
      - /UPLOAD_LOCATION/library
    run-after:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    run-after-fail:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    extended-status: true

immich-upload:
  backup:
    run-before:
      - "docker exec immich_server immich-admin enable-maintenance-mode"
    source:
      - /UPLOAD_LOCATION/upload
    run-after:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    run-after-fail:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    extended-status: true

immich-profile:
  backup:
    run-before:
      - "docker exec immich_server immich-admin enable-maintenance-mode"
    source:
      - /UPLOAD_LOCATION/profile
    run-after:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    run-after-fail:
      - "docker exec immich_server immich-admin disable-maintenance-mode"
    extended-status: true

immich-db:
  backup:
    source:
      - /UPLOAD_LOCATION/backups
    extended-status: true
```

Points à noter :
- `/UPLOAD_LOCATION` est le point de montage que vous avez défini dans le `.env` d'Immich — adaptez le chemin réel.
- Pour déclencher les quatre profils en une fois (backup manuel ou tâche planifiée), utilisez le nom du groupe `full-immich-backup` comme cible — voir la section 4 du [guide principal](Restic-Backups.md#4-resticprofileyaml--définir-vos-profils-de-sauvegarde) pour la syntaxe `groups:`.

Voir aussi le [guide principal Restic](Restic-Backups.md) pour l'installation, l'activation côté agent et le déclenchement depuis ServerSupervisor.
