import fs from 'fs'

const axios = require('axios')

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

export {downloadFile}
