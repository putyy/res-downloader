import type {appType} from "@/types/app"

export const storePluginVersion = (extension: appType.PluginStoreEntry) => extension.release?.tag.replace(/^v/, "") ?? ""

export const storePluginOwner = (extension: appType.PluginStoreEntry) => extension.repository.split("/")[0]

export const localizedStoreEntry = (extension: appType.PluginStoreEntry, locale: string) => {
  const entries = extension.locales ?? {}
  return entries[locale] ?? entries[locale.split("-")[0]] ?? entries.en ?? Object.values(entries)[0] ?? {}
}
