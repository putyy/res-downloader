import appApi from "@/api/app"
import {useIndexStore} from "@/stores"
<<<<<<< Updated upstream
import {BrowserOpenURL} from "../../wailsjs/runtime"
=======
import {Browser} from "@wailsio/runtime"
>>>>>>> Stashed changes

export const openDocumentation = async (page: "home" | "filename-template" | "installation") => {
  try {
    const result = await appApi.documentationURL(page, useIndexStore().globalConfig.Locale)
    if (result.code !== 1) throw new Error(result.message)
<<<<<<< Updated upstream
    BrowserOpenURL(result.data.url)
=======
    Browser.OpenURL(result.data.url)
>>>>>>> Stashed changes
  } catch (error) {
    window.$message?.error(String(error))
  }
}
