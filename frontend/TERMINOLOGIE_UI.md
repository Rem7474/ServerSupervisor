# Mini charte editoriale UI

Cette charte fixe le vocabulaire des pages web analytiques pour garder des libelles coherents.

## Termes de reference

- Chemin: segment URL requete (ex: /wp-login.php). Remplace Path ou path.
- Hote technique: machine supervisee (serveur/agent) identifiee par ID interne. Remplace Host quand on parle infrastructure.
- Domaine cible: domaine virtuel vise par la requete HTTP (Host header/vhost). Remplace vhost, host cible, domaine selon contexte web.
- IP cliente: adresse source de la requete.
- Requete: evenement HTTP unitaire.
- Tranche: regroupement temporel d une timeline (bucket).

## Regles d usage

- Dans les tableaux et filtres web, preferer Chemin a Path.
- Utiliser Hote technique uniquement pour les filtres relies a l ID d hote du systeme.
- Utiliser Domaine cible pour l analyse trafic, menaces, logs et details domaine.
- Eviter le melange Host/Hote/Domaine dans la meme section.
- Garder les labels courts et explicites: ex. Top chemins, Domaine cible, Trafic - requetes par tranche.
