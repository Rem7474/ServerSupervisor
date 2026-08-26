# Exemple resticprofile : Nextcloud AIO

Deux profils : les données Nextcloud (volumes Docker) et un dump PostgreSQL
séparé, avec bascule en mode maintenance autour du backup des données pour
garantir un état cohérent.

```yaml
version: "1"

global:
  status-file: /home/user/restic-backups/backup-status.json

nextcloud-data:
  backup:
    run-before:
      - "docker exec -u www-data nextcloud-aio-nextcloud php occ maintenance:mode --on"
    source:
      - /var/lib/docker/volumes/nextcloud_aio_nextcloud_data/_data
      - /var/lib/docker/volumes/nextcloud_aio_mastercontainer/_data
    exclude:
      - "**/.tmp"
      - "**/cache/*"
    run-after:
      - "docker exec -u www-data nextcloud-aio-nextcloud php occ maintenance:mode --off"
    run-after-fail:
      - "docker exec -u www-data nextcloud-aio-nextcloud php occ maintenance:mode --off"
    extended-status: true

nextcloud-db:
  backup:
    run-before: "docker exec nextcloud-aio-database pg_dumpall -U nextcloud > /home/user/restic-backups/nextcloud-db.sql"
    source:
      - /home/user/restic-backups/nextcloud-db.sql
    extended-status: true
```

Points à noter :
- `run-after-fail` désactive le mode maintenance même si le backup échoue — sans ça, un backup en erreur laisse l'instance bloquée en maintenance.
- Les deux profils sont indépendants : déclenchez-les séparément (backup manuel ou tâche planifiée par profil), ou groupez-les sous une entrée `groups:` si vous voulez les deux dans un seul appel — voir la section 4 du [guide principal](Restic-Backups.md#4-resticprofileyaml--définir-vos-profils-de-sauvegarde).

Voir aussi le [guide principal Restic](Restic-Backups.md) pour l'installation, l'activation côté agent et le déclenchement depuis ServerSupervisor.
