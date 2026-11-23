import {darkTheme, lightTheme} from "naive-ui"
import type {GlobalTheme, GlobalThemeOverrides} from "naive-ui"

export const appThemeIds = ["light", "dark", "sakura", "forest", "autumn", "seaSalt"] as const

export type AppThemeName = (typeof appThemeIds)[number]

export const themeColorKeys = ["sidebar", "sidebarText", "sidebarActive", "sidebarAccent", "background", "surface", "heading", "text", "textMuted", "accent"] as const
export type ThemeColorKey = (typeof themeColorKeys)[number]
type BasePalette = Record<ThemeColorKey, string>
export type AppThemeColors = Partial<BasePalette>

interface ThemePalette extends BasePalette {
  surfaceMuted: string
  surfaceHover: string
  border: string
  accentHover: string
  accentPressed: string
  accentSoft: string
}

export interface AppThemeDefinition {
  id: AppThemeName
  nameKey: string
  dark: boolean
  menuSelection: {background: string; text: string}
  naiveTheme: GlobalTheme
  overrides: GlobalThemeOverrides
  cssVars: Record<string, string>
  preview: ThemePalette
}

function channels(color: string): number[] {
  return [1, 3, 5].map((start) => Number.parseInt(color.slice(start, start + 2), 16))
}

function mix(from: string, to: string, amount: number): string {
  const target = channels(to)
  const hex = channels(from).map((value, index) =>
    Math.round(value + (target[index] - value) * amount)
      .toString(16)
      .padStart(2, "0"),
  )
  return "#" + hex.join("")
}

function luminance(color: string): number {
  const [red, green, blue] = channels(color).map((value) => {
    const channel = value / 255
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
  })
  return red * 0.2126 + green * 0.7152 + blue * 0.0722
}

function foreground(background: string): string {
  const light = luminance(background)
  return (light + 0.05) / 0.05 > 1.05 / (light + 0.05) ? "#000000" : "#ffffff"
}

const createTheme = (id: AppThemeName, nameKey: string, base: BasePalette): AppThemeDefinition => {
  const isDark = id === "dark"
  const onAccent = foreground(base.accent)
  const p: ThemePalette = {
    ...base,
    surfaceMuted: mix(base.surface, base.heading, isDark ? 0.05 : 0.035),
    surfaceHover: mix(base.surface, base.heading, isDark ? 0.1 : 0.07),
    border: mix(base.surface, base.heading, isDark ? 0.2 : 0.14),
    accentHover: mix(base.accent, onAccent, 0.09),
    accentPressed: mix(base.accent, onAccent, 0.17),
    accentSoft: mix(base.surface, base.accent, isDark ? 0.2 : 0.12),
  }
  const successColor = isDark ? "#81d2a2" : "#247246"
  const menuSelection = {background: p.sidebarActive, text: p.sidebarAccent}
  return {
    id,
    nameKey,
    dark: isDark,
    menuSelection,
    naiveTheme: isDark ? darkTheme : lightTheme,
    preview: p,
    cssVars: {
      "--app-background": p.background,
      "--app-surface": p.surface,
      "--app-surface-muted": p.surfaceMuted,
      "--app-surface-hover": p.surfaceHover,
      "--app-border": p.border,
      "--app-text-heading": p.heading,
      "--app-text": p.text,
      "--app-text-muted": p.textMuted,
      "--app-accent": p.accent,
      "--app-accent-soft": p.accentSoft,
      "--app-sidebar": p.sidebar,
      "--app-sidebar-text": p.sidebarText,
      "--app-sidebar-accent": p.sidebarAccent,
      "--app-sidebar-active": p.sidebarActive,
    },
    overrides: {
      common: {
        primaryColor: p.accent,
        primaryColorHover: p.accentHover,
        primaryColorPressed: p.accentPressed,
        primaryColorSuppl: p.accentHover,
        successColor,
        successColorHover: isDark ? "#a0e1bc" : "#2e7d4f",
        successColorPressed: isDark ? "#68ba8b" : "#195c38",
        successColorSuppl: successColor,
        bodyColor: p.background,
        cardColor: p.surface,
        modalColor: p.surface,
        popoverColor: p.surface,
        tableColor: p.surface,
        inputColor: p.surface,
        borderColor: p.border,
        dividerColor: p.border,
        textColorBase: p.text,
        textColor1: p.heading,
        textColor2: p.text,
        textColor3: p.textMuted,
        borderRadius: "6px",
      },
      Button: {
        textColorPrimary: onAccent,
        textColorDisabledPrimary: onAccent,
        textColorHoverPrimary: foreground(p.accentHover),
        textColorFocusPrimary: foreground(p.accentHover),
        textColorPressedPrimary: foreground(p.accentPressed),
      },
      Checkbox: {
        checkMarkColor: onAccent,
      },
      Tag: {
        color: p.surfaceMuted,
        textColor: p.textMuted,
      },
      Input: {
        color: p.surfaceMuted,
        colorHover: p.surfaceHover,
        colorFocus: p.surfaceMuted,
        border: "none",
        borderHover: "none",
        borderDisabled: "none",
      },
      Select: {
        peers: {
          InternalSelection: {
            color: `var(--app-select-hover-fill, ${p.surfaceMuted})`,
            colorActive: p.surfaceMuted,
            border: "none",
            borderHover: "none",
          },
        },
      },
      Switch: {
        railColorActive: isDark ? "#36875a" : successColor,
        boxShadowFocus: `0 0 0 2px ${successColor}`,
        ...(isDark
          ? {
              railColor: "#414141",
              buttonColor: "#fafafa",
              buttonBoxShadow: "0 1px 3px rgba(0, 0, 0, 0.24)",
              textColor: "#ffffff",
              iconColor: "#141414",
              loadingColor: "#141414",
              railBorderRadiusSmall: "999px",
              railBorderRadiusMedium: "999px",
              railBorderRadiusLarge: "999px",
              buttonBorderRadiusSmall: "999px",
              buttonBorderRadiusMedium: "999px",
              buttonBorderRadiusLarge: "999px",
            }
          : {}),
      },
      Layout: {
        color: p.background,
        textColor: p.text,
        siderColorInverted: p.sidebar,
        footerColorInverted: p.sidebar,
        textColorInverted: p.sidebarText,
        siderToggleButtonColor: p.sidebarActive,
        siderToggleButtonBorder: "none",
        siderToggleButtonIconColor: p.sidebarText,
        siderToggleButtonIconColorInverted: p.sidebarText,
      },
      Menu: {
        borderRadius: "8px",
        itemHeight: "44px",
        colorInverted: p.sidebar,
        itemColorHoverInverted: mix(p.sidebar, p.sidebarActive, 0.55),
        itemColorActiveInverted: menuSelection.background,
        itemColorActiveHoverInverted: menuSelection.background,
        itemColorActiveCollapsedInverted: menuSelection.background,
        itemTextColorInverted: p.sidebarText,
        itemTextColorHoverInverted: p.sidebarAccent,
        itemTextColorActiveInverted: menuSelection.text,
        itemTextColorActiveHoverInverted: menuSelection.text,
        itemIconColorInverted: p.sidebarText,
        itemIconColorHoverInverted: p.sidebarAccent,
        itemIconColorCollapsedInverted: p.sidebarText,
        itemIconColorActiveInverted: menuSelection.text,
        itemIconColorActiveHoverInverted: menuSelection.text,
      },
      DataTable: {
        borderRadius: "0px",
        borderColor: p.border,
        thColor: p.surfaceMuted,
        thColorHover: p.surfaceHover,
        thColorSorting: p.surfaceHover,
        thButtonColorHover: p.surfaceHover,
        thIconColorActive: p.accent,
        thTextColor: p.textMuted,
        thFontWeight: "500",
        tdColor: p.surface,
        tdColorHover: p.surfaceHover,
        tdColorSorting: p.surfaceHover,
        tdColorStriped: p.surfaceMuted,
      },
    },
  }
}

const presets: Record<AppThemeName, BasePalette> = {
  light: {
    sidebar: "#edeeef",
    sidebarText: "#171717",
    sidebarActive: "#d9dcde",
    sidebarAccent: "#000000",
    background: "#fafafa",
    surface: "#f6f6f6",
    heading: "#242424",
    text: "#50545a",
    textMuted: "#656b72",
    accent: "#246b61",
  },
  dark: {
    sidebar: "#1f1f1f",
    sidebarText: "#b3b3b3",
    sidebarActive: "#3a3a3a",
    sidebarAccent: "#dadada",
    background: "#101010",
    surface: "#1a1a1a",
    heading: "#e2e2e2",
    text: "#b5b8bc",
    textMuted: "#92969c",
    accent: "#84c8b8",
  },
  sakura: {
    sidebar: "#f1e4e9",
    sidebarText: "#624453",
    sidebarActive: "#e4ccd7",
    sidebarAccent: "#57283d",
    background: "#fffafb",
    surface: "#f6edf1",
    heading: "#402634",
    text: "#63505a",
    textMuted: "#73616b",
    accent: "#93425f",
  },
  forest: {
    sidebar: "#e2eae1",
    sidebarText: "#405446",
    sidebarActive: "#ceddcc",
    sidebarAccent: "#243e2b",
    background: "#f9fbf7",
    surface: "#edf2e9",
    heading: "#273d2d",
    text: "#4d5c4e",
    textMuted: "#616e60",
    accent: "#426449",
  },
  autumn: {
    sidebar: "#f1e4d7",
    sidebarText: "#664e3a",
    sidebarActive: "#e4cfba",
    sidebarAccent: "#50351f",
    background: "#fffbf6",
    surface: "#f6eee4",
    heading: "#493423",
    text: "#645448",
    textMuted: "#736557",
    accent: "#88522d",
  },
  seaSalt: {
    sidebar: "#dfeaed",
    sidebarText: "#3c5660",
    sidebarActive: "#c9dbdf",
    sidebarAccent: "#244551",
    background: "#f8fbfc",
    surface: "#eaf1f3",
    heading: "#293e47",
    text: "#4a5c64",
    textMuted: "#5e6e75",
    accent: "#396879",
  },
}

const nameKey = (id: AppThemeName) => `setting.theme_${id === "seaSalt" ? "sea_salt" : id}_name`

export const appThemes: readonly AppThemeDefinition[] = appThemeIds.map((id) => createTheme(id, nameKey(id), presets[id]))

export function resolveAppTheme(id?: string, colors?: AppThemeColors): AppThemeDefinition {
  const preset = appThemes.find((theme) => theme.id === id)
  if (!preset) return appThemes[0]
  if (!colors) return preset

  const base = {...presets[preset.id]}
  for (const key of themeColorKeys) {
    const value = colors[key]
    if (typeof value === "string" && /^#[0-9a-f]{6}$/i.test(value)) base[key] = value.toLowerCase()
  }
  return createTheme(preset.id, preset.nameKey, base)
}
