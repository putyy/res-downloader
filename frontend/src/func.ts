const ipv4Regex = /^(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)){3}$/
const domainRegex = /^(?!:\/\/)([a-zA-Z0-9-_]+\.)*[a-zA-Z0-9][a-zA-Z0-9-_]+\.[a-zA-Z]{2,11}?$/
const localhostRegex = /^localhost$/

export const compareVersions = (v1: string, v2: string) => {
    const parts1 = v1.split('.').map(Number)
    const parts2 = v2.split('.').map(Number)

    const maxLength = Math.max(parts1.length, parts2.length)

    for (let i = 0; i < maxLength; i++) {
        const num1 = parts1[i] || 0
        const num2 = parts2[i] || 0

        if (num1 < num2) return -1
        if (num1 > num2) return 1
    }

    return 0
}

export const isValidHost = (host: string) => {
    return ipv4Regex.test(host) || domainRegex.test(host) || localhostRegex.test(host)
}

export const isValidPort = (port: number) => {
    const portNumber = Number(port)
    return Number.isInteger(portNumber) && portNumber > 1024 && portNumber < 65535
}

export const formatSize = (size: number | string) => {
    if (typeof size === "string") return size
    if (size > 1048576) {
        return (size / 1048576).toFixed(2) + 'MB';
    }
    if (size > 1024) {
        return (size / 1024).toFixed(2) + 'KB';
    }
    return Math.floor(size) + 'b';
}

type ResourceSourceCandidate = {
    Url?: string
    Domain?: string
    OtherData?: Record<string, string>
}

const hostnameOf = (value: string) => {
    const candidate = value.trim()
    if (!candidate) return ''

    try {
        const url = new URL(candidate.includes('://') ? candidate : `https://${candidate}`)
        return url.hostname.toLowerCase().replace(/\.$/, '')
    } catch {
        return ''
    }
}

const isFinderVideoHostname = (hostname: string) => {
    return /^finder[a-z0-9-]*\.video\.qq\.com$/.test(hostname)
}

// 新资源由后端显式标记；URL 判断用于兼容升级前已保存在 localStorage 的视频号资源。
// 这个函数只决定是否展示入口，评论目标身份仍由后端完整 feed 三元组验证。
export const isWechatChannelResource = (row?: ResourceSourceCandidate | null) => {
    if (!row) return false
    if (row.OtherData?.wx_channel === '1') return true

    const rawUrl = String(row.Url || '').trim()
    if (!rawUrl) return false

    // WeChat 4.1.11 的资源主机已出现 findera4.video.qq.com，且旧版后端会把
    // URL 端口保留在 Domain（qq.com:443）。同时兼容这两种已观测格式。
    const domain = hostnameOf(String(row.Domain || ''))
    if (domain && domain !== 'qq.com') return false

    return isFinderVideoHostname(hostnameOf(rawUrl))
}
