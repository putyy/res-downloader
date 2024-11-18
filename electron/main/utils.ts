import fs from 'fs'
import {Transform} from 'stream'
import {getDecryptionArray} from '../wxjs/decrypt.js'

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
    let config = {
        responseType: 'stream',
        headers: {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.127 Safari/537.36',
        },
    }

    if (url.includes("douyin")) {
        config.headers['Referer'] = url
    }

    return axios.get(url, config).then(({data, headers}) => {
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
            } else {
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

function decodeWxFile(fileName, decodeKey, fullFileName) {
    let xorStream = xorTransform(getDecryptionArray(decodeKey));
    let data = fs.createReadStream(fileName);

    return new Promise((resolve, reject) => {
        data.on('error', err => reject(err));
        data.pipe(xorStream).pipe(
            fs.createWriteStream(fullFileName).on('finish', () => {
                resolve({
                    fullFileName,
                });
            }),
        );
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

function typeSuffix(type: string) {
    switch (type ? type.toLowerCase() : type) {
        case "video/mp4":
        case "video/webm":
        case "video/ogg":
        case "video/x-msvideo":
        case "video/mpeg":
        case "video/quicktime":
        case "video/x-ms-wmv":
        case "video/3gpp":
        case "video/x-matroska":
            return ["video", ".mp4"];
        case "audio/video":
        case "video/x-flv":
            return ["live", ".mp4"];
        case "image/png":
        case "image/webp":
        case "image/jpeg":
        case "image/jpg":
        case "image/gif":
        case "image/avif":
        case "image/bmp":
        case "image/tiff":
        case "image/heic":
        case "image/x-icon":
        case "image/svg+xml":
        case "image/vnd.adobe.photoshop":
            return ["image", ".png"];
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
        case "audio/mp3":
        case "audio/mp4;charset=UTF-8":
            return ["audio", ".mp3"];
        case "application/vnd.apple.mpegurl":
        case "application/x-mpegurl":
            return ["m3u8", ".m3u8"];
        case "application/pdf":
            return ["pdf", ".pdf"];
        case "application/vnd.ms-powerpoint":
        case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
            return ["ppt", ".ppt"];
        case "application/vnd.ms-excel":
        case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
            return ["xls", ".xls"];
        case "application/msword":
        case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
            return ["doc", ".doc"];
    }
    return ["", ""]
}

function getCurrentDateTimeFormatted() {
    const now = new Date();

    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0'); // 月份从0开始，所以要加1
    const day = String(now.getDate()).padStart(2, '0');
    const hours = String(now.getHours()).padStart(2, '0');
    const minutes = String(now.getMinutes()).padStart(2, '0');
    const seconds = String(now.getSeconds()).padStart(2, '0');

    return `${year}${month}${day}${hours}${minutes}${seconds}`;
}

export {downloadFile, toSize, decodeWxFile, typeSuffix, getCurrentDateTimeFormatted}
