package handlers

import (
	"fmt"
	"strconv"

	"github.com/serversupervisor/server/internal/models"
)

// parseVMID converts a Proxmox task object ID string to an integer VMID.
// Returns 0 if the string is not a valid positive integer.
func parseVMID(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// resolveSecret returns the token secret and connection details for a connection ID.
// It reads the secret from GetEnabledProxmoxConnections (which includes TokenSecret).
func (h *ProxmoxHandler) resolveSecret(connectionID string) (secret string, conn *models.ProxmoxConnection, err error) {
	conns, err := h.db.GetEnabledProxmoxConnections()
	if err != nil {
		return "", nil, err
	}
	for _, co := range conns {
		if co.ID == connectionID {
			secret = co.TokenSecret
			break
		}
	}
	if secret == "" {
		return "", nil, fmt.Errorf("connection not found or disabled")
	}
	c, err := h.db.GetProxmoxConnectionByID(connectionID)
	if err != nil || c == nil {
		return "", nil, fmt.Errorf("failed to load connection")
	}
	return secret, c, nil
}
