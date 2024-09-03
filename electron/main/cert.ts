import CONFIG from './const'
import {mkdirp} from 'mkdirp'
import fs from 'fs'
import path from 'path'
import {clipboard, dialog} from 'electron'
import spawn from 'cross-spawn'

export function checkCertInstalled() {
    return fs.existsSync(CONFIG.INSTALL_CERT_FLAG)
}

export async function installCert(checkInstalled = true) {
    if (checkInstalled && checkCertInstalled()) {
        return;
    }
    mkdirp.sync(path.dirname(CONFIG.INSTALL_CERT_FLAG))

    if (process.platform === 'darwin') {
        return new Promise((resolve, reject) => {
            clipboard.writeText(
                `echo "输入本地登录密码" && sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "${CONFIG.CERT_PUBLIC_PATH}" &&  touch ${CONFIG.INSTALL_CERT_FLAG} && echo "安装完成"`,
            )
            dialog.showMessageBoxSync({
                type: "info",
                message: `命令已复制到剪贴板,粘贴命令到终端并运行以安装并信任证书`,
            });

            reject()
        });
    } else if (process.platform === 'linux') {
        return new Promise((resolve, reject) => {
            clipboard.writeText(
                "https://github.com/putyy/res-downloader/blob/master/electron/res/keys/public.pem",
            )
            dialog.showMessageBoxSync({
                type: "info",
                message: `Linux系统请手动安装证书，已复制下载地址`,
            });

            reject()
        });
    } else {
        return new Promise((resolve: any, reject) => {
            const result = spawn.sync(CONFIG.WIN_CERT_INSTALL_HELPER, [
                '-c',
                '-add',
                CONFIG.CERT_PUBLIC_PATH,
                '-s',
                'root',
            ]);
            if (result.stdout.toString().indexOf('Succeeded') > -1) {
                fs.writeFileSync(CONFIG.INSTALL_CERT_FLAG, '')
                resolve()
            } else {
                reject()
            }
        })
    }
}
