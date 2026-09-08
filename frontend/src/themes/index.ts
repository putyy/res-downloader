import {darkTheme, lightTheme} from 'naive-ui'
import type {GlobalTheme, GlobalThemeOverrides} from 'naive-ui'

export const appThemeIds = [
    'lightTheme',
    'darkTheme',
    'sakuraTheme',
    'forestTheme',
    'autumnTheme',
    'seaSaltTheme',
] as const

export type AppThemeName = typeof appThemeIds[number]

interface ThemePalette {
    background: string
    surface: string
    surfaceMuted: string
    surfaceHover: string
    border: string
    text: string
    textMuted: string
    accent: string
    accentHover: string
    accentPressed: string
    accentSoft: string
    sidebar: string
    sidebarText: string
    sidebarAccent: string
    sidebarActive: string
}

export interface AppThemeDefinition {
    id: AppThemeName
    nameKey: string
    dark: boolean
    menuInset: boolean
    menuSelection: {background: string, text: string, shadow: string}
    naiveTheme: GlobalTheme
    overrides: GlobalThemeOverrides
    cssVars: Record<string, string>
    preview: ThemePalette
}

// Navigation has its own inverted colors, independent of the workspace theme.
// Keep existing theme IDs so saved preferences continue to resolve.
const createTheme = (id: AppThemeName, nameKey: string, p: ThemePalette): AppThemeDefinition => {
    const isDark = id === 'darkTheme'
    const subtleSelection = id === 'darkTheme' || id === 'sakuraTheme'
    const menuHoverText = isDark ? p.text : '#ffffff'
    const menuSelection = {
        background: subtleSelection ? 'rgba(255, 255, 255, 0.12)' : p.sidebarActive,
        text: subtleSelection ? menuHoverText : p.sidebarAccent,
        shadow: subtleSelection ? 'none' : `inset 2px 0 0 ${p.sidebarAccent}`,
    }
    return {
        id,
        nameKey,
        dark: isDark,
        menuInset: subtleSelection || id === 'lightTheme' || id === 'seaSaltTheme',
        menuSelection,
        naiveTheme: isDark ? darkTheme : lightTheme,
        preview: p,
        cssVars: {
            '--app-background': p.background,
            '--app-surface': p.surface,
            '--app-surface-muted': p.surfaceMuted,
            '--app-surface-hover': p.surfaceHover,
            '--app-border': p.border,
            '--app-text': p.text,
            '--app-text-muted': p.textMuted,
            '--app-accent': p.accent,
            '--app-accent-soft': p.accentSoft,
            '--app-sidebar': p.sidebar,
            '--app-sidebar-text': p.sidebarText,
            '--app-sidebar-accent': p.sidebarAccent,
            '--app-sidebar-active': p.sidebarActive,
            '--app-menu-selection-shadow': menuSelection.shadow,
        },
        overrides: {
            common: {
                primaryColor: p.accent,
                primaryColorHover: p.accentHover,
                primaryColorPressed: p.accentPressed,
                primaryColorSuppl: p.accentHover,
                bodyColor: p.background,
                cardColor: p.surface,
                modalColor: p.surface,
                popoverColor: p.surface,
                tableColor: p.surface,
                inputColor: p.surface,
                borderColor: p.border,
                dividerColor: p.border,
                textColorBase: p.text,
                textColor1: p.text,
                textColor2: p.textMuted,
                textColor3: p.textMuted,
                borderRadius: '6px',
            },
            ...(isDark ? {
                Switch: {
                    railColor: '#44453e',
                    railColorActive: '#477354',
                    buttonColor: '#f5f0e6',
                    buttonBoxShadow: '0 1px 3px rgba(0, 0, 0, 0.24)',
                    textColor: '#f5f0e6',
                    iconColor: '#477354',
                    loadingColor: '#477354',
                    boxShadowFocus: '0 0 0 2px rgba(143, 198, 164, 0.3)',
                    railBorderRadiusSmall: '999px',
                    railBorderRadiusMedium: '999px',
                    railBorderRadiusLarge: '999px',
                    buttonBorderRadiusSmall: '999px',
                    buttonBorderRadiusMedium: '999px',
                    buttonBorderRadiusLarge: '999px',
                },
            } : {}),
            Layout: {
                color: p.background,
                textColor: p.text,
                siderColorInverted: p.sidebar,
                footerColorInverted: p.sidebar,
                textColorInverted: p.sidebarText,
                siderToggleButtonColor: p.sidebarActive,
                siderToggleButtonBorder: '1px solid rgba(255, 255, 255, 0.14)',
                siderToggleButtonIconColor: p.sidebarText,
                siderToggleButtonIconColorInverted: p.sidebarText,
            },
            Menu: {
                borderRadius: '8px',
                itemHeight: '44px',
                colorInverted: p.sidebar,
                itemColorHoverInverted: 'rgba(255, 255, 255, 0.06)',
                itemColorActiveInverted: menuSelection.background,
                itemColorActiveHoverInverted: menuSelection.background,
                itemColorActiveCollapsedInverted: menuSelection.background,
                itemTextColorInverted: p.sidebarText,
                itemTextColorHoverInverted: menuHoverText,
                itemTextColorActiveInverted: menuSelection.text,
                itemTextColorActiveHoverInverted: menuSelection.text,
                itemIconColorInverted: p.sidebarText,
                itemIconColorHoverInverted: menuHoverText,
                itemIconColorCollapsedInverted: p.sidebarText,
                itemIconColorActiveInverted: menuSelection.text,
                itemIconColorActiveHoverInverted: menuSelection.text,
            },
            DataTable: {
                borderRadius: '0px',
                borderColor: p.border,
                thColor: p.surfaceMuted,
                thColorHover: p.surfaceHover,
                thColorSorting: p.surfaceHover,
                thButtonColorHover: p.surfaceHover,
                thIconColorActive: p.accent,
                thTextColor: p.textMuted,
                thFontWeight: '500',
                tdColor: p.surface,
                tdColorHover: p.surfaceHover,
                tdColorSorting: p.surfaceHover,
                tdColorStriped: p.surfaceMuted,
            },
        },
    }
}

export const appThemes: readonly AppThemeDefinition[] = [
    createTheme('lightTheme', 'setting.theme_light_name', {
        background: '#f3f5f4', surface: '#ffffff', surfaceMuted: '#edf1ef', surfaceHover: '#f5f8f6',
        border: '#dfe5e1', text: '#27332e', textMuted: '#616e67',
        accent: '#357458', accentHover: '#408164', accentPressed: '#285c44', accentSoft: '#e5efe9',
        sidebar: '#252d2a', sidebarText: '#b9c5be', sidebarAccent: '#c2e6d1', sidebarActive: '#354b40',
    }),
    createTheme('darkTheme', 'setting.theme_dark_name', {
        background: '#1b1c19', surface: '#242521', surfaceMuted: '#2d2e29', surfaceHover: '#32342e',
        border: '#3d3f37', text: '#eee9df', textMuted: '#b9b2a5',
        accent: '#8fc6a4', accentHover: '#a4d6b6', accentPressed: '#79b48e', accentSoft: '#2a3b31',
        sidebar: '#11120f', sidebarText: '#beb8ac', sidebarAccent: '#eee9df', sidebarActive: '#2d3028',
    }),
    createTheme('sakuraTheme', 'setting.theme_sakura_name', {
        background: '#fcf4f7', surface: '#fffdfd', surfaceMuted: '#f5e9ef', surfaceHover: '#fcf0f5',
        border: '#e9dde2', text: '#46353d', textMuted: '#78626d',
        accent: '#9e4869', accentHover: '#aa5475', accentPressed: '#873d5a', accentSoft: '#f3e3eb',
        sidebar: '#382830', sidebarText: '#cfbbc5', sidebarAccent: '#f2c4d6', sidebarActive: '#583b49',
    }),
    createTheme('forestTheme', 'setting.theme_forest_name', {
        background: '#f3f5f0', surface: '#fdfefa', surfaceMuted: '#eaf0e6', surfaceHover: '#f3f7ef',
        border: '#dde4d6', text: '#303c2e', textMuted: '#616e5c',
        accent: '#526d3e', accentHover: '#5f7a4b', accentPressed: '#415b31', accentSoft: '#e5eddc',
        sidebar: '#243128', sidebarText: '#bdcbb8', sidebarAccent: '#cde2b5', sidebarActive: '#3b4e37',
    }),
    createTheme('autumnTheme', 'setting.theme_autumn_name', {
        background: '#f8f5ef', surface: '#fffefa', surfaceMuted: '#f1eadf', surfaceHover: '#faf5ec',
        border: '#e7dfd2', text: '#44382d', textMuted: '#766655',
        accent: '#905725', accentHover: '#9d6431', accentPressed: '#7d4b20', accentSoft: '#f1e5d5',
        sidebar: '#352b24', sidebarText: '#cec1b2', sidebarAccent: '#efc99b', sidebarActive: '#544232',
    }),
    createTheme('seaSaltTheme', 'setting.theme_sea_salt_name', {
        background: '#f1f6f5', surface: '#fcfefd', surfaceMuted: '#e7efed', surfaceHover: '#f0f7f5',
        border: '#d9e4e1', text: '#2c3e3b', textMuted: '#5b6f6a',
        accent: '#2e7269', accentHover: '#397d74', accentPressed: '#275f58', accentSoft: '#dfefea',
        sidebar: '#233536', sidebarText: '#b5ccca', sidebarAccent: '#b9e4d9', sidebarActive: '#354f4e',
    }),
]

const themeMap = new Map<string, AppThemeDefinition>(appThemes.map(theme => [theme.id, theme]))

export const resolveAppTheme = (themeName?: string): AppThemeDefinition => {
    return themeMap.get(themeName ?? '') ?? appThemes[0]
}
