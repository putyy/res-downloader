import {app, dialog, shell} from 'electron'
import semver from 'semver'
import fs from 'fs'

const axios = require('axios')

// packageUrl 需要包含 { "version": "1.0.0" } 结构
function checkUpdate(
    // 可以使用加速地址 https://cdn.jsdelivr.net/gh/lecepin/electron-react-tpl/package.json
    packageUrl = 'https://raw.githubusercontent.com/lecepin/electron-react-tpl/master/package.json',
    downloadUrl = 'https://github.com/lecepin/electron-react-tpl/releases',
) {
    axios.get(packageUrl)
        .then(({data}) => {
            if (semver.gt(data?.version, app.getVersion())) {
                const result = dialog.showMessageBoxSync({
                    message: '发现新版本，是否更新？',
                    type: 'question',
                    cancelId: 1,
                    defaultId: 0,
                    buttons: ['进入新版本下载页面', '取消'],
                });

                if (result === 0 && downloadUrl) {
                    shell.openExternal(downloadUrl).then(r => {
                    })
                }
            }
        })
        .catch(err => {
        })
}

function downloadFile(url, fullFileName, progressCallback) {
    return axios.get(url, {
        responseType: 'stream',
        headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36',
        },
    }).then(({data, headers}) => {
        let currentLen = 0
        const totalLen = headers['content-length']

        return new Promise((resolve, reject) => {
            data.on('data', ({length}) => {
                currentLen += length
                // @ts-ignore
                progressCallback?.(currentLen / totalLen)
            });

            data.on('error', err => reject(err))

            data.pipe(
                fs.createWriteStream(fullFileName).on('finish', () => {
                    resolve({
                        fullFileName,
                        totalLen,
                    });
                }),
            );
        });
    });
}

export {checkUpdate, downloadFile}
