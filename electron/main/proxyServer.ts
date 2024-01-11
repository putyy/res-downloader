import fs from 'fs'
import log from 'electron-log'
import CONFIG from './const'
import {closeProxy, setProxy} from './setProxy'
import {app} from "electron"
import * as urlTool from "url"
import {toSize} from "./utils"
// @ts-ignore
import {hexMD5} from '../../src/common/md5'

const hoXy = require('hoxy')

const port = 8899

let videoList = {}

if (process.platform === 'win32') {
    process.env.OPENSSL_BIN = CONFIG.OPEN_SSL_BIN_PATH
    process.env.OPENSSL_CONF = CONFIG.OPEN_SSL_CNF_PATH
}

// setTimeout to allow working in macOS
// in windows: H5ExtTransfer:ok
// in macOS: finderH5ExtTransfer:ok

const injection_script1 = `
setTimeout(() => {
  let receiver_url = "https://res-downloader.666666.com";

  function send_response_if_is_video(response) {
    if (response == undefined) return;
    if (!response["err_msg"].includes("H5ExtTransfer:ok")) return;
    let value = JSON.parse(response["jsapi_resp"]["resp_json"]);
    if (value["object"] == undefined || value["object"]["object_desc"] == undefined  || value["object"]["object_desc"]["media"].length == 0) {
      return;
    }
    let media = value["object"]["object_desc"]["media"][0];
    let description = value["object"]["object_desc"]["description"].trim();
    let video_data = {
      "decode_key": media["decode_key"],
      "url": media["url"]+media["url_token"],
      "size": media["file_size"],
      "description":  description,
      "uploader": value["object"]["nickname"]
    };
    fetch(receiver_url, {
      method: "POST",
      mode: "no-cors",
      body: JSON.stringify(video_data),
    }).then((resp) => {
      // alert(\`video data for \${video_data["description"]} sent!\`);
    });
  }

  function wrapper(name,origin) {
    return function() {
      let cmdName = arguments[0];
      if (arguments.length == 3) {
        let original_callback = arguments[2];
        arguments[2] = async function () {
          if (arguments.length == 1) {
            send_response_if_is_video(arguments[0]);
          }
          return await original_callback.apply(this, arguments);
        }
      } else {
      }
      let result = origin.apply(this,arguments);
      return result;
    }
  }

  window.WeixinJSBridge.invoke = wrapper("WeixinJSBridge.invoke", window.WeixinJSBridge.invoke);
  window.wvds = true;
}, 200);`;

export async function startServer({
                                      win,
                                      setProxyErrorCallback = f => f,
                                  }) {
    return new Promise(async (resolve: any, reject) => {
        const proxy = hoXy.createServer({
                certAuthority: {
                    key: fs.readFileSync(CONFIG.CERT_PRIVATE_PATH),
                    cert: fs.readFileSync(CONFIG.CERT_PUBLIC_PATH),
                },
            })
            .listen(port, () => {
                setProxy('127.0.0.1', port)
                    .then(() => {
                        // log.log("--------------setProxy success--------------")
                        resolve()
                    })
                    .catch((err) => {
                        // log.log("--------------setProxy error--------------")
                        // setProxyErrorCallback(data);
                        setProxyErrorCallback({});
                        reject('设置代理失败');
                    });
            })
            .on('error', err => {
                log.log("--------------proxy err--------------", err)
            });


        proxy.intercept(
            {
                phase: 'request',
                hostname: 'res-downloader.666666.com',
                as: 'json',
            },
            (req, res) => {
                // console.log('req.json: ', req.json)
                res.string = 'ok'
                res.statusCode = 200
                let url_sign: string = hexMD5(req.json.url)
                let urlInfo = urlTool.parse(req.json.url, true)
                win?.webContents?.send?.('on_get_queue', {
                    url_sign: url_sign,
                    url: req.json.url,
                    down_url: req.json.url,
                    high_url: '',
                    platform: urlInfo.hostname,
                    size: toSize(req.json.size ?? 0),
                    type: "video/mp4",
                    type_str: 'video',
                    progress_bar: '',
                    save_path: '',
                    downing: false,
                    decode_key: req.json.decode_key,
                    description: req.json.description,
                    uploader: '',
                })
            },
        );

        proxy.intercept(
            {
                phase: 'response',
                hostname: 'channels.weixin.qq.com',
                as: 'string',
            },
            async (req, res) => {
                // console.log('inject[channels.weixin.qq.com] req.url:', req.url);
                if (req.url.includes('/web/pages/feed') || req.url.includes('/web/pages/home')) {
                    res.string = res.string.replace('</body>', '\n<script>' + injection_script1 + '</script>\n</body>');
                    res.statusCode = 200;
                    // console.log('inject[channels.weixin.qq.com]:', req.url, res.string.length);
                }
            },
        );

        proxy.intercept(
            {
                phase: 'response',
            },
            async (req, res) => {
                // 拦截响应
                let ctype = res?._data?.headers?.['content-type']
                let url_sign: string = hexMD5(req.fullUrl())
                let res_url = req.fullUrl()
                let urlInfo = urlTool.parse(res_url, true)
                switch (ctype) {
                    case "video/mp4":
                        if (videoList.hasOwnProperty(url_sign) === false) {
                            videoList[url_sign] = req.fullUrl()
                            let high_url = ''
                            let down_url = res_url
                            // console.log('down_url', down_url)
                            win?.webContents?.send?.('on_get_queue', {
                                url_sign: url_sign,
                                url: down_url,
                                down_url: down_url,
                                high_url: high_url,
                                platform: urlInfo.hostname,
                                size: toSize(res?._data?.headers?.['content-length'] ?? 0),
                                type: ctype,
                                type_str: 'video',
                                progress_bar: '',
                                save_path: '',
                                downing: false,
                                decode_key: '',
                                description: '',
                                uploader: '',
                            })
                        }
                        break;
                    case "image/png":
                    case "image/webp":
                    case "image/svg+xml":
                    case "image/gif":
                        win?.webContents?.send?.('on_get_queue', {
                            url_sign: url_sign,
                            url: res_url,
                            down_url: res_url,
                            high_url: '',
                            platform: urlInfo.hostname,
                            size: toSize(res?._data?.headers?.['content-length'] ?? 0),
                            type: ctype,
                            type_str: 'image',
                            progress_bar: '',
                            save_path: '',
                            downing: false,
                            decode_key: '',
                            description: '',
                            uploader: '',
                        })
                        break;
                    case "audio/mpeg":
                        win?.webContents?.send?.('on_get_queue', {
                            url_sign: url_sign,
                            url: res_url,
                            down_url: res_url,
                            high_url: '',
                            platform: urlInfo.hostname,
                            size: toSize(res?._data?.headers?.['content-length'] ?? 0),
                            type: ctype,
                            type_str: 'audio',
                            progress_bar: '',
                            save_path: '',
                            downing: false,
                            decode_key: '',
                            description: '',
                            uploader: '',
                        })
                        break;
                    case "application/vnd.apple.mpegurl":
                        win.webContents?.send?.('on_get_queue', {
                            url_sign: url_sign,
                            url: res_url,
                            down_url: res_url,
                            high_url: '',
                            platform: urlInfo.hostname,
                            size: toSize(res?._data?.headers?.['content-length'] ?? 0),
                            type: ctype,
                            type_str: 'm3u8',
                            progress_bar: '',
                            save_path: '',
                            downing: false,
                            decode_key: '',
                            description: '',
                            uploader: '',
                        })
                        break;

                }

            },
        )
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
