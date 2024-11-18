import CONFIG from './const'
import {mkdirp} from 'mkdirp'
import fs from 'fs'
import path from 'path'
import {clipboard, dialog} from 'electron'
import spawn from 'cross-spawn'

export function checkCertInstalled() {
    return fs.existsSync(CONFIG.INSTALL_CERT_FLAG)
}

export function installCert(checkInstalled = true) {
    try {
        if (checkInstalled && checkCertInstalled()) {
            return;
        }
        mkdirp.sync(path.dirname(CONFIG.INSTALL_CERT_FLAG))

        if (process.platform === 'darwin') {
            handleMacCertInstallation()
        } else if (process.platform === 'win32') {
            handleWindowsCertInstallation()
        } else {
            handleOtherCertInstallation()
        }
    } catch (e) {
        handleOtherCertInstallation()
    }
}


// MacOS 证书安装处理
function handleMacCertInstallation() {
    clipboard.writeText(
        `echo "输入本地登录密码" && sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "${CONFIG.CERT_PUBLIC_PATH}" && touch ${CONFIG.INSTALL_CERT_FLAG} && echo "安装完成"`
    );

    dialog.showMessageBoxSync({
        type: 'info',
        message: '命令已复制到剪贴板，粘贴到终端并运行以安装并信任证书',
    });
}

// Linux 证书安装处理
function handleOtherCertInstallation() {
    clipboard.writeText(CONFIG.CERT_PUBLIC_PATH);

    dialog.showMessageBoxSync({
        type: "info",
        message: `请手动安装证书，证书文件路径:${CONFIG.CERT_PUBLIC_PATH}  已复制到剪贴板`,
    });
}

// Windows 证书安装处理
function handleWindowsCertInstallation() {
    const result = spawn.sync(CONFIG.WIN_CERT_INSTALL_HELPER, [
        '-c',
        '-add',
        CONFIG.CERT_PUBLIC_PATH,
        '-s',
        'root',
    ]);

    if (result.stdout.toString().includes('Succeeded')) {
        fs.writeFileSync(CONFIG.INSTALL_CERT_FLAG, '');
    } else {
        handleOtherCertInstallation();
    }
}

