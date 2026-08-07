# Superviser un cluster Proxmox VE

1. Dans Proxmox, créer un token API avec les permissions minimales en lecture :
   ```
   # Rôle lecture seule (nœuds, VMs, LXC, stockage, disques)
   pveum role add SSAuditor -privs "Datastore.Audit Sys.Audit VM.Audit"
   pveum user add supervision@pve
   pveum aclmod / -user supervision@pve -role SSAuditor
   pveum user token add supervision@pve monitoring --privsep 0
   ```
   Copier le `token ID` (ex : `supervision@pve!monitoring`) et le `secret` affiché.

   > **Mises à jour apt (optionnel)** : l'endpoint `/nodes/{node}/apt/update` requiert `Sys.Modify`.
   > Si vous souhaitez voir les paquets en attente et rafraîchir le cache depuis le dashboard,
   > ajoutez ce privilege au rôle ou créez un second rôle complémentaire :
   > ```
   > pveum role modify SSAuditor -privs "Datastore.Audit Sys.Audit Sys.Modify VM.Audit"
   > ```
   > **Important** : si votre token a "Privilege Separation" activé (coché par défaut à la création),
   > les permissions doivent être assignées **directement au token** et pas seulement à l'utilisateur :
   > ```
   > pveum aclmod / -token supervision@pve!monitoring -role SSAuditor
   > ```

2. Dans ServerSupervisor → **Paramètres** → carte **Proxmox VE** → **Ajouter une connexion** :
   - Nom : label interne (ex : `Cluster prod`)
   - URL API : `https://pve.example.com:8006/api2/json`
   - Token ID : `supervision@pve!monitoring`
   - Token secret : le secret copié
   - Cocher **Ignorer TLS** uniquement si le certificat est auto-signé

3. Cliquer **Tester la connexion** pour valider, puis **Créer**.

4. La première collecte démarre automatiquement. L'entrée **Proxmox** apparaît dans la navigation.
