const ipv4Regex = /^(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)){3}$/
const hostLabelRegex = /^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$/

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
    if (!host || host.length > 253) return false
    if (ipv4Regex.test(host)) return true
    if (host.includes('[') || host.includes(']')) return false
    if (host.includes(':')) {
        try {
            return new URL(`http://[${host}]/`).hostname.length > 0
        } catch {
            return false
        }
    }
    const labels = host.split('.')
    if (labels.length === 4 && labels.every(label => /^\d+$/.test(label))) return false
    return labels.every(label => hostLabelRegex.test(label))
}

export const isValidPort = (port: string) => {
    if (!/^\d+$/.test(port)) return false
    const portNumber = Number(port)
    return Number.isInteger(portNumber) && portNumber > 1024 && portNumber < 65535
}

export const isValidUpstreamProxy = (value: string) => {
    if (!/^https?:\/\//.test(value)) return false
    try {
        const parsed = new URL(value)
        const authority = value.slice(value.indexOf('://') + 3).split('/')[0]
        const host = parsed.hostname.replace(/^\[|\]$/g, '')
        if (!isValidHost(host) || (parsed.pathname !== '/' && parsed.pathname !== '') ||
            parsed.search || parsed.hash || authority.endsWith(':')) return false
        const rawPort = value.match(/:\d+(?:\/|$)/)?.[0].slice(1).replace(/\/$/, '')
        return !rawPort || Number(rawPort) >= 1 && Number(rawPort) <= 65535
    } catch {
        return false
    }
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
