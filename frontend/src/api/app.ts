import request from '@/api/request'

const pluginStoreRequestTimeout = 25_000
const pluginInstallRequestTimeout = 75_000

export default {
    openSystemProxy(data: { password?: string } = {}) {
        return request({url: 'api/proxy-open', method: 'post', data,})
    },
    unsetSystemProxy(data: { password?: string } = {}) {
        return request({url: 'api/proxy-unset', method: 'post', data,})
    },
    openDirectoryDialog() {
        return request({url: 'api/open-directory', method: 'post'})
    },
    openFileDialog() {
        return request({url: 'api/open-file', method: 'post'})
    },
    openFolder(data: object) {
        return request({url: 'api/open-folder', method: 'post', data: data})
    },
    isProxy() {
        return request({url: 'api/is-proxy', method: 'post'})
    },
    appInfo() {
        return request({url: 'api/app-info', method: 'post',})
    },
    getConfig() {
        return request({url: 'api/get-config', method: 'post',})
    },
    mediaStatus() {
        return request({url: 'api/media/status', method: 'post'})
    },
    certificateStatus() {
        return request({url: 'api/certificate/status', method: 'post'})
    },
    installCurrentCertificate(data: { password?: string } = {}) {
        return request({url: 'api/certificate/install', method: 'post', data})
    },
    uninstallCurrentCertificate(data: { password?: string } = {}) {
        return request({url: 'api/certificate/uninstall', method: 'post', data})
    },
    retryCertificateCleanup(data: { password?: string } = {}) {
        return request({url: 'api/certificate/cleanup', method: 'post', data})
    },
    setConfig(data: object) {
        return request({url: 'api/set-config', method: 'post', data: data})
    },
    setResourceFilter(data: string[]) {
        return request({url: 'api/resources/filter', method: 'post', data: {types: data}})
    },
    clearResources() {
        return request({url: 'api/resources/clear', method: 'post'})
    },
    deleteResources(data: { ids: string[] }) {
        return request({url: 'api/resources/delete', method: 'post', data: data})
    },
    listResources(data: { offset?: number, limit?: number } = {}) {
        return request({url: 'api/resources', method: 'post', data})
    },
    runResourceAction(data: { id: string, actionId: string }) {
        return request({url: 'api/resources/action', method: 'post', data})
    },
    importResources(data: { items: object[] }) {
        return request({url: 'api/resources/import', method: 'post', data})
    },
    createDownload(data: { id: string }) {
        return request({url: 'api/download/create', method: 'post', data: data})
    },
    downloadTasks() {
        return request({url: 'api/download/tasks', method: 'post'})
    },
    retryDownload(id: string) {
        return request({url: 'api/download/retry', method: 'post', data: {id}})
    },
    pauseDownloadTask(id: string) {
        return request({url: 'api/download/pause', method: 'post', data: {id}})
    },
    resumeDownloadTask(id: string) {
        return request({url: 'api/download/resume', method: 'post', data: {id}})
    },
    cancelDownloadTask(id: string) {
        return request({url: 'api/download/cancel', method: 'post', data: {id}})
    },
    stopRecordingTask(id: string) {
        return request({url: 'api/download/stop-recording', method: 'post', data: {id}})
    },
    deleteDownloadTask(id: string) {
        return request({url: 'api/download/delete', method: 'post', data: {id}})
    },
    batchDownloadTasks(ids: string[], action: 'pause' | 'resume' | 'cancel' | 'retry' | 'delete') {
        return request({url: 'api/download/batch', method: 'post', data: {ids, action}})
    },
    clearDownloadTasks() {
        return request({url: 'api/download/clear', method: 'post'})
    },
    exportResources(data: { content: string }) {
        return request({url: 'api/resources/export', method: 'post', data: data})
    },
    plugins() {
        return request({url: 'api/plugins', method: 'post'})
    },
    pluginStore() {
        return request({url: 'api/plugins/store', method: 'post', timeout: pluginStoreRequestTimeout})
    },
    reloadPlugins() {
        return request({url: 'api/plugins/reload', method: 'post'})
    },
    installPlugin(data: {
        id: string,
        version: string,
        approvePermissions?: boolean
    }) {
        return request({url: 'api/plugins/install', method: 'post', data, timeout: pluginInstallRequestTimeout})
    },
    inspectPluginFile() {
        return request({url: 'api/plugins/file/inspect', method: 'post'})
    },
    installPluginFile(data: { token: string, replace: boolean, approvePermissions?: boolean }) {
        return request({url: 'api/plugins/file/install', method: 'post', data})
    },
    uninstallPlugin(id: string) {
        return request({url: 'api/plugins/uninstall', method: 'post', data: {id}})
    },
    rollbackPlugin(id: string) {
        return request({url: 'api/plugins/rollback', method: 'post', data: {id}})
    },
    enablePlugin(data: { id: string, enabled: boolean }) {
        return request({url: 'api/plugins/enable', method: 'post', data})
    },
    validatePlugin(id: string) {
        return request({url: 'api/plugins/validate', method: 'post', data: {id}})
    },
    setPluginSettings(data: { id: string, settings: { [key: string]: any } }) {
        return request({url: 'api/plugins/settings', method: 'post', data})
    },
}
