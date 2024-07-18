import fs from 'fs'
import log from 'electron-log'
import CONFIG from './const'
import {closeProxy, setProxy} from './setProxy'
import {app} from "electron"
import * as urlTool from "url"
import {toSize} from "./utils"
// @ts-ignore
import {hexMD5} from '../../src/common/md5'
import pkg from '../../package.json'

const hoXy = require('hoxy')

const port = 8899

global.videoList = {}

if (process.platform === 'win32') {
    process.env.OPENSSL_BIN = CONFIG.OPEN_SSL_BIN_PATH
    process.env.OPENSSL_CONF = CONFIG.OPEN_SSL_CNF_PATH
}

const resObject = {
    url: "",
    url_sign: "",
    platform: "",
    size: "",
    type: "video/mp4",
    type_str: 'video',
    progress_bar: "",
    save_path: "",
    decode_key: "",
    description: ""
}

const vv = hexMD5(pkg.version) + (CONFIG.IS_DEV ? Math.random() :"")

export async function startServer({win, upstreamProxy, setProxyErrorCallback = f => f,}) {
    return new Promise(async (resolve: any, reject) => {
        try {
            const proxy = hoXy.createServer({
                upstreamProxy: upstreamProxy,
                certAuthority: {
                    key: fs.readFileSync(CONFIG.CERT_PRIVATE_PATH),
                    cert: fs.readFileSync(CONFIG.CERT_PUBLIC_PATH),
                },
            })
                .listen(port, () => {
                    setProxy('127.0.0.1', port)
                        .then((res) => {
                            resolve()
                        })
                        .catch((err) => {
                            setProxyErrorCallback(err)
                            reject('setting proxy err: ' + err.toString())
                        });
                })
                .on('error', err => {
                    setProxyErrorCallback(err)
                    reject('proxy service err: ' + err.toString())
                })


            proxy.intercept(
                {
                    phase: 'request',
                    hostname: 'res-downloader.666666.com',
                    as: 'json',
                },
                (req, res) => {
                    res.string = 'ok'
                    res.statusCode = 200
                    try {
                        if (!req.json?.description || req.json?.media?.length <= 0) {
                            return
                        }
                        const media = req.json?.media[0]
                        const url_sign: string = hexMD5(media.url)
                        if (global.videoList.hasOwnProperty(url_sign) === true) {
                            return
                        }
                        const urlInfo = urlTool.parse(media.url, true)
                        global.videoList[url_sign] = media.url
                        win.webContents.send('on_get_queue', Object.assign({}, resObject, {
                            url_sign: url_sign,
                            url: media.url + media.urlToken,
                            platform: urlInfo.hostname,
                            size: media?.fileSize ? toSize(media.fileSize) : 0,
                            type: "video/mp4",
                            type_str: 'video',
                            decode_key: media?.decodeKey ? media?.decodeKey : '',
                            description: req.json.description,
                        }))
                    } catch (e) {
                        log.log(e.toString())
                    }
                },
            )

            proxy.intercept(
                {
                    phase: 'response',
                    hostname: 'channels.weixin.qq.com',
                    as: 'string',
                },
                async (req, res) => {
                    if (req.url.includes('/web/pages/feed') || req.url.includes('/web/pages/home')) {
                        res.string = res.string.replaceAll('.js"', '.js?v=' + vv + '"')
                        res.statusCode = 200
                    }
                },
            )

            proxy.intercept(
                {
                    phase: 'response',
                    as: 'string',
                },
                async (req, res) => {
                    if (req.url.endsWith('.js?v=' + vv)) {
                        res.string = res.string.replaceAll('.js"', '.js?v=' + vv + '"');
                    }
                    if (req.url.includes("web/web-finder/res/js/virtual_svg-icons-register.publish")) {
                        // console.log(res.string.match(/return\s*\{\s*width:([\s\S]*?)scalingInfo:([\s\S]*?)\}/))
//                         res.string = res.string.replace(
//                             /return\s*{\s*width:(.*?)scalingInfo:(.*?)\s*}/,
//                             `var mediaInfo = {width:$1scalingInfo:$2};
// console.log("mediaInfo", mediaInfo);
// console.log("this.objectDesc", this.objectDesc);
// return mediaInfo;`
//                         )
                        res.string = res.string.replace(/get\s*media\s*\(\)\s*\{/, `
                        get media(){
                            if(this.objectDesc){
                                fetch("https://res-downloader.666666.com", {
                                  method: "POST",
                                  mode: "no-cors",
                                  body: JSON.stringify(this.objectDesc),
                                });
                            };
                        `)
                    }
                }
            );

            proxy.intercept(
                {
                    phase: 'response',
                },
                async (req, res) => {
                    try {
                        // 拦截响应
                        const ctype = res?._data?.headers?.['content-type']
                        const url_sign: string = hexMD5(req.fullUrl())
                        const res_url = req.fullUrl()
                        const urlInfo = urlTool.parse(res_url, true)
                        switch (ctype) {
                            case "video/mp4":
                            case "video/webm":
                            case "video/ogg":
                            case "video/x-msvideo":
                            case "video/mpeg":
                            case "video/quicktime":
                            case "video/x-ms-wmv":
                            case "video/x-flv":
                            case "video/3gpp":
                            case "video/x-matroska":
                                if (global.videoList.hasOwnProperty(url_sign) === false) {
                                    global.videoList[url_sign] = res_url
                                    win.webContents.send('on_get_queue', Object.assign({}, resObject, {
                                        url: res_url,
                                        url_sign: url_sign,
                                        platform: urlInfo.hostname,
                                        size: toSize(res?._data?.headers?.['content-length'] ?? 0),
                                        type: ctype,
                                        type_str: 'video',
                                    }))
                                }
                                break;
                            case "image/png":
                            case "image/webp":
                            case "image/jpeg":
                            case "image/jpg":
                            case "image/svg+xml":
                            case "image/gif":
                            case "image/avif":
                            case "image/bmp":
                            case "image/tiff":
                            case "image/x-icon":
                            case "image/heic":
                            case "image/vnd.adobe.photoshop":
                                win.webContents.send('on_get_queue', Object.assign({}, resObject, {
                                    url: res_url,
                                    url_sign: url_sign,
                                    platform: urlInfo.hostname,
                                    size: res?._data?.headers?.['content-length'] ? toSize(res?._data?.headers?.['content-length']) : 0,
                                    type: ctype,
                                    type_str: 'image',
                                }))
                                break
                            case "audio/mpeg":
                            case "audio/wav":
                            case "audio/aiff":
                            case "audio/x-aiff":
                            case "audio/aac":
                            case "audio/ogg":
                            case "audio/flac":
                            case "audio/midi":
                            case "audio/x-midi":
                            case "audio/x-ms-wma":
                            case "audio/opus":
                            case "audio/webm":
                            case "audio/mp4":
                                win.webContents.send('on_get_queue', Object.assign({}, resObject, {
                                    url: res_url,
                                    url_sign: url_sign,
                                    platform: urlInfo.hostname,
                                    size: res?._data?.headers?.['content-length'] ? toSize(res?._data?.headers?.['content-length']) : 0,
                                    type: ctype,
                                    type_str: 'audio',
                                }))
                                break
                            case "application/vnd.apple.mpegurl":
                            case "application/x-mpegURL":
                                win.webContents.send('on_get_queue', Object.assign({}, resObject, {
                                    url: res_url,
                                    url_sign: url_sign,
                                    platform: urlInfo.hostname,
                                    size: res?._data?.headers?.['content-length'] ? toSize(res?._data?.headers?.['content-length']) : 0,
                                    type: ctype,
                                    type_str: 'm3u8',
                                }))
                                break

                        }
                    } catch (e) {
                        log.log(e.toString())
                    }
                },
            )
        } catch (e) {
            log.log("--------------proxy catch err--------------", e)
        }
    })
}

app.on('before-quit', async e => {
    e.preventDefault()
    try {
        await closeProxy()
        log.log("--------------closeProxy success--------------")
    } catch (error) {
    }
    app.exit()
})
