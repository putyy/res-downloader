import fs from 'fs'
import log from 'electron-log'
import CONFIG from './const'
import {closeProxy, setProxy} from './setProxy'
import {app} from "electron"

const hoXy = require('hoxy')

if (process.platform === 'win32') {
    process.env.OPENSSL_BIN = CONFIG.OPEN_SSL_BIN_PATH
    process.env.OPENSSL_CONF = CONFIG.OPEN_SSL_CNF_PATH
}

export async function startServer({
                                      interceptCallback = f => f => f,
                                      setProxyErrorCallback = f => f,
                                  }) {
    // const port = await getPort()
    const port = 8899

    return new Promise(async (resolve: any, reject) => {
        const proxy = hoXy
            .createServer({
                certAuthority: {
                    key: fs.readFileSync(CONFIG.CERT_PRIVATE_PATH),
                    cert: fs.readFileSync(CONFIG.CERT_PUBLIC_PATH),
                },
            })
            .listen(port, () => {
                setProxy('127.0.0.1', port)
                    .then(() => {
                        log.log("--------------setProxy success--------------")
                        resolve()
                    })
                    .catch((err) => {
                        log.log("--------------setProxy error--------------")
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
            },
            interceptCallback('request'),
        );

        proxy.intercept(
            {
                phase: 'response',
            },
            interceptCallback('response'),
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
