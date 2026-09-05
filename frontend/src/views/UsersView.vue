<template>
  <div>
    <PageRefreshBar
      v-model="autoRefresh"
      :label="t('account.usersPageLabel')"
      :interval-sec="USERS_REFRESH_SEC"
      :last-updated-at="lastUpdatedAt"
    />
    <div class="page-header d-flex flex-column flex-md-row align-items-md-center justify-content-between gap-3 mb-4">
      <div>
        <div class="page-pretitle">
          <router-link
            to="/"
            class="text-decoration-none"
          >
            {{ t('account.dashboardBreadcrumb') }}
          </router-link>
          <span class="text-muted mx-1">/</span>
          <span>{{ t('account.usersPageLabel') }}</span>
        </div>
        <h2 class="page-title">
          {{ t('account.usersPageLabel') }}
        </h2>
        <div class="text-secondary">
          {{ t('account.rolesManagementSubtitle') }}
        </div>
      </div>
    </div>

    <!-- Create User Form -->
    <div class="card mb-4">
      <div class="card-header">
        <h3 class="card-title">
          {{ t('account.addUserTitle') }}
        </h3>
      </div>
      <div class="card-body">
        <form
          autocomplete="off"
          @submit.prevent="createUser"
        >
          <div class="row g-3">
            <div class="col-md-4">
              <label class="form-label">{{ t('account.usernameLabel') }}</label>
              <input
                v-model="newUserForm.username"
                name="username"
                type="text"
                class="form-control"
                placeholder="john_doe"
                autocomplete="username"
                autocapitalize="none"
                spellcheck="false"
                required
                :disabled="creatingUser"
              >
            </div>
            <div class="col-md-4">
              <label class="form-label">{{ t('account.passwordLabel') }}</label>
              <input
                v-model="newUserForm.password"
                name="new-password"
                type="password"
                class="form-control"
                placeholder="••••••••"
                autocomplete="new-password"
                required
                :disabled="creatingUser"
              >
            </div>
            <div class="col-md-3">
              <label class="form-label">{{ t('account.roleLabel') }}</label>
              <select
                v-model="newUserForm.role"
                class="form-select"
                :disabled="creatingUser"
                name="role"
                autocomplete="off"
              >
                <option value="viewer">
                  viewer
                </option>
                <option value="operator">
                  operator
                </option>
                <option value="admin">
                  admin
                </option>
              </select>
            </div>
            <div class="col-md-1 d-flex align-items-end">
              <button
                type="submit"
                class="btn btn-primary w-100"
                :disabled="creatingUser"
              >
                {{ creatingUser ? t('account.creatingEllipsisLabel') : t('account.addButtonLabel') }}
              </button>
            </div>
          </div>
        </form>
        <div
          v-if="createMessage"
          :class="['alert mt-3 mb-0', createSuccess ? 'alert-success' : 'alert-danger']"
        >
          {{ createMessage }}
        </div>
      </div>
    </div>

    <div
      v-if="actionMessage"
      :class="['alert alert-dismissible mb-3', actionSuccess ? 'alert-success' : 'alert-danger']"
    >
      {{ actionMessage }}
      <a
        class="btn-close"
        data-bs-dismiss="alert"
        aria-label="close"
      />
    </div>

    <!-- Users List -->
    <div class="card">
      <div
        v-if="loading && !users.length"
        class="card-body"
      >
        <LoadingSkeleton
          variant="table"
          :lines="4"
        />
      </div>
      <div
        v-else
        class="table-responsive scroll-table"
      >
        <table class="table table-vcenter card-table">
          <thead>
            <tr>
              <th>{{ t('account.userColumnLabel') }}</th>
              <th>{{ t('account.roleLabel') }}</th>
              <th>{{ t('account.creationColumnLabel') }}</th>
              <th style="width: 200px;" />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="user in users"
              :key="user.id"
            >
              <td class="fw-semibold">
                {{ user.username }}
                <span
                  v-if="user.username === auth.username"
                  class="badge bg-blue-lt text-blue ms-2"
                >{{ t('account.youBadge') }}</span>
              </td>
              <td>
                <select
                  v-model="user.role"
                  class="form-select form-select-sm"
                  :disabled="saving || user.username === auth.username"
                  :title="user.username === auth.username ? t('account.cannotEditOwnRoleTooltip') : ''"
                  @change="saveRole(user)"
                >
                  <option value="viewer">
                    viewer
                  </option>
                  <option value="operator">
                    operator
                  </option>
                  <option value="admin">
                    admin
                  </option>
                </select>
              </td>
              <td class="text-secondary small">
                {{ formatDate(user.created_at) }}
              </td>
              <td class="text-end">
                <button
                  type="button"
                  class="btn btn-icon btn-sm btn-ghost-danger"
                  :disabled="saving || user.username === auth.username || (isLastAdmin(user.id) && user.role === 'admin')"
                  :title="getDeleteButtonTitle(user)"
                  :aria-label="t('account.deleteUserAriaLabel')"
                  @click="deleteUser(user)"
                >
                  <IconTrash
                    :size="14"
                    class="icon"
                  />
                </button>
              </td>
            </tr>
            <tr v-if="!users.length && !loading">
              <td colspan="4">
                <EmptyState :title="t('account.noUsersTitle')" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconTrash } from '@tabler/icons-vue'
import LoadingSkeleton from '../components/LoadingSkeleton.vue'
import PageRefreshBar from '../components/PageRefreshBar.vue'
import EmptyState from '../components/EmptyState.vue'
import { useUsers } from '../composables/useUsers'

const { t } = useI18n()

const {
  auth,
  users,
  loading,
  saving,
  creatingUser,
  newUserForm,
  createMessage,
  createSuccess,
  actionMessage,
  actionSuccess,
  autoRefresh,
  lastUpdatedAt,
  USERS_REFRESH_SEC,
  formatDate,
  isLastAdmin,
  getDeleteButtonTitle,
  createUser,
  saveRole,
  deleteUser,
} = useUsers()
</script>
