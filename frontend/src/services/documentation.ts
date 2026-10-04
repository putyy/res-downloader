import appApi from "@/api/app"
import {useIndexStore} from "@/stores"
import {BrowserOpenURL} from "../../wailsjs/runtime"

export const openDocumentation = async (page: "home" | "filename-template" | "installation") => {
  try {
    const result = await appApi.documentationURL(page, useIndexStore().globalConfig.Locale)
    if (result.code !== 1) throw new Error(result.message)
    BrowserOpenURL(result.data.url)
  } catch (error) {
    window.$message?.error(String(error))
  }
}
