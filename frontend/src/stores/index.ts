import {defineStore} from 'pinia'
import {ref} from "vue"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {Environment} from "../../wailsjs/runtime"

export const useIndexStore = defineStore("index-store", () => {
    const appInfo = ref<appType.App>({
        AppName: "",
        Version: "",
        Description: "",
        Copyright: "",
    })

    const globalConfig = ref<appType.Config>({
        Theme: "lightTheme",
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
        UserAgent: "",
        UseHeaders: "",
    })

    const envInfo = ref({
        buildType: "",
        platform: "",
        arch: "",
    });

    const tableHeight = ref(800)

    const isProxy = ref(false)

    const init = async () => {
        Environment().then((res) => {
            envInfo.value = res
        })
        await getAppInfo()
        await appApi.getConfig().then((res) => {
            globalConfig.value = Object.assign({}, globalConfig.value, res.data)
        })
        setTimeout(() => {
            appApi.isProxy().then((res: any) => {
                isProxy.value = res.data.isProxy
            })
        }, 150)
        window.addEventListener("resize", handleResize);
        handleResize()
    }

    const getAppInfo = async () => {
        await appApi.appInfo().then((res) => {
            appInfo.value = Object.assign({}, appInfo.value, res.data)
        })
    }

    const setConfig = (formValue: appType.Config) => {
        globalConfig.value = Object.assign({}, globalConfig.value, formValue)
        appApi.setConfig(globalConfig.value)
    }

    const handleResize = () => {
        tableHeight.value = document.documentElement.clientHeight || window.innerHeight
    }

    const updateProxyStatus = (res: any) => {
        isProxy.value = res.isProxy
    }

    return {appInfo, globalConfig, tableHeight, isProxy, envInfo, init, getAppInfo, setConfig, updateProxyStatus}
})