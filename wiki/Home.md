# ServerSupervisor

Système de supervision d'infrastructure : monitoring de VMs, conteneurs Docker, mises à jour
APT, services systemd, tâches planifiées, suivi des releases GitHub, supervision Proxmox VE
via API et monitoring synthétique (sondes uptime, certificats SSL, intégration Nginx Proxy
Manager).

Ce wiki couvre les intégrations qui demandent une vraie procédure de configuration côté
service tiers (dépannage inclus). Pour l'installation, la liste complète des fonctionnalités
et la référence API, voir le
[README](https://github.com/Rem7474/ServerSupervisor/blob/main/README.md) du dépôt.

## Guides

| Guide | Sujet |
|---|---|
| [Proxmox](Proxmox.md) | Connecter un cluster Proxmox VE (token API, permissions, actions en écriture) |
| [NPM](NPM.md) | Connecter Nginx Proxy Manager (sync, monitoring auto par proxy host) |
| [Git-Webhooks-and-Releases](Git-Webhooks-and-Releases.md) | Webhooks Git et suivi de releases GitHub/GitLab/Gitea/Docker |
| [Runbooks-and-Scheduled-Tasks](Runbooks-and-Scheduled-Tasks.md) | Runbooks multi-étapes vs tâches planifiées par hôte |
| [Restic-Backups](Restic-Backups.md) | Sauvegardes Restic (installation, resticprofile, déclenchement) |
| [Restic-Example-Nextcloud-AIO](Restic-Example-Nextcloud-AIO.md) / [Restic-Example-Immich](Restic-Example-Immich.md) | Recettes resticprofile complètes pour deux apps auto-hébergées courantes |
| [Alerting](Alerting.md) | Moteur d'alertes : seuils, hystérésis, incidents, ack/escalade, corrélation, maintenance, modèles |
| [Custom-Tasks-Examples](Custom-Tasks-Examples.md) | Exemples de `tasks.yaml` prêts à copier |

## Autres ressources

- [Vision produit & roadmap](https://github.com/Rem7474/ServerSupervisor/blob/main/ROADMAP.md)
- [Signaler une vulnérabilité](https://github.com/Rem7474/ServerSupervisor/blob/main/SECURITY.md)
- [Contribuer](https://github.com/Rem7474/ServerSupervisor/blob/main/CONTRIBUTING.md)

---

*Ce wiki est généré automatiquement depuis le dossier [`wiki/`](https://github.com/Rem7474/ServerSupervisor/tree/main/wiki)
du dépôt à chaque push sur `main` — toute modification doit passer par une pull request sur
ce dossier, pas par l'éditeur du wiki GitHub (il serait écrasé au prochain déploiement).*
