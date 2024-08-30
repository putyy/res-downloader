import {app, BrowserWindow, shell, ipcMain, Menu} from 'electron'
import {release} from 'node:os'
import {join} from 'node:path'
import CONFIG from './const'
import initIPC, {setWin} from './ipc'
import {closeProxy} from "./setProxy"
import log from "electron-log"
import path from 'path'
import {spawn} from 'child_process'

// The built directory structure
//
// ├─┬ dist-electron
// │ ├─┬ main
// │ │ └── index.js    > Electron-Main
// │ └─┬ preload
// │   └── index.js    > Preload-Scripts
// ├─┬ dist
// │ └── index.html    > Electron-Renderer
//
process.env.DIST_ELECTRON = join(__dirname, '..')
process.env.DIST = join(process.env.DIST_ELECTRON, '../dist')
process.env.VITE_PUBLIC = process.env.VITE_DEV_SERVER_URL
    ? join(process.env.DIST_ELECTRON, '../public')
    : process.env.DIST

// Disable GPU Acceleration for Windows 7
if (release().startsWith('6.1')) app.disableHardwareAcceleration()

// Set application name for Windows 10+ notifications
if (process.platform === 'win32') app.setAppUserModelId(app.getName())

if (!app.requestSingleInstanceLock()) {
    app.quit()
    process.exit(0)
}

// Remove electron security warnings
// This warning only shows in development mode
// Read more on https://www.electronjs.org/docs/latest/tutorial/security
// process.env['ELECTRON_DISABLE_SECURITY_WARNINGS'] = 'true'

app.commandLine.appendSwitch('--no-proxy-server')
process.on('uncaughtException', () => {
});
process.on('unhandledRejection', () => {
});


let mainWindow: BrowserWindow | null = null
let previewWin: BrowserWindow | null = null
let aria2Process

// Here, you can also use other preload
const preload = join(__dirname, '../preload/index.js')
const url = process.env.VITE_DEV_SERVER_URL
const indexHtml = join(process.env.DIST, 'index.html')

// app.whenReady().then(createWindow)

app.on('window-all-closed', () => {
    mainWindow = null
    if (process.platform !== 'darwin') app.quit()
})

app.on('second-instance', () => {
    if (mainWindow) {
        // Focus on the main window if the user tried to open another
        if (mainWindow.isMinimized()) mainWindow.restore()
        mainWindow.focus()
    }
})

app.on('activate', () => {
    const allWindows = BrowserWindow.getAllWindows()
    if (allWindows.length) {
        allWindows[0].focus()
    } else {
        createWindow()
        createPreviewWindow(mainWindow)
        setWin(mainWindow, previewWin)
    }
})

app.on('before-quit', async e => {
    e.preventDefault()
    try {
        await closeProxy()
        aria2Process && aria2Process.kill();
        log.log("--------------closeProxy success--------------")
    } catch (error) {
        log.log("--------------proxy catch err--------------", error)
    }
    app.exit()
})

function createWindow() {
    Menu.setApplicationMenu(null);

    mainWindow = new BrowserWindow({
        title: 'Main window',
        icon: join(process.env.VITE_PUBLIC, 'favicon.ico'),
        width: 800,
        height: 600,
        webPreferences: {
            preload,
            // Warning: Enable nodeIntegration and disable contextIsolation is not secure in production
            // Consider using contextBridge.exposeInMainWorld
            // Read more on https://www.electronjs.org/docs/latest/tutorial/context-isolation
            nodeIntegration: true,
            contextIsolation: false,
            webSecurity: false,
        },
    })

    if (process.env.VITE_DEV_SERVER_URL) { // electron-vite-vue#298
        mainWindow.loadURL(url).then(r => {
        })
        // Open devTool if the app is not packaged
        // mainWindow.webContents.openDevTools()
    } else {
        mainWindow.loadFile(indexHtml).then(r => {
        })
    }


    CONFIG.IS_DEV && mainWindow.webContents.openDevTools()

    // Test actively push message to the Electron-Renderer
    mainWindow.webContents.on('did-finish-load', () => {
        mainWindow?.webContents.send('main-process-message', new Date().toLocaleString())
    })

    // Make all links open with the browser, not with the application
    mainWindow.webContents.setWindowOpenHandler(({url}) => {
        if (url.startsWith('https:')) shell.openExternal(url)
        return {action: 'deny'}
    })
    // win.webContents.on('will-navigate', (event, url) => { }) #344
}

function createPreviewWindow(parent: BrowserWindow) {
    previewWin = new BrowserWindow({
        parent: parent,
        width: 600,
        height: 400,
        show: false,
        // paintWhenInitiallyHidden: false,
        webPreferences: {
            webSecurity: false,
            nodeIntegration: true,
            contextIsolation: false,
        },
    })

    // previewWin.hide()
    previewWin.setTitle("预览")

    previewWin.on("page-title-updated", (event) => {
        // 阻止该事件
        event.preventDefault()
    })

    previewWin.on("close", (event) => {
        // 不关闭窗口
        event.preventDefault()
        // 影藏窗口
        previewWin.hide()
    })
}

function createArua2Process() {
    // 根据操作系统选择 aria2 的路径
    try {
        let aria2Path, aria2Conf
        if (process.platform === 'win32') {
            // Windows
            aria2Path = path.join(CONFIG.EXECUTABLE_PATH, "./win/aria2/aria2c.exe")
            aria2Conf = path.join(CONFIG.EXECUTABLE_PATH, "./win/aria2/aria2.conf")
        } else {
            aria2Path = path.join(CONFIG.EXECUTABLE_PATH, "./mac/aria2" + (CONFIG.IS_DEV ? `/${process.arch}` : '/') + "/aria2c");
            aria2Conf = path.join(CONFIG.EXECUTABLE_PATH, "./mac/aria2/aria2.conf")
        }
        // 启动 aria2
        console.log("启动 aria2")
        aria2Process = spawn(aria2Path, [`--conf-path=${aria2Conf}`, `--rpc-listen-port=${CONFIG.ARIA_PORT}`], {
            windowsHide: false,
            stdio: CONFIG.IS_DEV ? 'pipe' : 'ignore'
        });
        if(!aria2Process){
            console.log("启动 aria2 失败")
        }
        if (CONFIG.IS_DEV) {
            aria2Process.stdout.on('data', (data) => {
                console.log(`aria2: ${data}`);
            });
            aria2Process.stderr.on('data', (data) => {
                console.log(`aria2 error: ${data}`);
            });
        }
        console.log("aria2 成功启动")
    } catch (e) {
        console.log(`aria2 process start err`, e);
    }
}

app.whenReady().then(() => {
    initIPC()
    createWindow()
    createPreviewWindow(mainWindow)
    createArua2Process()
    setWin(mainWindow, previewWin)
})