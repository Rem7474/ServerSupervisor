# 🎯 Frontend Implementation Guide - ServerSupervisor v1.1

Date: 2026-02-20  
Backend Status: ✅ Complete and compiled  
Next Phase: Frontend UI updates

---

## Overview

4 major backend features have been implemented:
1. ✅ Audit Log (APT action history)
2. ✅ RBAC (Role-based access control: admin, operator, viewer)
3. ✅ MFA TOTP (Two-factor authentication)
4. ✅ Health Checks + Downsampling (Metrics optimization)

This guide focuses on frontend implementation.

---

## 1. MFA Implementation (2FA Login)

### Models Needed
```javascript
// frontend/src/stores/auth.js - ADD to store
mfaRequired: false,  // Flag to show TOTP prompt
totp: {
  secret: '',
  qrCode: '',
  backupCodes: [],
  enabled: false
}
```

### Login Flow Update

**Current Flow**:
```
┌─ Username + Password 
└─→ Get JWT Token
```

**New Flow**:
```
┌─ Username + Password
├─→ Check if MFA required (require_mfa: true)
├─→ Show TOTP Code Prompt
└─→ Username + Password + TOTP Code
    └─→ Get JWT Token
```

### Components to Create/Update

#### 1. LoginView.vue - Add TOTP prompt
```vue
<template>
  <!-- Existing login form -->
  <form v-if="!showTOTPPrompt">
    <input v-model="username" placeholder="Username">
    <input v-model="password" type="password" placeholder="Password">
    <button @click="handleLogin">Login</button>
  </form>

  <!-- NEW: TOTP Code Prompt -->
  <form v-else>
    <p>Authenticator app code:</p>
    <input v-model="totpCode" placeholder="000000" maxlength="6">
    <button @click="handleTOTPVerification">Verify</button>
  </form>
</template>

<script>
export default {
  data() {
    return {
      username: '',
      password: '',
      totpCode: '',
      showTOTPPrompt: false
    };
  },
  methods: {
    async handleLogin() {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: this.username,
          password: this.password
        })
      });

      const data = await response.json();
      
      if (data.require_mfa) {
        this.showTOTPPrompt = true;
        return;
      }

      // Normal login success
      this.$store.setToken(data.token);
      this.$router.push('/');
    },

    async handleTOTPVerification() {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: this.username,
          password: this.password,
          totp_code: this.totpCode
        })
      });

      const data = await response.json();
      this.$store.setToken(data.token);
      this.$router.push('/');
    }
  }
};
</script>
```

#### 2. Create UserSettings.vue (or add to existing user menu)

```vue
<template>
  <div class="card">
    <h3>Security Settings</h3>
    
    <!-- MFA Status -->
    <div class="setting">
      <span>Two-Factor Authentication:</span>
      <span v-if="mfaEnabled" class="badge badge-success">ENABLED</span>
      <span v-else class="badge badge-secondary">DISABLED</span>
    </div>

    <!-- Enable MFA Button -->
    <button 
      v-if="!mfaEnabled" 
      @click="showMFASetup = true"
      class="btn btn-primary"
    >
      Enable 2FA
    </button>
    
    <!-- Disable MFA Button -->
    <button 
      v-else 
      @click="disableMFA"
      class="btn btn-danger"
    >
      Disable 2FA
    </button>

    <!-- MFA Setup Modal -->
    <div v-if="showMFASetup" class="modal">
      <div class="modal-content">
        <h4>Set Up Two-Factor Authentication</h4>
        
        <!-- Step 1: Show QR Code -->
        <div v-if="setupStep === 1">
          <p>Scan this QR code with your authenticator app:</p>
          <img :src="totp.qrCode" alt="QR Code">
          <p>Or enter key manually: <code>{{ totp.secret }}</code></p>
          <button @click="setupStep = 2">Next</button>
        </div>

        <!-- Step 2: Enter TOTP Code -->
        <div v-if="setupStep === 2">
          <p>Enter the 6-digit code from your app:</p>
          <input v-model="verifyCode" placeholder="000000" maxlength="6">
          <button @click="verifyMFA">Verify & Enable</button>
        </div>

        <!-- Step 3: Show Backup Codes -->
        <div v-if="setupStep === 3">
          <p>Save these backup codes in a safe place:</p>
          <textarea v-model="backupCodesDisplay" readonly rows="8"></textarea>
          <button @click="downloadBackupCodes">Download</button>
          <button @click="showMFASetup = false">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { useAuthStore } from '@/stores/auth';

export default {
  setup() {
    const authStore = useAuthStore();
    return { authStore };
  },
  data() {
    return {
      mfaEnabled: false,
      showMFASetup: false,
      setupStep: 1, // 1: QR, 2: Verify, 3: Backup codes
      totp: {
        secret: '',
        qrCode: ''
      },
      backupCodes: [],
      verifyCode: ''
    };
  },
  computed: {
    backupCodesDisplay() {
      return this.backupCodes.join('\n');
    }
  },
  methods: {
    async loadMFAStatus() {
      const response = await fetch('/api/v1/auth/mfa/status', {
        headers: { 'Authorization': `Bearer ${this.authStore.token}` }
      });
      const data = await response.json();
      this.mfaEnabled = data.mfa_enabled;
    },

    async initiateMFASetup() {
      const response = await fetch('/api/v1/auth/mfa/setup', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${this.authStore.token}` }
      });
      const data = await response.json();
      this.totp = data;
      this.backupCodes = data.backup_codes;
    },

    async verifyMFA() {
      const response = await fetch('/api/v1/auth/mfa/verify', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${this.authStore.token}` },
        body: JSON.stringify({
          secret: this.totp.secret,
          totp_code: this.verifyCode,
          backup_codes: this.backupCodes
        })
      });

      if (response.ok) {
        this.mfaEnabled = true;
        this.setupStep = 3; // Show backup codes
      }
    },

    async disableMFA() {
      const password = prompt('Enter your password to disable 2FA:');
      if (!password) return;

      const response = await fetch('/api/v1/auth/mfa/disable', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${this.authStore.token}` },
        body: JSON.stringify({ password })
      });

      if (response.ok) {
        this.mfaEnabled = false;
        alert('2FA disabled');
      }
    },

    downloadBackupCodes() {
      const content = this.backupCodesDisplay;
      const element = document.createElement('a');
      element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(content));
      element.setAttribute('download', 'mfa-backup-codes.txt');
      element.click();
    }
  },
  mounted() {
    this.loadMFAStatus();
    this.initiateMFASetup();
  }
};
</script>
```

---

## 2. RBAC Implementation

### Update Role Display

#### Components/UserDropdown.vue
```vue
<template>
  <div class="user-dropdown">
    <img :src="avatarUrl" :alt="username" class="avatar">
    <div class="user-info">
      <div class="username">{{ username }}</div>
      <div class="role">{{ roleLabel }}</div>
    </div>
  </div>
</template>

<script>
export default {
  computed: {
    roleLabel() {
      const roles = {
        'admin': '👑 Admin',
        'operator': '⚙️ Operator',
        'viewer': '👁️ Viewer'
      };
      return roles[this.$store.state.auth.role] || 'Unknown';
    }
  }
};
</script>
```

### Hide/Show UI Based on Role

#### APT Command Button - Hide for viewers
```vue
<button 
  v-if="userRole === 'admin' || userRole === 'operator'"
  @click="showAPTModal = true"
  class="btn btn-warning"
>
  Run APT Command
</button>

<p v-else class="text-muted">
  Only operators and admins can run APT commands
</p>
```

#### Audit Logs Tab - Show only for admin
```vue
<div class="tabs">
  <button @click="activeTab = 'overview'">Overview</button>
  <button @click="activeTab = 'docker'">Docker</button>
  <button @click="activeTab = 'apt'">APT</button>
  <button 
    v-if="userRole === 'admin'"
    @click="activeTab = 'audit'"
  >
    Audit Logs
  </button>
</div>
```

---

## 3. Audit Logs UI

### Create AuditLogsView.vue

```vue
<template>
  <div class="page">
    <h2>Audit Logs</h2>
    
    <div class="filters">
      <input 
        v-model="filters.action" 
        placeholder="Filter by action (apt_update, apt_upgrade, etc)"
      >
      <input 
        v-model="filters.username" 
        placeholder="Filter by user"
      >
      <select v-model="filters.status">
        <option>All Statuses</option>
        <option value="pending">Pending</option>
        <option value="completed">Completed</option>
        <option value="failed">Failed</option>
      </select>
    </div>

    <table class="table">
      <thead>
        <tr>
          <th>Timestamp</th>
          <th>User</th>
          <th>Action</th>
          <th>Host</th>
          <th>IP Address</th>
          <th>Status</th>
          <th>Details</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in auditLogs" :key="log.id">
          <td>{{ formatDate(log.created_at) }}</td>
          <td>{{ log.username }}</td>
          <td><code>{{ log.action }}</code></td>
          <td>{{ log.host_id || '—' }}</td>
          <td>{{ log.ip_address }}</td>
          <td>
            <span :class="`badge badge-${statusClass(log.status)}`">
              {{ log.status }}
            </span>
          </td>
          <td>
            <button @click="showDetails(log)">View</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="pagination">
      <button @click="prevPage" :disabled="page === 1">Previous</button>
      <span>Page {{ page }}</span>
      <button @click="nextPage">Next</button>
    </div>

    <!-- Detail Modal -->
    <div v-if="detailLog" class="modal">
      <div class="modal-content">
        <h3>{{ detailLog.action }} Details</h3>
        <pre>{{ detailLog.details }}</pre>
        <button @click="detailLog = null">Close</button>
      </div>
    </div>
  </div>
</template>

<script>
import dayjs from 'dayjs';

export default {
  data() {
    return {
      auditLogs: [],
      filters: {
        action: '',
        username: '',
        status: 'All Statuses'
      },
      page: 1,
      limit: 50,
      detailLog: null
    };
  },
  methods: {
    async loadAuditLogs() {
      const params = new URLSearchParams({
        page: this.page,
        limit: this.limit
      });

      const response = await fetch(`/api/v1/audit/logs?${params}`, {
        headers: { 'Authorization': `Bearer ${this.$store.state.auth.token}` }
      });
      const data = await response.json();
      this.auditLogs = data.logs || [];
    },

    formatDate(date) {
      return dayjs(date).format('YYYY-MM-DD HH:mm:ss');
    },

    statusClass(status) {
      return {
        'pending': 'warning',
        'completed': 'success',
        'failed': 'danger'
      }[status] || 'secondary';
    },

    showDetails(log) {
      this.detailLog = log;
    },

    nextPage() {
      this.page++;
      this.loadAuditLogs();
    },

    prevPage() {
      if (this.page > 1) {
        this.page--;
        this.loadAuditLogs();
      }
    }
  },
  mounted() {
    this.loadAuditLogs();
  }
};
</script>
```

### Add Audit Logs to Router

```javascript
// frontend/src/router/index.js
const routes = [
  // ... existing routes
  {
    path: '/audit',
    component: () => import('@/views/AuditLogsView.vue'),
    meta: { requiresAdmin: true }
  }
];
```

### Add Audit Tab to HostDetailView

```vue
<template>
  <div class="host-detail">
    <div class="tabs">
      <button @click="activeTab = 'overview'">Overview</button>
      <button @click="activeTab = 'metrics'">Metrics</button>
      <button @click="activeTab = 'docker'">Docker</button>
      <button @click="activeTab = 'apt'">APT</button>

      <!-- NEW: Audit tab for this host -->
      <button @click="activeTab = 'audit'">Audit</button>
    </div>

    <div v-if="activeTab === 'audit'" class="tab-content">
      <h3>Audit History for {{ host.hostname }}</h3>
      <table class="table">
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>User</th>
            <th>Action</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in hostAuditLogs" :key="log.id">
            <td>{{ formatDate(log.created_at) }}</td>
            <td>{{ log.username }}</td>
            <td><code>{{ log.action }}</code></td>
            <td>{{ log.status }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script>
export default {
  methods: {
    async loadHostAuditLogs() {
      const response = await fetch(`/api/v1/audit/logs/host/${this.hostId}`, {
        headers: { 'Authorization': `Bearer ${this.$store.state.auth.token}` }
      });
      this.hostAuditLogs = await response.json();
    }
  }
};
</script>
```

---

## 4. APT Handler Update

### Add triggered_by to APT History

Update AptView.vue to show who ran each command:

```vue
<template>
  <table class="table apt-history">
    <thead>
      <tr>
        <th>Timestamp</th>
        <th>Command</th>
        <th>Status</th>
        <th>Triggered By</th>  <!-- NEW -->
        <th>Duration</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="cmd in aptHistory" :key="cmd.id">
        <td>{{ formatDate(cmd.created_at) }}</td>
        <td><code>apt {{ cmd.command }}</code></td>
        <td>
          <span :class="`badge badge-${statusClass(cmd.status)}`">
            {{ cmd.status }}
          </span>
        </td>
        <td>{{ cmd.triggered_by }}</td>  <!-- NEW -->
        <td v-if="cmd.started_at && cmd.ended_at">
          {{ calculateDuration(cmd.started_at, cmd.ended_at) }}
        </td>
      </tr>
    </tbody>
  </table>
</template>
```

---

## 5. Health Status Display

### Add to Host List View

```vue
<template>
  <div class="host-card">
    <div class="host-status">
      <span 
        :class="{
          'status-online': host.status === 'online',
          'status-offline': host.status === 'offline'
        }"
      >
        {{ host.status === 'online' ? '🟢 Online' : '🔴 Offline' }}
      </span>
      
      <span class="last-seen">
        Last seen: {{ formatRelativeTime(host.last_seen) }}
      </span>
    </div>
  </div>
</template>

<script>
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';

dayjs.extend(relativeTime);

export default {
  methods: {
    formatRelativeTime(date) {
      return dayjs(date).fromNow();
    }
  }
};
</script>
```

---

## 6. API Integration Helper

Create a new file `frontend/src/api/audit.js`:

```javascript
import axios from 'axios';

const API_BASE = '/api/v1';

export default {
  // Audit logs
  getAuditLogs(page = 1, limit = 50) {
    return axios.get(`${API_BASE}/audit/logs?page=${page}&limit=${limit}`);
  },

  getAuditLogsByHost(hostId, limit = 100) {
    return axios.get(`${API_BASE}/audit/logs/host/${hostId}?limit=${limit}`);
  },

  getAuditLogsByUser(username, limit = 100) {
    return axios.get(`${API_BASE}/audit/logs/user/${username}?limit=${limit}`);
  },

  // MFA
  getMFAStatus() {
    return axios.get(`${API_BASE}/auth/mfa/status`);
  },

  setupMFA() {
    return axios.post(`${API_BASE}/auth/mfa/setup`);
  },

  verifyMFA(secret, totpCode, backupCodes) {
    return axios.post(`${API_BASE}/auth/mfa/verify`, {
      secret,
      totp_code: totpCode,
      backup_codes: backupCodes
    });
  },

  disableMFA(password) {
    return axios.post(`${API_BASE}/auth/mfa/disable`, { password });
  }
};
```

---

## Implementation Checklist

### Phase 1: Core MFA (Critical)
- [ ] Update LoginView.vue with TOTP prompt
- [ ] Create UserSettings.vue component
- [ ] Update auth store to track MFA state

### Phase 2: RBAC Display (Important)
- [ ] Add role badge next to username
- [ ] Hide/show APT command buttons based on role
- [ ] Add role selector in user management

### Phase 3: Audit Logs (Important)
- [ ] Create AuditLogsView.vue page
- [ ] Add audit endpoint to API client
- [ ] Integrate audit tab in HostDetailView
- [ ] Show triggered_by in APT history

### Phase 4: Health Status (Nice-to-have)
- [ ] Add last_seen timestamp to host list
- [ ] Display online/offline indicator
- [ ] Add last_seen to host detail view

### Phase 5: Polish (Polish)
- [ ] Add loading states
- [ ] Error handling
- [ ] Mobile responsive design

---

## Testing Checklist

```bash
# Check API is accessible
curl http://localhost:8080/api/health

# Test each endpoint locally before integrating:
curl -X GET http://localhost:8080/api/v1/audit/logs \
  -H "Authorization: Bearer <token>"

curl -X POST http://localhost:8080/api/v1/auth/mfa/setup \
  -H "Authorization: Bearer <token>"
```

---

## Notes for Frontend Dev

1. **TOTP Code Entry**: Only allow 6 digits, auto-focus next field
2. **Backup Codes**: Should be displayed in monospace font
3. **Status Badges**: Use colors consistently (error=red, success=green, warning=yellow)
4. **Pagination**: When showing audit logs with 50+ entries
5. **Error Handling**: Show clear message if TOTP code is invalid

---

## Dependencies Already Installed

- ✅ dayjs (date formatting)
- ✅ axios (HTTP requests)
- ✅ vue-router (routing)
- ✅ pinia (state management)

**No additional frontend packages needed!**

---

Generated: 2026-02-20  
Ready for development 🚀
