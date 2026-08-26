# Audit produit — ServerSupervisor (août 2026)

Cet audit complète [AUDIT-2025.md](AUDIT-2025.md) (architecture technique, résolu) par une
lecture **produit** : cartographie fonctionnelle, valeur utilisateur, écarts vis-à-vis d'un
outil de supervision complet type Checkmk, et priorisation. Il a été réalisé à partir d'une
inspection directe du code (`server/internal/*`, `agent/internal/*`, `frontend/src/*`,
`docker-compose.yml`, `protocol/`) — les manques listés ci-dessous ont été vérifiés par
recherche dans le code (grep), pas supposés.

Le plan d'exécution détaillé (phases, priorités, fonctionnalités par palier) est dans
**[ROADMAP.md](ROADMAP.md)**. Ce document-ci répond à « où en est-on et pourquoi », ROADMAP.md
répond à « dans quel ordre et pourquoi ».

---

## 0. Résumé exécutif

ServerSupervisor est nettement plus mature qu'un projet self-hosted typique à ce stade : RBAC
3 niveaux + permissions par hôte, MFA TOTP/WebAuthn, moteur d'alertes avec hystérésis et
cooldown, WebSocket temps réel sur les domaines cœur, supervision Proxmox sans agent,
intégration NPM native, exécution de commandes distantes whitelistées, runbooks, tâches
planifiées, suivi de releases + webhooks Git, sauvegardes Restic, analytics trafic/menaces
avec corrélation CrowdSec, PWA. La dette « de fondation » (secrets qui fuitaient, goroutines
non protégées, pas de backup DB) est déjà traitée — voir [AUDIT-2025.md](AUDIT-2025.md).

**L'écart réel avec Checkmk n'est pas featuritude, c'est l'architecture de check.**
ServerSupervisor a des collecteurs agent figés + 3 types de sondes synthétiques (HTTP/TCP/ICMP)
+ polling Proxmox API + un scan de sous-réseau par ping ICMP pour la découverte — pas de moteur
de check générique, pas de plugins, pas de découverte réseau au-delà du ping (pas d'ARP, pas de
scan de ports). Confirmé par recherche dans le code : **aucun SNMP** (l'absence d'ICMP listée
ici à l'origine, ainsi que la fenêtre de maintenance, l'escalade d'alerte, les templates de
règles réutilisables cross-host et la découverte de sous-réseau, sont depuis corrigées — voir
ROADMAP.md items #2, #3, #8, #9 et #12).
C'est le choix structurant à trancher avant d'investir davantage : rester un outil
« agent-first, zéro friction » ou basculer vers un moteur de check extensible façon Checkmk
(bien plus de valeur long terme, bien plus de complexité).

Les manques les plus limitants pour un usage NOC/ops sérieux au quotidien : pas de canal
Slack/Teams/Discord/webhook générique, pas de groupes d'hôtes dérivés des tags. Quatre items listés ici à
l'origine sont depuis **corrigés** : la faille RBAC sur la création/modification de tâches
planifiées (voir [ROADMAP.md](ROADMAP.md) item #1 et
[Runbooks-and-Scheduled-Tasks](https://github.com/Rem7474/ServerSupervisor/wiki/Runbooks-and-Scheduled-Tasks#3-lasymétrie-en-un-coup-dœil)),
l'absence de fenêtres de maintenance (item #2, `internal/services/maintenance`), l'absence
d'escalade/acquittement d'incident (items #3/#4, `AlertActions.EscalateAfterMinutes` +
`AcknowledgeIncident`) et l'absence de déduplication (item #5, `alert_incidents.correlated_with`
— scope MVP : un incident sur un hôte, un container/projet Docker sur cet hôte, ou une VM/CT
Proxmox avec un lien confirmé, se corrèle avec l'incident `status_offline`/`heartbeat_timeout`
ouvert de ce même hôte et n'envoie pas sa propre notification tant que celui-ci reste ouvert ;
ça couvre le cas concret cité — un hôte down faisant tomber tous ses containers Docker d'un
coup — pas un moteur de corrélation général entre alertes par ailleurs indépendantes).

**Recommandation directrice** : ne pas refondre. Fermer d'abord les trous d'exploitabilité
restants (groupes d'hôtes, canal webhook générique) avant d'élargir encore la couverture de
check (SNMP, plugins — ICMP est fait, item #8). Trancher
explicitement l'ambition « moteur de check extensible » avant de la communiquer — c'est la
seule décision qui change l'ordre de grandeur de l'effort à venir.

---

## 1. État des lieux fonctionnel

| Domaine | Ce qui existe | Valeur produit | Maturité | Manques confirmés | À consolider |
|---|---|---|---|---|---|
| **Supervision système** | Agent Go : CPU/RAM/disque/réseau/uptime, Docker, APT+CVE, S.M.A.R.T., température, systemd, journal, processus. Protocole agent↔serveur verrouillé par test contractuel golden-fixture (`protocol/`). | Forte — cœur du produit | Élevée | Pas de checks custom pluggables (seulement exécution de scripts allowlistés via `tasks.yaml`, pas de métriques/seuils arbitraires) ; agent Linux uniquement (hypothèse — aucune mention Windows/macOS trouvée dans le code) | Rien de majeur, module le plus mûr |
| **Supervision réseau** | Topologie Docker (liens réseau, overrides manuels), sondes HTTP/TCP/ICMP (ROADMAP.md item #8) | Topologie Docker = différenciant réel | Moyenne | Pas de SNMP, pas de cartographie réseau physique L2/L3 | — |
| **Inventaire / découverte** | Ajout d'hôte manuel (wizard), auto-import Proxmox (guests) et NPM (proxy hosts), scan de sous-réseau IPv4 par ping ICMP (`/24` à `/30`, ROADMAP.md item #12) avec ajout en masse des adresses trouvées | Onboarding correct à petite/moyenne échelle | Moyenne | Découverte limitée à un ping-sweep (pas d'ARP, pas de scan de ports, pas de fingerprinting) ; `hosts.tags` existe et sert au filtrage, mais rien ne dérive de groupe à partir d'un tag | Devient un frein au-delà de ~50 hôtes sans groupes dynamiques |
| **Alertes / notifications** | Moteur avec hystérésis warn/crit + seuils de clear, cooldown, 3 sources (`agent`/`proxmox`/`synthetic`), déclenchement de commande (`command_trigger`), fenêtres de maintenance (`internal/services/maintenance`), acquittement + escalade d'incident (`AcknowledgeIncident`, `AlertActions.EscalateAfterMinutes`), corrélation host-down → cascade Docker/Proxmox (`alert_incidents.correlated_with`). Canaux : SMTP, ntfy, push navigateur, in-app. | Cœur différenciant, anti-flapping déjà pensé | Élevée sur le moteur, faible sur l'écosystème de canaux | Pas de Slack/Teams/Discord/webhook générique | Clarifier la vraie liste de canaux (le « webhook » évoqué en prose README recouvre en fait ntfy) |
| **Dashboards** | Dashboard fleet temps réel (KPIs, statuts, drift versions Docker, résumé Proxmox), WS-driven | Bon point d'entrée quotidien | Élevée | Pas de dashboard personnalisable, pas de vue « santé globale » agrégée type SLA | — |
| **Logs / événements** | Journalctl streamé par hôte, historique de commandes, web logs (trafic + menaces) avec corrélation CrowdSec | Bon niveau debug ad hoc, web logs très travaillés | Moyenne-élevée sur le web, faible sur logs applicatifs génériques | Pas de recherche full-text cross-host sur les logs système | — |
| **Configuration** | Settings globaux en DB, override par variables d'env, cartes settings par domaine | Correcte | Bonne | Pas d'export/import config-as-code, pas de versionning des règles d'alerte | — |
| **Auth / rôles** | JWT + refresh tokens, MFA TOTP/WebAuthn, RBAC 3 niveaux (admin/operator/viewer) + permissions par hôte, audit logs, blocage IP persistant en DB | Solide pour du self-hosted mono-tenant | Élevée sur l'authentification | Aucun — le CRUD des tâches planifiées est désormais vérifié Operator+ comme `run` (voir ROADMAP.md item #1) | — |
| **Intégrations** | Proxmox (riche), NPM (riche), GitHub/GitLab/Gitea + registres Docker, Git webhooks, CrowdSec, Restic | Différenciant réel — peu d'outils combinent tout ça nativement | Élevée | Pas de Prometheus/Grafana, pas de Slack/Teams/Discord, pas d'API publique documentée pour tiers | — |
| **Reporting / historisation** | TimescaleDB (hypertables), rétention configurable **par catégorie** pour le journal d'audit (ROADMAP.md item #13, `models.AuditCategories`), export CSV du journal d'audit, historique commandes | Bonne base technique | Bonne côté stockage | Pas de reporting périodique disponibilité/incidents ni d'export PDF (item #10, distinct de l'export CSV du journal d'audit déjà livré) | — |
| **Administration** | Users, Settings, Audit, RBAC | Correcte pour équipe unique | Bonne | Pas de multi-tenant/organisations | — |

---

## 2. Vision produit

**Proposition de valeur actuelle (implicite, à rendre explicite)** : un cockpit agent-first,
zéro-friction, pour équipes infra qui gèrent elles-mêmes leur parc (VMs, conteneurs, Proxmox,
reverse-proxy) — combinant métriques, gestion de paquets/services, remédiation à distance
sécurisée (whitelist stricte) et automatisation (runbooks, tâches planifiées, webhooks Git)
dans un seul outil installable en quelques minutes.

**Angle différenciant crédible face à Checkmk** : Checkmk est une plateforme d'*observation*
exhaustive (plugins, découverte, SNMP, échelle entreprise) au prix d'une complexité de
configuration élevée. ServerSupervisor a déjà, contrairement à Checkmk, la **remédiation
intégrée** (exécution de commandes, runbooks, déploiement automatique sur nouvelle
release/push Git) directement liée à la supervision. C'est l'angle à assumer : « supervision
+ remédiation légère », pas « supervision exhaustive ».

- **Cœur de produit** : métriques agent, moteur d'alertes, exécution de commandes distantes
  whitelistées, awareness Proxmox/NPM/Docker, dashboards temps réel.
- **Secondaire** (garder en périphérie, ne pas dénaturer) : analytics trafic/menaces web,
  supervision Restic, suivi de releases.
- **À ne pas faire trop tôt** : multi-tenant, marketplace de plugins, moteur de check générique
  complet façon Checkmk, SNMP exhaustif, agent Windows/macOS, dashboard builder généraliste,
  SSO/OIDC — aucun n'est justifié par une demande utilisateur avérée à ce stade.

---

## 3. Lecture architecture — backend

| Sujet | État actuel | À améliorer | À éviter de sur-architecturer |
|---|---|---|---|
| Séparation des responsabilités | Déjà propre : `handlers` → `services/<domaine>` (port Repository) → `database`. Erreurs typées (`apperr`), goroutines protégées (`safego`). | Rien d'urgent | Ne pas ajouter de couche supplémentaire « juste pour la forme » |
| Modèle de données | Un fichier de modèle par domaine (`internal/models/`), pas de `models.go` monolithique | OK | — |
| Gestion des checks | Collecteurs agent fixes + sondes synthétiques HTTP/TCP/ICMP (item #8) + poll Proxmox — **pas de modèle « Check » générique unifié** | Introduire un modèle léger `Check{type, target, interval, thresholds}` couvrant HTTP/TCP/ICMP et un futur SNMP, sans runtime de plugin | Ne pas construire un vrai moteur de plugins tant que la demande n'est pas prouvée |
| Collecte de métriques | Solide : push agent 30s, TimescaleDB hypertables | — | — |
| Ingestion d'événements | Bus pub/sub in-process minimal (`internal/events`), volontairement single-instance | Cohérent avec le déploiement actuel (1 conteneur `server`) | Ne pas introduire Redis avant un besoin réel de scale horizontal — et si besoin, migrer bus + WS hubs + rate limiter *ensemble* |
| Planification / scheduler | Deux mécanismes cohérents : `poller.Every` (pollers génériques) + `scheduler.TaskScheduler` (cron) | Réutilisable tel quel pour de futurs checks actifs | Ne pas fusionner artificiellement les deux |
| Stockage des états | Postgres/TimescaleDB, pas de cache | — | Pas de Redis prématuré |
| Historisation | Bonne (politiques de rétention) | Manque exports/rapports | — |
| Gestion des règles | Moteur d'alertes correct (hystérésis, cooldown), templates réutilisables cross-host (`alert_rule_templates`, ROADMAP.md item #9) | Pas encore de règles par tag : les tags existent (`hosts.tags`, utilisables pour filtrer les hôtes à l'application d'un modèle) mais aucune règle ne cible un tag — item #7 | — |
| API | REST bien organisée par domaine | Aucun — le RBAC sur le CRUD des tâches planifiées, manquant à l'origine, est corrigé (ROADMAP.md item #1) | — |
| Extension / plugins | Absent — plus gros écart structurel avec Checkmk | Élargir encore le nombre de types de check « en dur » (SNMP basique — ICMP fait, item #8) avant tout runtime de plugin | Ne pas construire de plugin engine sans demande prouvée |
| Sécurité | Bon niveau (JWT, MFA, rate limiting, audit, secrets jamais renvoyés au frontend) | Pas de secret scanning ni signature d'image en CI, Trivy non bloquant — dette silencieuse, effort faible | — |
| Scalabilité | Assumée single-instance par design (WS hubs, event bus, rate limiter, store WebAuthn) — cohérent avec `docker-compose.yml` actuel | Anticiper explicitement avant toute promesse de HA/clustering | Ne pas scaler une seule brique isolément |

## 4. Lecture architecture — frontend

- **Navigation** : 29 routes, déjà des efforts réels de consolidation (`/commands`→Audit,
  `/notifications`→Alerts, Uptime+SSL→Monitoring). Discipline à poursuivre.
- **Hiérarchie de l'info** : Dashboard → détail hôte/Proxmox (drill-down) → domaines
  transverses (Docker/Network/APT/Alerts/Audit/Traffic-Threats) → automatisation
  (Runbooks/Tâches/Releases/Webhooks) → admin. Cohérente, pas de refonte structurelle
  nécessaire.
- **Écrans manquants** : pas de status page publique/partageable, pas de vue « santé globale »
  agrégée cross-domaine. La vue « incidents actifs » transverse type war-room listée ici à
  l'origine est depuis **corrigée** (ROADMAP.md item #11, `WarRoomPanel.vue` — onglet « Vue
  active », nouvel onglet par défaut de `/alerts`, groupé par sévérité).
- **Temps réel** : bien ciblé (Dashboard/Host/Docker/Network/APT/Alerts/Notifications/Audit/
  Commandes). Proxmox et les domaines d'automatisation restent en polling — choix documenté et
  cohérent, **pas un gap à corriger** sans preuve d'usage contraire.
- **Mobile** : PWA réelle (manifest, service worker, shortcuts, `share_target`) mais assets
  visuels (icônes/captures) sont des placeholders — à finir avant de la vendre comme
  différenciateur.
- **Système de design** : mature et auto-critique (règles ESLint custom, stylelint,
  conventions documentées avec historique des incohérences corrigées) — à préserver
  strictement.
- **Dette UX mineure mais réelle** : pas d'`AppModal` unifié, chaque modale re-implémente
  `useModalChrome`.
- **i18n** : français uniquement — choix assumé, à transformer en décision produit explicite
  plutôt qu'en dette accidentelle.

---

## 5. Risques et dettes

- ✅ **RBAC scheduled-tasks** : corrigé (voir ROADMAP.md item #1) — création/modification/
  suppression sont désormais vérifiées Operator+ par hôte, comme `run`.
- 🟡 **Architecture single-instance assumée** (WS hubs, event bus, rate limiter, store
  WebAuthn) : pas un problème aujourd'hui, mais toute promesse de HA/clustering sans ce
  chantier groupé serait trompeuse.
- 🟡 **Migrations forward-only sans rollback tooling** : mitigé par une procédure
  `pg_dump`/`pg_restore` testée bout-en-bout (voir README), mais dépend de la discipline
  humaine (snapshot avant upgrade) — automatiser le snapshot pré-migration réduirait le
  risque.
- 🟡 **CI incomplète** : Trivy non bloquant, pas de secret scanning, pas de gate de
  couverture, tests navigateur désactivés (bug amont Vite/Vitest) — dette silencieuse qui
  grossit si non traitée.
- 🟢 **Absence d'auto-observabilité du serveur** (pas de `/metrics`, pas d'OTel) — ironique
  pour un outil de supervision, gênant pour diagnostiquer l'outil lui-même à plus grande
  échelle.
- 🟢 **i18n non traité** : verrou de marché FR-only — à assumer consciemment ou budgéter.
- 🟢 **PWA avec assets placeholder** : risque de crédibilité si mise en avant comme feature
  mobile finie.
- 🟢 **Dépendance au modèle agent-first** : toute cible non-agentable (équipement réseau,
  IoT, cloud managé) reste hors de portée sans SNMP/ICMP/checks API-based — limite le marché
  adressable actuel.

---

## 6. Questions prioritaires à trancher

1. Le produit vise-t-il en priorité les mêmes équipes actuelles (self-hosted/PME infra
   interne) ou une montée en gamme NOC/MSP professionnel ? Conditionne l'investissement en
   maintenance/escalade/SLA vs en simplicité d'onboarding.
2. Faut-il un vrai moteur de check extensible (plugins) à moyen terme, ou le scope figé
   (agent + HTTP/TCP + Proxmox) suffit-il à l'ambition réelle ? C'est LE choix qui détermine
   si « façon Checkmk » reste un slogan ou devient un chantier pluri-mois.
3. Le FR-only est-il un choix de marché définitif ou une dette à lever dès qu'un client
   anglophone se présente ?
4. Y a-t-il une demande réelle de scale horizontal (plusieurs instances serveur) à 12 mois,
   ou le mono-instance reste-t-il l'hypothèse de déploiement de référence ?
5. Le produit doit-il rester interne à une seule équipe infra, ou faut-il anticiper un usage
   multi-équipes (même léger) qui impacterait RBAC et tagging dès maintenant plutôt que plus
   tard ?

---

## 7. Recommandation finale

Ne pas refondre — la base (couches, sécurité, moteur d'alertes, temps réel, design system) est
saine et déjà auto-critiquée par l'équipe elle-même. Fermer d'abord les trous d'exploitabilité
connus et documentés (groupes d'hôtes, canal webhook générique — RBAC tâches planifiées, dédup,
maintenance windows et escalade/ack sont déjà corrigés) : effort limité, impact immédiat sur la
confiance en production.
Trancher ensuite, explicitement et avant toute communication publique, le pari « moteur de
check extensible façon Checkmk » — c'est la seule décision qui change réellement l'ordre de
grandeur des mois à venir. Le reste (SNMP, status page, SLO, scale-out) doit rester conditionné
à une demande utilisateur prouvée, pas anticipé par défaut.

→ Plan d'exécution détaillé : **[ROADMAP.md](ROADMAP.md)**.
