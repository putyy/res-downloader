import request from '@/api/request'

export default {
    install() {
        return request({
            url: '/api/install',
            method: 'post'
        })
    },
    setSystemPassword(data: object) {
        return request({
            url: 'api/set-system-password',
            method: 'post',
            data: data
        })
    },
    openSystemProxy() {
        return request({
            url: 'api/proxy-open',
            method: 'post',
        })
    },
    unsetSystemProxy() {
        return request({
            url: 'api/proxy-unset',
            method: 'post',
        })
    },
    openDirectoryDialog() {
        return request({
            url: 'api/open-directory',
            method: 'post'
        })
    },
    openFileDialog() {
        return request({
            url: 'api/open-file',
            method: 'post'
        })
    },
    openFolder(data: object) {
        return request({
            url: 'api/open-folder',
            method: 'post',
            data: data
        })
    },
    isProxy() {
        return request({
            url: 'api/is-proxy',
            method: 'post'
        })
    },
    appInfo() {
        return request({
            url: 'api/app-info',
            method: 'post',
        })
    },
    getConfig() {
        return request({
            url: 'api/get-config',
            method: 'post',
        })
    },
    setConfig(data: object) {
        return request({
            url: 'api/set-config',
            method: 'post',
            data: data
        })
    },
    setType(data: string[]) {
        return request({
            url: 'api/set-type',
            method: 'post',
            data: {
                type: data.toString()
            }
        })
    },
    clear() {
        return request({
            url: 'api/clear',
            method: 'post'
        })
    },
    delete(data: object) {
        return request({
            url: 'api/delete',
            method: 'post',
            data: data
        })
    },
    download(data: object) {
        return request({
            url: 'api/download',
            method: 'post',
            data: data
        })
    },
    wxFileDecode(data: object) {
        return request({
            url: 'api/wx-file-decode',
            method: 'post',
            data: data
        })
    },
    batchImport(data: object) {
        return request({
            url: 'api/batch-import',
            method: 'post',
            data: data
        })
    },
}