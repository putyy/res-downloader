import appApi from "@/api/app"
import {useIndexStore} from "@/stores"
import {Browser} from "@wailsio/runtime"

export const openDocumentation = async (page: "home" | "filename-template" | "installation") => {
  try {
    const result = await appApi.documentationURL(page, useIndexStore().globalConfig.Locale)
    if (result.code !== 1) throw new Error(result.message)
    Browser.OpenURL(result.data.url)
  } catch (error) {
    window.$message?.error(String(error))
  }
}
