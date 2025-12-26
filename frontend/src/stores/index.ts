import {defineStore} from 'pinia'
import {ref} from "vue"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {Environment} from "../../wailsjs/runtime"
import * as bind from "../../wailsjs/go/core/Bind"
import {core} from "../../wailsjs/go/models"

export const useIndexStore = defineStore("index-store", () => {
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
        Quality: 0,
        SaveDirectory: "",
        UpstreamProxy: "",
        FilenameLen: 0,
        FilenameTime: false,
        OpenProxy: false,
        DownloadProxy: false,
        AutoProxy: false,
        WxAction: false,
        TaskNumber: 8,
        DownNumber: 3,
        UserAgent: "",
        UseHeaders: "",
        InsertTail: true,
        MimeMap: {},
        Rule: "*"
    })

    const envInfo = ref({
        buildType: "",
        platform: "",
        arch: "",
    });

    const isProxy = ref(false)
    const baseUrl = ref("")

    const init = async () => {
        Environment().then((res) => {
            envInfo.value = res
        })

        await bind.AppInfo().then((res: core.ResponseData)=>{
            appInfo.value = Object.assign({}, appInfo.value, res.data)
            isProxy.value = res.data.IsProxy
        })

        await bind.Config().then((res: core.ResponseData)=>{
            globalConfig.value = Object.assign({}, globalConfig.value, res.data)
        })

        baseUrl.value = "http://127.0.0.1:" +globalConfig.value.Port
        window.$baseUrl = baseUrl.value
    }

    const setConfig = (formValue: Object) => {
        globalConfig.value = Object.assign({}, globalConfig.value, formValue)
        appApi.setConfig(globalConfig.value)
    }

    const openProxy = async () => {
        return appApi.openSystemProxy().then(handleProxy)
    }

    const unsetProxy = async () => {
        return appApi.unsetSystemProxy().then(handleProxy)
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
        init,
        setConfig,
        openProxy,
        unsetProxy
    }
})