import {exec} from 'child_process'
// @ts-ignore
import regedit from 'regedit'
import CONFIG from './const'

regedit.setExternalVBSLocation(CONFIG.REGEDIT_VBS_PATH)

export async function setProxy(host, port) {
    if (process.platform === 'darwin') {
        const networks = await getMacAvailableNetworks()

        // @ts-ignore
        if (networks.length === 0) {
            throw 'no network'
        }

        return Promise.all(
            // @ts-ignore
            networks.map(network => {
                return new Promise((resolve, reject) => {
                    exec(`networksetup -setsecurewebproxy "${network}" ${host} ${port}`, error => {
                        if (error) {
                            reject(null)
                        } else {
                            exec(`networksetup -setwebproxy "${network}" ${host} ${port}`, error => {
                                if (error) {
                                    reject(null)
                                } else {
                                    resolve(network)
                                }
                            });
                        }
                    });
                });
            }),
        );
    } else {
        const valuesToPut = {
            'HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings': {
                ProxyServer: {
                    value: `${host}:${port}`,
                    type: 'REG_SZ',
                },
                ProxyEnable: {
                    value: 1,
                    type: 'REG_DWORD',
                },
            },
        };
        // @ts-ignore
        return regedit.promisified.putValue(valuesToPut)
    }
}

export async function closeProxy() {
    if (process.platform === 'darwin') {
        const networks = await getMacAvailableNetworks()

        // @ts-ignore
        if (networks.length === 0) {
            throw 'no network'
        }

        return Promise.all(
            // @ts-ignore
            networks.map(network => {
                return new Promise((resolve, reject) => {
                    exec(`networksetup -setsecurewebproxystate "${network}" off`, error => {
                        if (error) {
                            reject(null)
                        } else {
                            exec(`networksetup -setwebproxystate "${network}" off`, error => {
                                if (error) {
                                    reject(null)
                                } else {
                                    resolve(network)
                                }
                            });
                        }
                    });
                });
            }),
        );
    } else {
        const valuesToPut = {
            'HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings': {
                ProxyEnable: {
                    value: 0,
                    type: 'REG_DWORD',
                },
            },
        };
        // @ts-ignore
        return regedit.promisified.putValue(valuesToPut)
    }
}

function getMacAvailableNetworks() {
    return new Promise((resolve, reject) => {
        exec('networksetup -listallnetworkservices', (error, stdout) => {
            if (error) {
                reject(error)
            } else {
                Promise.all(
                    stdout
                        .toString()
                        .split('\n')
                        .map(network => {
                            return new Promise(resolve => {
                                exec(
                                    `networksetup getinfo "${network}" | grep "^IP address:\\s\\d"`,
                                    (error, stdout) => {
                                        if (error) {
                                            resolve(null)
                                        } else {
                                            resolve(stdout ? network : null)
                                        }
                                    },
                                )
                            })
                        }),
                ).then(networks => {
                    resolve(networks.filter(Boolean))
                })
            }
        })
    })
}