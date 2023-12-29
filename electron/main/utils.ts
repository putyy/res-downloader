import fs from 'fs'
import {Transform } from 'stream'
import {getDecryptionArray} from '../wxjs/decrypt'

const axios = require('axios')
function xorTransform(decryptionArray) {
    let processedBytes = 0;
    return new Transform({
        transform(chunk, encoding, callback) {
            if (processedBytes < decryptionArray.length) {
                let remaining = Math.min(decryptionArray.length - processedBytes, chunk.length);
                for (let i = 0; i < remaining; i++) {
                    chunk[i] = chunk[i] ^ decryptionArray[processedBytes + i];
                }
                processedBytes += remaining;
            }
            this.push(chunk);
            callback();
        }
    });
}

function downloadFile(url, decodeKey, fullFileName, progressCallback) {
    let xorStream = null
    if (decodeKey) {
        xorStream = xorTransform(getDecryptionArray(decodeKey));
    }

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
            if (xorStream) {
                data.pipe(xorStream).pipe(
                    fs.createWriteStream(fullFileName).on('finish', () => {
                        resolve({
                            fullFileName,
                            totalLen,
                        });
                    }),
                );
            }else{
                data.pipe(
                    fs.createWriteStream(fullFileName).on('finish', () => {
                        resolve({
                            fullFileName,
                            totalLen,
                        });
                    }),
                );
            }
        });
    });
}

function toSize(size: number) {
    if (size > 1048576) {
        return (size / 1048576).toFixed(2) + "MB"
    }
    if (size > 1024) {
        return (size / 1024).toFixed(2) + "KB"
    }
    return size + 'b'
}

export {downloadFile, toSize}
