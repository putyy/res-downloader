<template>
  <NModal
      :show="certificate.guideVisible"
      :mask-closable="false"
      :close-on-esc="false"
      transform-origin="center"
  >
    <NCard
        class="certificate-guide w-[560px] max-w-[calc(100vw-32px)]"
        :bordered="false"
        role="dialog"
        aria-modal="true"
    >
      <div class="flex items-start gap-4">
        <div class="certificate-guide__icon flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl">
          <NIcon size="27"><ShieldCheckmarkOutline/></NIcon>
        </div>
        <div class="min-w-0 flex-1">
          <div class="text-xl font-semibold">{{ guideTitle }}</div>
          <p class="app-muted-text mt-2 text-sm leading-6">{{ guideDescription }}</p>
        </div>
      </div>

      <div class="app-muted-surface mt-5 rounded-2xl p-4">
        <div class="flex gap-3">
          <NIcon class="mt-0.5 shrink-0 text-amber-600" size="19"><LockClosedOutline/></NIcon>
          <div class="text-sm leading-6">
            <div class="font-medium">{{ authorizationTitle }}</div>
            <div class="app-muted-text mt-1">{{ authorizationDescription }}</div>
          </div>
        </div>
      </div>

      <NAlert v-if="certificate.status?.error" type="error" :show-icon="false" class="mt-4">
        {{ certificate.status.error }}
      </NAlert>

      <div v-if="needsPassword" class="mt-4">
        <NInput
            v-model:value="password"
            type="password"
            show-password-on="click"
            :disabled="certificate.installing"
            :placeholder="t('components.password_placeholder')"
            @keyup.enter="submit"
        />
        <div class="app-muted-text mt-2 text-xs">{{ t('certificateGuide.password_tip') }}</div>
      </div>

      <div class="mt-6 flex items-center justify-between gap-3">
        <NButton quaternary :disabled="certificate.installing" @click="dismiss">
          {{ t('certificateGuide.later') }}
        </NButton>
        <NButton type="primary" :loading="certificate.installing" @click="submit">
          {{ primaryLabel }}
        </NButton>
      </div>
    </NCard>
  </NModal>
</template>

<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {LockClosedOutline, ShieldCheckmarkOutline} from '@vicons/ionicons5'
import {useI18n} from 'vue-i18n'
import {useCertificateStore} from '@/stores/certificate'
import {useIndexStore} from '@/stores'

const {t} = useI18n()
const app = useIndexStore()
const certificate = useCertificateStore()
const password = ref('')
const legacyCleanup = computed(() => certificate.guideIntent === 'legacyCleanup')
const needsPassword = computed(() => ['darwin', 'linux'].includes(app.envInfo.platform))
const guideTitle = computed(() => legacyCleanup.value
  ? t('certificateGuide.cleanup_title')
  : t('certificateGuide.title'))
const guideDescription = computed(() => legacyCleanup.value
  ? t('certificateGuide.cleanup_description')
  : t('certificateGuide.description'))
const authorizationTitle = computed(() => {
  if (legacyCleanup.value) return t('setting.certificate_cleanup_authorize')
  return needsPassword.value
    ? t('certificateGuide.password_authorization')
    : t('certificateGuide.system_authorization')
})
const authorizationDescription = computed(() => {
  if (legacyCleanup.value) {
    return app.envInfo.platform === 'windows'
      ? t('certificateGuide.cleanup_windows_tip')
      : t('setting.certificate_cleanup_authorize_tip')
  }
  return app.envInfo.platform === 'windows'
    ? t('certificateGuide.system_authorization_tip')
    : t('certificateGuide.password_authorization_tip')
})
const primaryLabel = computed(() => {
  if (legacyCleanup.value) return t('certificateGuide.cleanup')
  return certificate.guideIntent === 'capture'
    ? t('certificateGuide.install_and_capture')
    : t('certificateGuide.install')
})

const dismiss = () => {
  password.value = ''
  certificate.dismissGuide()
}

const submit = async () => {
  if (needsPassword.value && !password.value) {
    window.$message?.error(t('components.password_empty'))
    return
  }
  try {
    if (legacyCleanup.value) {
      const response = await certificate.cleanupLegacy(password.value)
      const migration = response.data
      if (response.code !== 1 || !['removed', 'notFound'].includes(migration?.status)) {
        window.$message?.error(migration?.message || response.message || t('certificateGuide.cleanup_failed'), {duration: 8000})
        return
      }
      window.$message?.success(t('setting.certificate_cleanup_success'))
      certificate.dismissGuide()
      if (!certificate.status?.desktop?.installed) certificate.showGuide('startup')
      return
    }
    const response = await certificate.install(password.value)
    if (response.code !== 1 || !response.data?.desktop?.installed) {
      window.$message?.error(response.message || t('certificateGuide.install_failed'), {duration: 8000})
      if (app.envInfo.platform === 'windows') {
        window.$message?.warning(t('index.win_install_tip'), {duration: 8000})
      }
      return
    }
    window.$message?.success(t('setting.certificate_install_success'))
    const shouldStartCapture = certificate.guideIntent === 'capture'
    certificate.dismissGuide()
    if (shouldStartCapture) {
      await app.openProxy(password.value)
    }
  } catch (error: any) {
    window.$message?.error(String(error?.message ?? error), {duration: 8000})
  } finally {
    password.value = ''
  }
}

onMounted(async () => {
  try {
    const response = await certificate.refresh()
    if (response.code !== 1) return
    const migrationStatus = response.data.migration?.status
    if (['authorizationRequired', 'needsManualCleanup', 'failed'].includes(migrationStatus)) {
      certificate.showGuide('legacyCleanup')
    } else if (!response.data.desktop.installed) {
      certificate.showGuide('startup')
    }
  } catch (error: any) {
    window.$message?.error(String(error?.message ?? error))
  }
})
</script>

<style scoped>
.certificate-guide.n-card {
  --n-color: var(--app-surface) !important;
  background-color: var(--app-surface) !important;
  box-shadow: 0 24px 64px rgba(25, 38, 30, 0.18);
}

.certificate-guide__icon {
  color: var(--app-accent);
  background-color: var(--app-accent-soft);
}
</style>
