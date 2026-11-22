<template>
  <footer class="footer footer-transparent d-print-none">
    <div class="container-xl">
      <div class="row text-center align-items-center flex-row-reverse g-2">
        <div class="col-lg-auto ms-lg-auto">
          <ul class="list-inline list-inline-dots mb-0">
            <li class="list-inline-item">
              <a
                href="https://github.com/Rem7474/ServerSupervisor"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >
                <IconBrandGithub
                  :size="16"
                  class="icon me-1"
                />
                GitHub
              </a>
            </li>
            <li class="list-inline-item">
              <a
                href="https://github.com/Rem7474/ServerSupervisor/releases"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >
                v{{ appVersion }}
              </a>
            </li>
          </ul>
        </div>
        <div class="col-12 col-lg-auto mt-0">
          <p class="mb-0">
            <span class="text-muted">
              Copyright &copy; {{ year }}
              <a
                href="https://github.com/Rem7474"
                target="_blank"
                rel="noopener noreferrer"
                class="link-secondary"
              >ServerSupervisor</a>.
              {{ t('common.footerAllRightsReserved') }}
            </span>
          </p>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { IconBrandGithub } from '@tabler/icons-vue'

const { t } = useI18n()

declare const __APP_VERSION__: string
const appVersion = __APP_VERSION__
const year = new Date().getFullYear()
</script>
