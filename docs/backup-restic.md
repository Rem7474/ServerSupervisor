# Configurer les sauvegardes Restic — guide de A à Z

Ce guide détaille comment mettre en place la supervision et le déclenchement de
sauvegardes [Restic](https://restic.net) via [resticprofile](https://creativeprojects.github.io/resticprofile/)
sur un hôte supervisé par ServerSupervisor, du premier `restic init` jusqu'à la
planification récurrente dans l'UI.

**Principe important** : ServerSupervisor n'installe rien et ne stocke aucun
secret. Le binaire `restic`, le dépôt, `resticprofile` et tous les credentials
(mot de passe du dépôt, clés S3/B2/Swift, identifiants SMTP…) restent
entièrement sur la machine supervisée, gérés par vous. L'agent se contente de
lire un fichier de statut et d'exécuter un script que vous fournissez.

---

## 1. Prérequis sur l'hôte supervisé

1. Installer `restic` ([instructions officielles](https://restic.readthedocs.io/en/stable/020_installation.html)).
2. Installer `resticprofile` ([instructions officielles](https://creativeprojects.github.io/resticprofile/installation/)).
3. Initialiser (ou avoir déjà) un dépôt Restic :
   ```bash
   restic -r /srv/restic-repo init
   # ou un backend distant : sftp:, s3:, swift:, b2:, rclone:, ...
   ```
4. Choisir un répertoire de travail pour la config, par exemple
   `/home/user/restic-backups/` — c'est celui utilisé dans les exemples
   ci-dessous.

## 2. `resticconf` — les secrets, jamais transmis au serveur

Créez `/home/user/restic-backups/resticconf`, un fichier shell sourcé
localement par l'agent avant chaque exécution (jamais lu par le serveur, jamais
loggé) :

```bash
# resticconf — permissions 600, uniquement lisible par l'utilisateur de l'agent
export RESTIC_REPOSITORY="/srv/restic-repo"
export RESTIC_PASSWORD_FILE="/home/user/restic-backups/restic-password.txt"

# Exemple backend S3 (facultatif selon votre backend) :
# export AWS_ACCESS_KEY_ID="..."
# export AWS_SECRET_ACCESS_KEY="..."
```

```bash
chmod 600 /home/user/restic-backups/resticconf
chmod 600 /home/user/restic-backups/restic-password.txt
```

Seules les variables dont le nom commence par `RESTIC_`, `OS_`, `SWIFT_`,
`ST_`, `B2_`, `AWS_`, `AZURE_`, `GOOGLE_` ou `RCLONE_` sont transmises par
l'agent au script (`resticEnvAllowedPrefixes`) — tout le reste (alias shell,
`PS1`, etc.) présent dans ce fichier est ignoré, jamais transmis au processus
`run_backup.sh`.

## 3. `resticprofile.yaml` — définir vos profils de sauvegarde

Créez `/home/user/restic-backups/resticprofile.yaml` (un profil par
périmètre à sauvegarder, par exemple `files` et `db`) :

```yaml
version: "1"

global:
  # status-file : lu par l'agent pour le monitoring passif (source privilégiée,
  # sans lui l'agent retombe sur `restic snapshots --json`/`stats --json`).
  status-file: /home/user/restic-backups/backup-status.json

files:
  backup:
    source:
      - /home/user/data
      - /etc
    exclude:
      - "**/.cache"
    extended-status: true

db:
  backup:
    run-before: "pg_dump -U postgres mydb > /home/user/restic-backups/mydb.sql"
    source:
      - /home/user/restic-backups/mydb.sql
    extended-status: true
```

`extended-status: true` est recommandé pour que le status-file contienne le
résultat de chaque exécution de profil.

## 4. `run_backup.sh` — le script que l'agent exécute

Créez `/home/user/restic-backups/run_backup.sh`, exécuté par l'agent avec le
nom du profil en premier argument (vide = profil par défaut de votre script) :

```bash
#!/usr/bin/env bash
set -euo pipefail

PROFILE="${1:-files}"
CONFIG_DIR="/home/user/restic-backups"

exec resticprofile --config "${CONFIG_DIR}/resticprofile.yaml" --name "${PROFILE}" backup --json
```

```bash
chmod +x /home/user/restic-backups/run_backup.sh
```

Le `--json` est indispensable : c'est ce que l'agent parse pour construire la
progression en direct (pourcentage, fichiers/octets traités, ETA) et le résumé
final (`ResticBackupSummary` : snapshot ID, fichiers nouveaux/modifiés,
volume). `RESTIC_PROGRESS_FPS` est injecté automatiquement par l'agent — pas
besoin de le fixer vous-même dans le script (restic désactive sa sortie de
progression par défaut hors TTY, cette variable la force malgré tout).

Testez le script manuellement avant de le brancher à ServerSupervisor :

```bash
source /home/user/restic-backups/resticconf
/home/user/restic-backups/run_backup.sh files
```

## 5. Activer la collecte côté agent (`agent.yaml`)

```yaml
collect_restic: true
restic_bin: "/usr/local/bin/restic"
restic_conf_path: "/home/user/restic-backups/resticconf"
restic_run_script_path: "/home/user/restic-backups/run_backup.sh"
restic_status_file_path: "/home/user/restic-backups/backup-status.json"
restic_profile_config_path: "/home/user/restic-backups/resticprofile.yaml"
restic_enable_progress: true
restic_progress_fps: 0.1
restic_backup_idle_timeout_minutes: 20
```

| Clé | Rôle |
|---|---|
| `collect_restic` | Active la section `restic` du rapport périodique de l'agent |
| `restic_bin` | Chemin du binaire `restic` (défaut : `restic` dans le `PATH`) |
| `restic_conf_path` | Chemin de `resticconf` (secrets, jamais lus par le serveur) |
| `restic_run_script_path` | Chemin de `run_backup.sh`, exécuté par l'action **Lancer un backup** |
| `restic_status_file_path` | Chemin du status-file resticprofile — source privilégiée du monitoring passif |
| `restic_profile_config_path` | Chemin de `resticprofile.yaml` — lu localement pour lister les noms de profils (`files`, `db`, …) et peupler les sélecteurs de profil dans l'UI (backup manuel + tâche planifiée). Seuls les noms sont transmis au serveur, jamais le contenu du fichier |
| `restic_enable_progress` | Active le parsing de la progression en direct pendant un backup manuel |
| `restic_progress_fps` | Fréquence des événements de progression forcés (défaut `0.1`, un toutes les 10s) |
| `restic_backup_idle_timeout_minutes` | Un backup manuel n'a pas de plafond de durée fixe ; il est coupé s'il reste silencieux (aucune ligne `--json`) plus longtemps que cette valeur (défaut 20 min) |

Seuls des chemins et des indicateurs de fonctionnalité vivent dans
`agent.yaml` — jamais un secret. Redémarrez l'agent après modification.

## 6. Vérifier le monitoring passif

Au rapport périodique suivant, l'onglet **Sauvegardes** de la fiche hôte doit
afficher un statut (dernier backup, snapshot, volume) sans qu'aucun backup
n'ait été déclenché depuis ServerSupervisor — c'est la lecture du status-file
(ou, à défaut, de `restic snapshots --json`/`restic stats --json`).

## 7. Déclencher un backup manuel depuis l'UI

Sur la fiche hôte, onglet **Sauvegardes**, le bouton **Lancer un backup**
(visible aux comptes Operator et Admin) dispatche une commande agent
(`module=restic`, `action=run_backup`, `target=<profil>`) qui exécute
directement `run_backup.sh` avec suivi de progression en direct tant que la
page reste ouverte. Fermer l'onglet n'interrompt que l'affichage : le backup
continue côté agent, et le résultat final apparaît au prochain rapport de
statut.

## 8. Planifier un backup récurrent

Il n'y a pas de cron ni de webhook à configurer côté ServerSupervisor. Un
backup récurrent est une tâche planifiée comme une autre :

1. Aller sur **Tâches planifiées**.
2. Créer une tâche avec module `restic`, action `run_backup`.
3. Cible (`target`) = nom du profil resticprofile (`files`, `db`, …) — laisser
   vide pour le profil par défaut défini dans `run_backup.sh`.
4. Choisir la fréquence (cron).

## 9. Dépannage

| Symptôme | Cause probable |
|---|---|
| Statut toujours absent dans l'onglet Sauvegardes | `collect_restic: false`, ou `restic_bin` introuvable dans le `PATH` de l'agent |
| Backup manuel échoue immédiatement, aucun résumé | `restic_run_script_path` absent ou non exécutable (`chmod +x`) |
| Backup manuel échoue avec `resticconf not readable` | Mauvais chemin/permissions sur `restic_conf_path`, ou fichier illisible par l'utilisateur qui exécute l'agent |
| Pas de progression en direct pendant un backup en cours | `restic_enable_progress: false`, ou le script n'appelle pas `restic`/`resticprofile` avec `--json` |
| Backup coupé alors qu'il semblait actif | Aucune ligne `--json` reçue pendant plus de `restic_backup_idle_timeout_minutes` — vérifier que le script ne reste pas bloqué sur une étape sans sortie (ex. `run-before` long et silencieux) |
| Statut passif absent alors que le script fonctionne en manuel | `restic_status_file_path` ne pointe pas vers le `status-file` déclaré dans `resticprofile.yaml`, ou `extended-status` non activé sur le profil |

## Pour aller plus loin

Voir aussi la section [Sauvegardes Restic](../README.md#sauvegardes-restic) du
README pour le résumé des trois modes (passif/actif/planifié) et le tableau
des routes API associées.
