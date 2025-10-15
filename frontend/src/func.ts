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