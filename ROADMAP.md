# Roadmap produit — ServerSupervisor

Plan d'exécution issu de **[AUDIT-PRODUIT-2026.md](AUDIT-PRODUIT-2026.md)** (état des lieux
fonctionnel, vision produit, lecture d'architecture). Ce document répond à « dans quel ordre
et pourquoi » ; l'audit répond à « où en est-on et pourquoi ».

Statut à maintenir au fil de l'eau : `Non démarré` / `En cours` / `Fait` / `Écarté` (avec
justification si écarté). Ne pas laisser ce document dériver du code réel — le mettre à jour
au moment où un item change de statut, pas rétroactivement en bloc.

---

## 1. Fonctionnalités manquantes, par palier

### Must have — ferme les trous d'exploitabilité connus

| # | Fonctionnalité | Pourquoi maintenant | Statut |
|---|---|---|---|
| 1 | RBAC serveur sur create/update/delete des tâches planifiées | Faille de sécurité documentée mais non corrigée (README : seul `run` est vérifié Operator+) | Non démarré |
| 2 | Fenêtres de maintenance | Aucune existante (confirmé par grep) — bruit d'alerte pendant les interventions planifiées | Non démarré |
| 3 | Escalade d'alertes (relance après N minutes si non acquittée) | Aucune existante — un incident critique non vu reste silencieux | Non démarré |
| 4 | Acquittement (ack) d'incident + statut « en cours de traitement » | Prérequis technique de l'escalade (#3) et du dédup (#5) | Non démarré |
| 5 | Déduplication / groupement d'alertes corrélées | Un hôte down peut aujourd'hui déclencher une cascade d'alertes filles indépendantes | Non démarré |
| 6 | Canal Slack / Teams / Discord / webhook générique | Seuls SMTP et ntfy existent réellement (le « webhook » du README recouvre ntfy) | Non démarré |
| 7 | Tagging des hôtes + groupes dynamiques | Aucun tagging trouvé dans le modèle de données — bloque le filtrage à l'échelle et les règles par groupe | Non démarré |

### Should have — élargit la couverture sans agent

| # | Fonctionnalité | Pourquoi | Statut |
|---|---|---|---|
| 8 | Check ICMP générique (indépendant de l'agent) | Aucun ICMP dans le code — équipements non-agentables hors de portée | Non démarré |
| 9 | Templates de règles d'alertes réutilisables cross-host | Chaque règle est créée individuellement aujourd'hui | Non démarré |
| 10 | Export / rapport périodique (disponibilité, incidents) | Aucun export PDF/CSV trouvé | Non démarré |
| 11 | Vue « incidents actifs » transverse (war-room) | `AlertsView` liste, ne regroupe pas visuellement par sévérité/statut | Non démarré |
| 12 | Découverte réseau basique (scan de sous-réseau) | Onboarding actuel = ajout manuel un par un | Non démarré |
| 13 | Audit log : rétention configurable par catégorie, export | Rétention actuelle globale (`AUDIT_RETENTION_DAYS`), pas par type d'événement | Non démarré |
| 14 | API publique documentée + clés dédiées pour intégrations tierces | L'API existe mais orientée frontend/agent, pas pensée « client tiers » | Non démarré |

### Could have — paris à valider par la demande

| # | Fonctionnalité | Statut |
|---|---|---|
| 15 | SNMP basique (compteurs simples, pas un moteur MIB complet) | Non démarré |
| 16 | Status page publique en lecture seule | Non démarré |
| 17 | SLA/SLO par service critique | Non démarré |
| 18 | Dashboard personnalisable (widgets réordonnables) | Non démarré |
| 19 | Endpoint `/metrics` Prometheus (scrape externe) | Non démarré |
| 20 | Séparation légère multi-équipes (sans plein multi-tenant) | Non démarré |

### Later / optional — ne pas engager sans demande prouvée

21. Vrai système de plugins/checks custom exécutables côté serveur
22. Agent Windows/macOS
23. Corrélation d'événements avancée (suppression topology-aware)
24. Multi-tenant complet
25. SSO/OIDC
26. Marketplace communautaire de plugins

---

## 2. Roadmap par horizon

### Horizon 1 — Court terme (1-2 mois) : rendre l'alerting exploitable en prod

**Objectif** : passer de « détecte » à « vivable au quotidien sans se noyer dans le bruit ».

**Livrables** : items #1 à #7 (Must have).

**Risques** : touche le moteur d'alertes existant (`internal/alerts` — hystérésis, cooldown) ;
risque de régression sur un mécanisme déjà bien réglé → couverture de tests avant/après
obligatoire, pas de raccourci.

**Valeur utilisateur** : forte et immédiate — moins de bruit, confiance accrue dans les
alertes reçues, fermeture d'une vraie faille RBAC.

**Valeur technique** : faible dette ajoutée ; comble des trous déjà identifiés et documentés
dans le code/README, pas de nouveau pari architectural.

### Horizon 2 — Moyen terme (3-6 mois) : élargir la couverture, industrialiser l'onboarding

**Objectif** : couvrir des cibles non-agentables et réduire la friction d'ajout à l'échelle
(30-100 hôtes).

**Livrables** : items #8 à #14 (Should have). Introduit au passage un modèle `Check` unifié
côté backend (voir AUDIT-PRODUIT-2026.md §3) pour porter HTTP/TCP/ICMP sous une même
abstraction, sans construire de runtime de plugin.

**Risques** : ICMP et découverte réseau nécessitent des privilèges réseau (raw socket / accès
au sous-réseau) — à valider avec le modèle de conteneurisation actuel (`docker-compose.yml`,
un seul conteneur `server`) avant de s'engager. Attention à la charge ajoutée sur le poller si
le scan réseau est mal borné.

**Valeur utilisateur** : onboarding significativement plus rapide à l'échelle, moins d'angles
morts (équipements sans agent possible).

**Valeur technique** : le modèle `Check` unifié devient la fondation propre pour un futur
SNMP, sans sur-architecturer maintenant.

### Horizon 3 — Long terme (6-12 mois+) : trancher le pari stratégique

**Objectif** : exécuter le positionnement différenciant choisi (voir Questions prioritaires,
AUDIT-PRODUIT-2026.md §6) plutôt que d'ajouter des fonctionnalités au fil de l'eau.

**Livrables candidats** (à arbitrer selon le retour des horizons précédents et les réponses
aux questions prioritaires) : items #15 à #20 (Could have) — SNMP basique, status page,
SLA/SLO, dashboard personnalisable, export Prometheus, scale-out (Redis) si la base
utilisateur le justifie réellement.

**Risques** : sur-ingénierie si engagé sans demande prouvée — c'est le principal risque de cet
horizon, pas un risque technique.

**Valeur utilisateur** : dépend fortement du profil de client à ce stade (PME interne vs offre
packagée plus large) — à valider avant d'investir, pas à anticiper par défaut.

**Valeur technique** : les gros chantiers d'architecture (scale-out notamment) doivent être
groupés — event bus + WS hubs + rate limiter migrent ensemble vers un backend partagé (Redis),
jamais pièce par pièce (voir `CLAUDE.md`, section serveur).

---

## 3. Priorités

### 10 prochaines choses à faire

1. RBAC serveur sur tâches planifiées (sécurité, dette connue, effort faible)
2. Fenêtres de maintenance
3. Escalade + ack d'incidents
4. Déduplication / groupement d'alertes
5. Canal webhook générique (Slack/Teams/Discord)
6. Tagging des hôtes
7. Secret scanning + Trivy bloquant en CI
8. Finir les assets PWA réels (icônes, captures — actuellement des placeholders SVG)
9. Mettre à jour les captures d'écran (`screenshot/` ne couvre que APT/Audit/Dashboard/
   Docker/HostDetail ; Proxmox, Network, Traffic/Threats, Monitoring, Runbooks n'en ont
   aucune alors que ce sont des fonctionnalités substantielles)
10. Décision explicite et documentée sur le scope i18n (rester FR-only assumé, ou prévoir
    l'anglais)

### 10 choses à ne pas faire tout de suite

1. Système de plugins/checks exécutables généraux
2. Multi-tenant complet
3. SSO/OIDC
4. Agent Windows/macOS (sauf demande utilisateur avérée)
5. Scale-out horizontal (Redis, WS partagé) sans besoin réel prouvé
6. Dashboard builder généraliste
7. SNMP complet avec support MIB étendu
8. Marketplace communautaire de plugins
9. Refonte de la navigation/structure frontend (déjà saine, pruning actif en cours)
10. Fusion forcée des onglets systemd/historique Host vs Proxmox (différence de modèle de
    données assumée et documentée — `CLAUDE.md`, section « Host detail / Proxmox node tab
    shell » — ne pas artificialiser une cohérence UX qui n'existe pas dans les données)

### 5 paris stratégiques les plus intéressants

1. Assumer « supervision + remédiation intégrée » comme positionnement (vs Checkmk =
   observation pure) — capitaliser sur runbooks/tâches planifiées/webhooks Git déjà là.
2. Modèle de check générique unifié plutôt qu'un plugin engine complet — viser 80 % de la
   valeur perçue « façon Checkmk » pour 20 % du coût d'architecture.
3. Tagging + groupes dynamiques comme fondation structurante — un investissement qui débloque
   en cascade alertes, dashboards et RBAC futur.
4. Status page publique en self-service — réutilise l'infra uptime/SSL déjà construite,
   différenciant crédible face aux outils dédiés type Statuspage.io.
5. API publique + webhooks sortants documentés — ouvre l'écosystème (scrape Prometheus,
   automatisations tierces) sans construire une intégration bidirectionnelle lourde.

---

## 4. Questions prioritaires (rappel)

Ces questions conditionnent l'ordre réel d'exécution ci-dessus — voir
[AUDIT-PRODUIT-2026.md §6](AUDIT-PRODUIT-2026.md#6-questions-prioritaires-à-trancher) pour le
détail. Tant qu'elles ne sont pas tranchées, l'Horizon 3 ne doit pas être engagé au-delà de la
veille/exploration.
