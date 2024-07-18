import {ipcMain, dialog, BrowserWindow, app, shell} from 'electron'
import {startServer} from './proxyServer'
import {installCert, checkCertInstalled} from './cert'
import {downloadFile, decodeWxFile, suffix} from './utils'
// @ts-ignore
import {hexMD5} from '../../src/common/md5'
import fs from "fs"
import {floor} from "lodash"

let win: BrowserWindow
let previewWin: BrowserWindow
let isStartProxy = false

export default function initIPC() {

    ipcMain.handle('invoke_app_is_init', async (event, arg) => {
        // 初始化应用 安装证书相关
        return checkCertInstalled()
    })

    ipcMain.handle('invoke_init_app',  (event, arg) => {
        // 开始 初始化应用 安装证书相关
        installCert(false).then(r => {})
    })

    ipcMain.handle('invoke_start_proxy', (event, arg) => {
        // 启动代理服务
        if (isStartProxy) {
            return
        }
        isStartProxy = true
        return startServer({
            win: win,
            upstreamProxy: arg.upstream_proxy ? arg.upstream_proxy : "",
            setProxyErrorCallback: err => {

            },
        })
    })

    ipcMain.handle('invoke_select_down_dir', async (event, arg) => {
        // 选择下载位置
        const result = dialog.showOpenDialogSync({title: '保存', properties: ['openDirectory']})
        if (!result?.[0]) {
            return false
        }

        return result?.[0]
    })

    ipcMain.handle('invoke_select_wx_file', async (event, {index, data}) => {
        // 选择下载位置
        const result = dialog.showOpenDialogSync({title: '保存', properties: ['openFile']})
        if (!result?.[0]) {
            return false
        }
        return decodeWxFile(result?.[0], data.decode_key, result?.[0].replace(".mp4", "_解密.mp4"))
    })

    ipcMain.handle('invoke_file_exists', async (event, {save_path, url, description}) => {
        let fileName = description ? description.replace(/[^a-zA-Z\u4e00-\u9fa5]/g, '') : hexMD5(url);
        let res = fs.existsSync(`${save_path}/${fileName}.mp4`)
        return {is_file: res, fileName: `${save_path}/${fileName}.mp4`}
    })

    ipcMain.handle('invoke_down_file', async (event, {data, save_path}) => {

        let down_url = data.url
        if (!down_url) {
            return false
        }
        let fileName = data?.description ? data.description.replace(/[^a-zA-Z0-9\u4e00-\u9fa5]/g, '') : hexMD5(down_url);
        let save_path_file = `${save_path}/${fileName}` + suffix(data.type)
        if (process.platform === 'win32') {
            save_path_file = `${save_path}\\${fileName}` + suffix(data.type)
        }

        if (fs.existsSync(save_path_file)) {
            return {fullFileName: save_path_file, totalLen: ""}
        }
        // 开始下载
        return downloadFile(
            down_url,
            data.decode_key,
            save_path_file,
            (res) => {
                win?.webContents.send('on_down_file_schedule', {schedule: floor(res * 100)})
            }
        ).catch(err => {
            // console.log("err:", err)
            return false
        })
    });

    ipcMain.handle('invoke_resources_preview', async (event, {url}) => {
        if (!url) {
            return
        }

        previewWin.loadURL(url).then(r => {
            return
        }).catch(res => {
        })
        previewWin.show()
        return
    })

    ipcMain.handle('invoke_open_default_browser', (event, {url}) => {
        shell.openExternal(url).then(r => {})
    })

    ipcMain.handle('invoke_open_file_dir', (event, {save_path}) => {
        shell.showItemInFolder(save_path)
    })

    ipcMain.handle('invoke_file_del', (event, {url_sign}) => {
        if (url_sign === "all"){
            global.videoList = {}
            return
        }
        if (url_sign) {
            delete global.videoList[url_sign]
            return
        }
    })

    ipcMain.handle('invoke_window_restart', (event) => {
       app.relaunch()
       app.exit()
    })
}

export function setWin(w, p) {
    win = w
    previewWin = p
}
