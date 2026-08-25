# Exemples de `tasks.yaml`

Recueil d'exemples prêts à copier pour le fichier de tâches custom de
l'agent, par défaut `/etc/serversupervisor/tasks.yaml`.

La référence des champs (`id`, `name`, `command`, `timeout`), les variables
`SS_*` injectées par un webhook Git et les trois façons de déclencher une
tâche sont dans le [README](../README.md#tâches-custom-tasksyaml). Ce fichier
ne contient que des exemples.

> **Rappel sur le shell.** L'agent exécute `command` comme un argv, sans
> shell : le serveur ne peut désigner qu'un `id` déjà présent dans ce
> fichier, jamais une commande arbitraire. Les exemples ci-dessous utilisent
> volontairement `["bash", "-c", "…"]` parce qu'ils enchaînent plusieurs
> commandes — ce faisant, ils réintroduisent un shell **à l'intérieur de
> votre propre commande**. C'est parfaitement légitime ici (rien n'y est
> interpolé depuis l'extérieur), mais dès qu'une variable `SS_*` venue d'un
> webhook entre dans la chaîne, préférez un script dédié.

## Mettre à jour des stacks Docker Compose

Un `id` par stack, pour pouvoir les déclencher indépendamment :

```yaml
tasks:
  - id: docker-pull-teslamate
    name: "docker pull and start Teslamate"
    command: ["bash", "-c", "cd /directory/docker/teslamate && docker compose pull && docker compose up -d"]
    timeout: 3600

  - id: docker-pull-bar-assistant
    name: "docker pull and start BarAssistant"
    command: ["bash", "-c", "cd /directory/docker/bar-assistant && docker compose pull && docker compose up -d"]
    timeout: 3600

  - id: docker-pull-mealie
    name: "docker pull and start Mealie"
    command: ["bash", "-c", "cd /directory/docker/mealie && docker compose pull && docker compose up -d"]
    timeout: 3600
```

`timeout: 3600` (1 h) est confortable pour un `pull` de plusieurs images sur
une connexion lente ; le maximum accepté est 3600 secondes.

> Pour des conteneurs gérés par Compose, le [suivi de
> releases](git-webhooks-releases.md) fait déjà `compose pull` + `up -d`
> automatiquement à la détection d'une nouvelle image. Ces tâches restent
> utiles pour un déclenchement manuel, un hôte sans tracker, ou un
> enchaînement qui dépasse la simple mise à jour.

## Maintenance

```yaml
tasks:
  - id: cleanup_logs
    name: "Nettoyer les vieux logs"
    command: ["find", "/var/log", "-name", "*.log", "-mtime", "+30", "-delete"]
    timeout: 120

  - id: backup_db
    name: "Backup PostgreSQL"
    command: ["pg_dump", "-U", "postgres", "mydb", "-f", "/backups/db.sql"]
    timeout: 300
```

Ces deux-là n'ont pas besoin de shell : un argv direct suffit, c'est la forme
à privilégier.

## Déploiement déclenché par un webhook Git

Le script reçoit `SS_REPO_NAME`, `SS_BRANCH`, `SS_COMMIT_SHA`,
`SS_COMMIT_MESSAGE`, `SS_PUSHER`, `SS_WEBHOOK_NAME` et `SS_EVENT_TYPE` dans
son environnement.

```yaml
tasks:
  - id: deploy-test
    name: "Deploy /home/root/test"
    command: ["/opt/scripts/deploy-test.sh"]
    timeout: 120
```

```bash
#!/bin/bash
set -euo pipefail
cd /home/root/test
git pull origin "${SS_BRANCH:-main}"
echo "Déploiement terminé (commit: ${SS_COMMIT_SHA:-inconnu})"
```

Passer par un script plutôt que par un `bash -c` inline garde la variable
entre guillemets et le `set -euo pipefail` au bon endroit — c'est la forme
recommandée dès qu'un webhook alimente la commande.
