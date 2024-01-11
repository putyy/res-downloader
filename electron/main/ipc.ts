import {ipcMain, dialog, BrowserWindow, app, shell} from 'electron'
import {startServer} from './proxyServer'
import {installCert, checkCertInstalled} from './cert'
import {downloadFile, decodeWxFile} from './utils'
// @ts-ignore
import {hexMD5} from '../../src/common/md5'
import fs from "fs"
import CryptoJS from 'crypto-js'
import {closeProxy, setProxy} from "./setProxy"
import log from "electron-log"
import {floor} from "lodash";

let getMac = require("getmac").default
let win: BrowserWindow
let previewWin: BrowserWindow
let isStartProxy = false
let isOpenProxy = false

let aesKey = "as5d45as4d6qe6wqfar6gt4749q6y7w6h34v64tv7t37ty5qwtv6t6qv"

const suffix = (type: string) => {
    switch (type) {
        case "video/mp4":
            return ".mp4";
        case "image/png":
            return ".png";
        case "image/webp":
            return ".webp";
        case "image/svg+xml":
            return ".svg";
        case "image/gif":
            return ".gif";
        case "audio/mpeg":
            return ".mp3";
        case "application/vnd.apple.mpegurl":
            return ".m3u8";
    }
}

export default function initIPC() {

    ipcMain.handle('invoke_app_is_init', async (event, arg) => {
        // 初始化应用 安装证书相关
        return checkCertInstalled()
    })

    ipcMain.handle('invoke_init_app', (event, arg) => {
        // 开始 初始化应用 安装证书相关
        // console.log('invoke_init_app')
        return installCert(false)
    })

    ipcMain.handle('invoke_start_proxy', async (event, arg) => {
        // 启动代理服务
        if (isStartProxy) {
            if (isOpenProxy === false) {
                isOpenProxy = true
                setProxy('127.0.0.1', 8899)
                    .then(() => {
                    })
                    .catch((err) => {
                    })
            }
            return
        }
        isStartProxy = true
        isOpenProxy = true
        return startServer({
            win: win,
            setProxyErrorCallback: err => {
                isStartProxy = false
                isOpenProxy = false
            },
        })
    })

    ipcMain.handle('invoke_close_proxy', (event, arg) => {
        // 关闭代理
        try {
            isOpenProxy = false
            return closeProxy()
        } catch (error) {
            log.log("--------------closeProxy error--------------", error)
        }

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


    ipcMain.handle('invoke_file_exists', async (event, {save_path, url}) => {
        let url_sign = hexMD5(url)
        let res = fs.existsSync(`${save_path}/${url_sign}.mp4`)
        return {is_file: res, fileName: `${save_path}/${url_sign}.mp4`}
    })

    ipcMain.handle('invoke_down_file', async (event, {index, data, save_path, high}) => {
        let down_url = data.down_url
        if (high && data.high_url) {
            down_url = data.high_url
        }

        if (!down_url) {
            return false
        }

        let url_sign = hexMD5(down_url)
        let save_path_file = `${save_path}/${url_sign}` + suffix(data.type)
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
            // console.log('invoke_down_file:err', err)
            return false
        })
    })

    ipcMain.handle('invoke_get_mac', async (event) => {
        let mac = getMac()
        if (mac === "") {
            return ""
        }
        return CryptoJS.AES.encrypt(mac, CryptoJS.enc.Hex.parse(aesKey), {
            mode: CryptoJS.mode.ECB,
            padding: CryptoJS.pad.Pkcs7
        }).ciphertext.toString()
    })

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
}

export function setWin(w, p) {
    win = w
    previewWin = p
}