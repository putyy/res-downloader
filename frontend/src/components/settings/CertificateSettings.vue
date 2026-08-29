<template>
  <div class="w-[760px] space-y-4">
    <NAlert type="info" :show-icon="false">{{ t('setting.certificate_device_tip') }}</NAlert>
    <NAlert
        v-if="store.envInfo.platform === 'windows' && status && !status.desktop.installed"
        type="warning"
        :show-icon="false"
    >
      {{ t('certificateGuide.system_authorization_tip') }}
    </NAlert>
    <NCard v-if="status" size="small" :title="t('setting.certificate_desktop_status')">
      <div class="flex items-center gap-2">
        <NTag :type="status.desktop.installed ? 'success' : 'error'">
          {{
            status.desktop.installed ? t('setting.certificate_desktop_installed') : t('setting.certificate_desktop_missing')
          }}
        </NTag>
        <span v-if="status.desktop.error" class="text-xs text-red-500">{{ status.desktop.error }}</span>
      </div>
      <div class="text-sm">{{ t('setting.certificate_fingerprint') }}</div>
      <div class="mt-1 break-all font-mono text-xs text-gray-500">{{
          status.fingerprintSha256 || status.error || '-'
        }}
      </div>
      <div class="mt-4 flex gap-2">
        <NButton type="primary" secondary :loading="installing" @click="beginInstall">
          {{ status.desktop.installed ? t('setting.certificate_reinstall') : t('setting.certificate_install') }}
        </NButton>
        <NButton secondary @click="load">{{ t('setting.certificate_refresh_status') }}</NButton>
        <NPopconfirm @positive-click="beginUninstall">
          <template #trigger>
            <NButton type="error" secondary :disabled="!status.desktop.installed" :loading="uninstalling">
              {{ t('setting.certificate_uninstall') }}
            </NButton>
          </template>
          {{ t('setting.certificate_uninstall_confirm') }}
        </NPopconfirm>
      </div>
    </NCard>
    <NCard v-if="status" size="small" :title="t('setting.certificate_phone_status')">
      <NAlert type="warning" :show-icon="false">{{ t('setting.certificate_phone_tip') }}</NAlert>
      <div class="mt-4">
        <NButton type="primary" secondary @click="BrowserOpenURL(certificateUrl)">{{
            t('setting.certificate_download')
          }}
        </NButton>
      </div>
    </NCard>
    <NCard v-if="status" size="small" :title="t('setting.certificate_legacy_status')">
      <div class="mt-3 flex items-center gap-2">
        <NTag :type="migrationTagType">
          {{ migrationStatusLabel }}
        </NTag>
        <span class="text-xs text-gray-500">{{ status.migration.message }}</span>
      </div>
      <div class="mt-4 flex gap-2">
        <NButton secondary :loading="cleaning" @click="beginCleanup">{{
            t('setting.certificate_cleanup_retry')
          }}
        </NButton>
      </div>
    </NCard>
    <NModal v-model:show="showAuthorization" preset="dialog" :title="authorizationTitle">
      <div class="space-y-3">
        <div class="text-sm text-gray-500">{{ authorizationTip }}</div>
        <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            :placeholder="t('components.password_placeholder')"
            @keyup.enter="submitAuthorization"
        />
      </div>
      <template #action>
        <NButton @click="showAuthorization = false">{{ t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="cleaning || installing || uninstalling" @click="submitAuthorization">
          {{ t('common.submit') }}
        </NButton>
      </template>
    </NModal>
  </div>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {useI18n} from 'vue-i18n'
import {BrowserOpenURL} from '../../../wailsjs/runtime'
import appApi from '@/api/app'
import type {appType} from '@/types/app'
import {useIndexStore} from '@/stores'
import {useCertificateStore} from '@/stores/certificate'

defineProps<{ certificateUrl: string }>()
const {t} = useI18n()
const store = useIndexStore()
const certificate = useCertificateStore()
const status = computed(() => certificate.status)
const cleaning = ref(false)
const installing = computed(() => certificate.installing)
const uninstalling = ref(false)
const showAuthorization = ref(false)
const password = ref('')
const authorizationAction = ref<'install' | 'uninstall' | 'cleanup'>('install')
const authorizationTitle = computed(() => {
  if (authorizationAction.value === 'install') return t('setting.certificate_install_authorize')
  if (authorizationAction.value === 'uninstall') return t('setting.certificate_uninstall_authorize')
  return t('setting.certificate_cleanup_authorize')
})
const authorizationTip = computed(() => {
  if (authorizationAction.value === 'install') return t('setting.certificate_install_authorize_tip')
  if (authorizationAction.value === 'uninstall') return t('setting.certificate_uninstall_authorize_tip')
  return t('setting.certificate_cleanup_authorize_tip')
})
const migrationStatusLabel = computed(() => {
  const value = status.value?.migration.status
  return value ? t(`setting.certificate_migration_${value}`) : '-'
})
const migrationTagType = computed(() => {
  const value = status.value?.migration.status
  if (value === 'removed' || value === 'notFound') return 'success'
  if (value === 'failed' || value === 'needsManualCleanup') return 'error'
  return 'warning'
})
const load = async () => {
  await certificate.refresh()
}
const beginInstall = () => {
  if (['darwin', 'linux'].includes(store.envInfo.platform)) {
    authorizationAction.value = 'install'
    showAuthorization.value = true
    return
  }
  void install()
}
const install = async () => {
  try {
    const response = await certificate.install(password.value)
    if (response.code === 0) {
      window.$message?.error(response.message, {duration: 8000})
      if (store.envInfo.platform === 'windows') {
        window.$message?.warning(t('index.win_install_tip'), {duration: 8000})
      }
      return
    }
    showAuthorization.value = false
    window.$message?.success(t('setting.certificate_install_success'))
  } catch (error: any) {
    window.$message?.error(String(error?.message ?? error), {duration: 8000})
  } finally {
    password.value = ''
  }
}
const beginUninstall = () => {
  if (['darwin', 'linux'].includes(store.envInfo.platform)) {
    authorizationAction.value = 'uninstall'
    showAuthorization.value = true
    return
  }
  void uninstall()
}
const uninstall = async () => {
  uninstalling.value = true
  try {
    const response = await appApi.uninstallCurrentCertificate({password: password.value}) as appType.Res<appType.CertificateStatus>
    if (response.code === 0) {
      window.$message?.error(response.message, {duration: 8000})
      return
    }
    certificate.status = response.data
    showAuthorization.value = false
    window.$message?.success(t('setting.certificate_uninstall_success'))
  } finally {
    password.value = ''
    uninstalling.value = false
  }
}
const beginCleanup = () => {
  const migrationStatus = status.value?.migration.status
  if (['darwin', 'linux'].includes(store.envInfo.platform) && migrationStatus !== 'notFound' && migrationStatus !== 'removed') {
    authorizationAction.value = 'cleanup'
    showAuthorization.value = true
    return
  }
  void cleanup()
}
const cleanup = async () => {
  if (showAuthorization.value && !password.value) {
    window.$message?.error(t('components.password_empty'))
    return
  }
  cleaning.value = true
  try {
    const response = await appApi.retryCertificateCleanup({password: password.value}) as appType.Res<appType.CertificateStatus['migration']>
    if (response.code === 1 && status.value) {
      status.value.migration = response.data
      if (response.data.status === 'removed' || response.data.status === 'notFound') {
        showAuthorization.value = false
        window.$message?.success(t('setting.certificate_cleanup_success'))
      } else if (response.data.message) {
        window.$message?.error(response.data.message, {duration: 8000})
      }
    }
  } finally {
    password.value = ''
    cleaning.value = false
  }
}
const submitAuthorization = () => {
  if (!password.value) {
    window.$message?.error(t('components.password_empty'))
    return
  }
  if (authorizationAction.value === 'install') void install()
  else if (authorizationAction.value === 'uninstall') void uninstall()
  else void cleanup()
}
onMounted(load)
</script>
