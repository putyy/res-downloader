const axios = require('axios')
import CONFIG from './const'

export class Aria2RPC {
    constructor() {
        this.url = `http://127.0.0.1:${CONFIG.ARIA_PORT}/jsonrpc`
        this.id = 1
    }

    call(method, params) {
        const requestData = {
            jsonrpc: "2.0",
            method: method,
            params: params,
            id: this.id++
        };
        return axios.post(this.url, requestData, {
            headers: {
                'Content-Type': 'application/json'
            },
        }).then((response)=>{
            return response.data
        })
    }

    addUri(uri, dir, filename, headers = {}) {
        return this.call('aria2.addUri', [uri, {
            dir: dir,
            out: filename,
            headers: headers,
        }]);
    }

    tellStatus(gid) {
        return this.call('aria2.tellStatus', [gid]);
    }

    calculateDownloadProgress(bitfield) {
        // 将十六进制的 bitfield 转换为二进制字符串
        const totalPieces = bitfield.length * 4; // 每个十六进制字符对应 4 位
        const binaryString = bitfield.split('').map(hex => parseInt(hex, 16).toString(2).padStart(4, '0')).join('');

        // 计算已下载的部分数
        const downloadedPieces = binaryString.split('').filter(bit => bit === '1').length;

        // 计算进度百分比
        const progressPercentage = (downloadedPieces / totalPieces) * 100;

        return progressPercentage.toFixed(2); // 保留两位小数
    }
}