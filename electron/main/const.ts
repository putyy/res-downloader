import path from 'path'
import isDev from 'electron-is-dev'
import os from 'os'
import {app} from 'electron'

const APP_PATH = app.getAppPath();
// 对于一些 shell 去执行的文件，asar 目录下无法使用。配合 extraResources
const EXECUTABLE_PATH = path.join(
    APP_PATH.indexOf('app.asar') > -1
        ? APP_PATH.substring(0, APP_PATH.indexOf('app.asar'))
        : APP_PATH,
    'electron/res',
)

const HOME_PATH = path.join(os.homedir(), '.res-downloader@putyy')

export default {
    IS_DEV: isDev,
    EXECUTABLE_PATH,
    HOME_PATH,
    CERT_PRIVATE_PATH: path.join(EXECUTABLE_PATH, './keys/private.pem'),
    CERT_PUBLIC_PATH: path.join(EXECUTABLE_PATH, './keys/public.pem'),
    INSTALL_CERT_FLAG: path.join(HOME_PATH, './installed.lock'),
    WIN_CERT_INSTALL_HELPER: path.join(EXECUTABLE_PATH, './w_c.exe'),
    APP_CN_NAME: '爱享素材下载器',
    APP_EN_NAME: 'ResDownloader',
    REGEDIT_VBS_PATH: path.join(EXECUTABLE_PATH, './regedit-vbs'),
    OPEN_SSL_BIN_PATH: path.join(EXECUTABLE_PATH, './openssl/openssl.exe'),
    OPEN_SSL_CNF_PATH: path.join(EXECUTABLE_PATH, './openssl/openssl.cnf'),
};
