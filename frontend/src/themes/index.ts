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

export interface AppThemeDefinition {
    id: AppThemeName
    nameKey: string
    dark: boolean
    naiveTheme: GlobalTheme
    overrides: GlobalThemeOverrides
    preview: {
        background: string
        surface: string
        accent: string
        muted: string
        text: string
    }
}

const createCommonOverrides = (
    primary: string,
    hover: string,
    pressed: string,
    body: string,
    surface: string,
    surfaceMuted: string,
    surfaceHover: string,
    border: string,
    text: string,
    secondaryText: string,
): GlobalThemeOverrides => ({
    common: {
        primaryColor: primary,
        primaryColorHover: hover,
        primaryColorPressed: pressed,
        primaryColorSuppl: hover,
        bodyColor: body,
        cardColor: surface,
        modalColor: surface,
        popoverColor: surface,
        tableColor: surface,
        inputColor: surface,
        borderColor: border,
        dividerColor: border,
        textColorBase: text,
        textColor1: text,
        textColor2: secondaryText,
    },
    DataTable: {
        borderColor: border,
        thColor: surfaceMuted,
        thColorHover: surfaceHover,
        thColorSorting: surfaceHover,
        thButtonColorHover: surfaceHover,
        thIconColorActive: primary,
        tdColor: surface,
        tdColorHover: surfaceHover,
        tdColorSorting: surfaceHover,
        tdColorStriped: surfaceMuted,
    },
})

export const appThemes: readonly AppThemeDefinition[] = [
    {
        id: 'lightTheme',
        nameKey: 'setting.theme_light_name',
        dark: false,
        naiveTheme: lightTheme,
        overrides: createCommonOverrides(
            '#18a058', '#36ad6a', '#0c7a43', '#ffffff', '#ffffff', '#eef3f0', '#f8faf9', '#e5e7eb', '#1f2937', '#4b5563',
        ),
        preview: {background: '#f5f7f6', surface: '#ffffff', accent: '#18a058', muted: '#dcefe4', text: '#27332c'},
    },
    {
        id: 'darkTheme',
        nameKey: 'setting.theme_dark_name',
        dark: true,
        naiveTheme: darkTheme,
        overrides: createCommonOverrides(
            '#63c58b', '#7bd39d', '#4cad76', '#181a1f', '#202329', '#292d33', '#25292f', '#343840', '#f3f4f6', '#c4c8cf',
        ),
        preview: {background: '#17191d', surface: '#25282e', accent: '#63c58b', muted: '#34443b', text: '#f3f4f6'},
    },
    {
        id: 'sakuraTheme',
        nameKey: 'setting.theme_sakura_name',
        dark: false,
        naiveTheme: lightTheme,
        overrides: createCommonOverrides(
            '#d96c8d', '#e3829e', '#bd526f', '#fff8fa', '#fffdfd', '#fbeef2', '#fff5f8', '#f2dce3', '#4b343c', '#765761',
        ),
        preview: {background: '#fff4f7', surface: '#fffdfd', accent: '#d96c8d', muted: '#f8dce5', text: '#553942'},
    },
    {
        id: 'forestTheme',
        nameKey: 'setting.theme_forest_name',
        dark: false,
        naiveTheme: lightTheme,
        overrides: createCommonOverrides(
            '#557a5b', '#698f6e', '#3f6246', '#f6f8f3', '#fdfefb', '#edf2e9', '#f8fbf5', '#dce4d7', '#29382c', '#566459',
        ),
        preview: {background: '#f1f5ed', surface: '#fdfefb', accent: '#557a5b', muted: '#dce8d7', text: '#304034'},
    },
    {
        id: 'autumnTheme',
        nameKey: 'setting.theme_autumn_name',
        dark: false,
        naiveTheme: lightTheme,
        overrides: createCommonOverrides(
            '#b96d32', '#ca8248', '#965425', '#fffaf3', '#fffefd', '#f7eee2', '#fff9f1', '#eadfce', '#46372c', '#715e4e',
        ),
        preview: {background: '#fbf3e8', surface: '#fffefd', accent: '#b96d32', muted: '#f0dfc6', text: '#493a2f'},
    },
    {
        id: 'seaSaltTheme',
        nameKey: 'setting.theme_sea_salt_name',
        dark: false,
        naiveTheme: lightTheme,
        overrides: createCommonOverrides(
            '#347f7a', '#48938e', '#286964', '#f4faf9', '#fcfefe', '#e9f3f1', '#f7fcfb', '#d4e6e3', '#263e3c', '#526b68',
        ),
        preview: {background: '#edf7f5', surface: '#fcfefe', accent: '#347f7a', muted: '#d2e9e5', text: '#29413f'},
    },
]

const themeMap = new Map<string, AppThemeDefinition>(appThemes.map(theme => [theme.id, theme]))

export const resolveAppTheme = (themeName?: string): AppThemeDefinition => {
    return themeMap.get(themeName ?? '') ?? appThemes[0]
}
