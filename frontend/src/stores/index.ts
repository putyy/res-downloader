import {defineStore} from 'pinia'
import {ref} from "vue"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {Environment} from "../../wailsjs/runtime"
import * as bind from "../../wailsjs/go/app/Bind"
import {httpapi} from "../../wailsjs/go/models"
import {frontendErrorDetails, reportFrontendError} from '@/services/diagnostics'

export type StartupState = 'loading' | 'ready' | 'failed'

export const useIndexStore = defineStore("index-store", () => {
	let configSaveTimer: ReturnType<typeof setTimeout> | undefined
	let configSaveChain: Promise<void> = Promise.resolve()
    const appInfo = ref<appType.App>({
        AppName: "",
        Version: "",
        Description: "",
        Copyright: "",
    })

    const globalConfig = ref<appType.Config>({
        Theme: "lightTheme",
        Locale: "zh",
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

    const init = async () => {
		startupState.value = 'loading'
		startupError.value = ''
		try {
			envInfo.value = await Environment()

			const session = await bind.APISession() as httpapi.ResponseData
			if (session.code !== 1) throw new Error(session.message || 'API session initialization failed')
			window.$apiToken = String((session.data as { token?: string })?.token ?? '')
			if (!window.$apiToken) throw new Error('API session token is empty')

			const info = await bind.AppInfo() as httpapi.ResponseData
			if (info.code !== 1) throw new Error(info.message || 'Application information initialization failed')
			appInfo.value = Object.assign({}, appInfo.value, info.data)
			isProxy.value = info.data.IsProxy

			const configuration = await bind.Config() as httpapi.ResponseData
			if (configuration.code !== 1) throw new Error(configuration.message || 'Configuration initialization failed')
			globalConfig.value = Object.assign({}, globalConfig.value, configuration.data)

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

    const setConfig = (formValue: Object) => {
        globalConfig.value = Object.assign({}, globalConfig.value, formValue)
		const snapshot = JSON.parse(JSON.stringify(globalConfig.value)) as appType.Config
		if (configSaveTimer) clearTimeout(configSaveTimer)
		configSaveTimer = setTimeout(() => {
			configSaveTimer = undefined
			configSaveChain = configSaveChain
				.then(async () => {
					const response = await appApi.setConfig(snapshot) as appType.Res
					if (response.code !== 1) throw new Error(response.message || 'save config failed')
				})
				.catch(error => {
					window.$message?.error(String(error?.message ?? error))
				})
		}, 500)
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
        isProxy,
        envInfo,
        baseUrl,
        startupState,
        startupError,
        init,
        setConfig,
        openProxy,
        unsetProxy
    }
})
