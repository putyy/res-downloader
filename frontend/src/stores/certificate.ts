import {defineStore} from 'pinia'
import {ref} from 'vue'
import appApi from '@/api/app'
import type {appType} from '@/types/app'

export type CertificateGuideIntent = 'startup' | 'capture' | 'legacyCleanup'

export const useCertificateStore = defineStore('certificate-store', () => {
  const status = ref<appType.CertificateStatus>()
  const loading = ref(false)
  const installing = ref(false)
  const guideVisible = ref(false)
  const guideIntent = ref<CertificateGuideIntent>('startup')

  const refresh = async () => {
    loading.value = true
    try {
      const response = await appApi.certificateStatus() as appType.Res<appType.CertificateStatus>
      if (response.code === 1) status.value = response.data
      return response
    } finally {
      loading.value = false
    }
  }

  const install = async (password = '') => {
    installing.value = true
    try {
      const response = await appApi.installCurrentCertificate({password}) as appType.Res<appType.CertificateStatus>
      if (response.code === 1) status.value = response.data
      return response
    } finally {
      installing.value = false
    }
  }

  const cleanupLegacy = async (password = '') => {
    installing.value = true
    try {
      const response = await appApi.retryCertificateCleanup({password}) as appType.Res<appType.CertificateStatus['migration']>
      if (response.code === 1 && status.value) status.value.migration = response.data
      return response
    } finally {
      installing.value = false
    }
  }

  const showGuide = (intent: CertificateGuideIntent = 'startup') => {
    if (guideVisible.value && guideIntent.value === 'capture' && intent === 'startup') return
    guideIntent.value = intent
    guideVisible.value = true
  }

  const dismissGuide = () => {
    guideVisible.value = false
    guideIntent.value = 'startup'
  }

  return {
    status,
    loading,
    installing,
    guideVisible,
    guideIntent,
    refresh,
    install,
    cleanupLegacy,
    showGuide,
    dismissGuide,
  }
})
