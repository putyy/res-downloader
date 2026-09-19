import {defineStore} from 'pinia'
import {ref} from "vue"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {Environment} from "../../wailsjs/runtime"
import * as bind from "../../wailsjs/go/app/Bind"
import {httpapi} from "../../wailsjs/go/models"
import {frontendErrorDetails, reportFrontendError} from '@/services/diagnostics'

export type StartupState = 'loading' | 'ready' | 'failed'
type ConfigField = 'SaveDirectory' | 'FilenameTemplate' | 'Host' | 'Port' | 'UpstreamProxy' | 'FFmpegPath' | 'FFprobePath'

class ConfigSubmissionError extends Error {
    constructor(message: string, readonly field?: ConfigField) {
        super(message)
    }
}

type ConfigSaveError = {field?: ConfigField; message: string; submittedValue?: string}

export const useIndexStore = defineStore("index-store", () => {
	let configSaveTimer: ReturnType<typeof setTimeout> | undefined
	let configSaveChain: Promise<boolean> = Promise.resolve(true)
	let pendingConfig: appType.Config | undefined
	let pendingRevision = 0
	let configRevision = 0
	let savedConfig: appType.Config | undefined
	let configLoaded = false
    const configSaveError = ref<ConfigSaveError | null>(null)
    const cloneConfig = (value: appType.Config): appType.Config => JSON.parse(JSON.stringify(value))
    const appInfo = ref<appType.App>({
        AppName: "",
        Version: "",
        Description: "",
        Copyright: "",
    })

    const globalConfig = ref<appType.Config>({
        Theme: "lightTheme",
        Locale: "en",
        Host: "0.0.0.0",
        Port: "8899",
        SaveDirectory: "",
        UpstreamProxy: "",
        FilenameTemplate: "{{title|default:resource|sanitize|truncate:80}}_{{date:20060102_150405}}.{{ext}}",
        FilenameConflict: "rename",
        OpenProxy: false,
        DownloadProxy: false,
        FFmpegPath: "",
        FFprobePath: "",
        AutoProxy: false,
        TaskNumber: 8,
        DownNumber: 3,
        UserAgent: "",
        UseHeaders: "",
        InsertTail: true,
        InterceptionPolicies: [{
            id: 'default', name: 'Default', enabled: true, domains: ['*'], exclude: [], action: 'mitm'
        }]
    })

    const envInfo = ref({
        buildType: "",
        platform: "",
        arch: "",
    });

    const isProxy = ref(false)
    const baseUrl = ref("")
    const startupState = ref<StartupState>('loading')
    const startupError = ref("")

    const loadConfig = async () => {
        let timeout: ReturnType<typeof setTimeout> | undefined
        try {
            const configuration = await Promise.race([
                bind.Config(),
                new Promise<never>((_, reject) => {
                    timeout = setTimeout(() => reject(new Error('Configuration initialization timed out')), 5000)
                }),
            ]) as httpapi.ResponseData
            if (configuration.code !== 1) throw new Error(configuration.message || 'Configuration initialization failed')
            globalConfig.value = Object.assign({}, globalConfig.value, configuration.data)
            savedConfig = cloneConfig(globalConfig.value)
            configLoaded = true
        } finally {
            if (timeout !== undefined) clearTimeout(timeout)
        }
    }

    const init = async () => {
		startupState.value = 'loading'
		startupError.value = ''
		try {
			if (!configLoaded) await loadConfig()
			envInfo.value = await Environment()

			const session = await bind.APISession() as httpapi.ResponseData
			if (session.code !== 1) throw new Error(session.message || 'API session initialization failed')
			window.$apiToken = String((session.data as { token?: string })?.token ?? '')
			if (!window.$apiToken) throw new Error('API session token is empty')

			const info = await bind.AppInfo() as httpapi.ResponseData
			if (info.code !== 1) throw new Error(info.message || 'Application information initialization failed')
			appInfo.value = Object.assign({}, appInfo.value, info.data)
			isProxy.value = info.data.IsProxy

			baseUrl.value = "http://127.0.0.1:" + globalConfig.value.Port
			window.$baseUrl = baseUrl.value
			const health = await appApi.appInfo() as appType.Res
			if (health.code !== 1) throw new Error(health.message || 'Local service health check failed')
			startupState.value = 'ready'
		} catch (error) {
			startupError.value = frontendErrorDetails(error)
			startupState.value = 'failed'
			void reportFrontendError('startup', error)
		}
    }

    const savePendingConfig = () => {
        if (configSaveTimer !== undefined) clearTimeout(configSaveTimer)
        configSaveTimer = undefined
        if (!pendingConfig) return
        const snapshot = pendingConfig
        const revision = pendingRevision
        pendingConfig = undefined
        configSaveChain = configSaveChain
            .then(async () => {
                const response = await appApi.setConfig(snapshot) as appType.Res
                if (response.code !== 1) {
                    const field = (response.data as {field?: string} | null)?.field
                    throw new ConfigSubmissionError(response.message || 'save config failed',
                        field === 'SaveDirectory' || field === 'FilenameTemplate' || field === 'Host' ||
                        field === 'Port' || field === 'UpstreamProxy' || field === 'FFmpegPath' ||
                        field === 'FFprobePath' ? field : undefined)
                }
                savedConfig = cloneConfig(snapshot)
                if (revision === configRevision && configSaveError.value) {
                    const previousError = configSaveError.value
                    if (!previousError.field || snapshot[previousError.field] === previousError.submittedValue) {
                        configSaveError.value = null
                    }
                }
                return true
            })
            .catch(error => {
                const message = String(error?.message ?? error)
                if (revision === configRevision) {
                    if (savedConfig) globalConfig.value = cloneConfig(savedConfig)
                    const field = error instanceof ConfigSubmissionError ? error.field : undefined
                    configSaveError.value = {field, message, submittedValue: field ? snapshot[field] : undefined}
                }
                window.$message?.error(message)
                return false
            })
    }

    const setConfig = (formValue: Partial<appType.Config>) => {
        const previousError = configSaveError.value
        // Partial updates (such as changing the theme) must not dismiss an
        // error for a field that is still invalid in the settings form.
        if (previousError?.field && Object.prototype.hasOwnProperty.call(formValue, previousError.field) &&
            formValue[previousError.field] !== previousError.submittedValue) {
            configSaveError.value = null
        }
        globalConfig.value = Object.assign({}, globalConfig.value, formValue)
		pendingConfig = cloneConfig(globalConfig.value)
		pendingRevision = ++configRevision
        if (configSaveTimer !== undefined) clearTimeout(configSaveTimer)
        configSaveTimer = setTimeout(savePendingConfig, 500)
    }

    const flushConfig = async (): Promise<boolean> => {
        // Include edits queued while an earlier save is still in flight.
        while (true) {
            savePendingConfig()
            const saving = configSaveChain
            const saved = await saving
            if (saving === configSaveChain && !pendingConfig) return saved
        }
    }

    const openProxy = async (password = '') => {
        return appApi.openSystemProxy({password}).then(handleProxy)
    }

    const unsetProxy = async (password = '') => {
        return appApi.unsetSystemProxy({password}).then(handleProxy)
    }

    const handleProxy = (res: appType.Res) => {
        isProxy.value = res.data.value
        if (res.code === 0) {
            window?.$message?.error(res.message)
        }
        return res
    }

    return {
        appInfo,
        globalConfig,
        configSaveError,
        isProxy,
        envInfo,
        baseUrl,
        startupState,
        startupError,
        init,
        loadConfig,
        setConfig,
        flushConfig,
        openProxy,
        unsetProxy
    }
})
