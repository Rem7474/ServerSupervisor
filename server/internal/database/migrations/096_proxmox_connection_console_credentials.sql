-- Optional PVE user credentials per connection, used only to open an
-- interactive LXC console (termproxy/vncwebsocket). PVE's vncwebsocket
-- upgrade endpoint requires a real user ticket, not the PVEAPIToken auth
-- used for every other call — see proxmoxclient.Client.login.
ALTER TABLE proxmox_connections
    ADD COLUMN pve_username TEXT NOT NULL DEFAULT '',
    ADD COLUMN pve_password TEXT NOT NULL DEFAULT '';
