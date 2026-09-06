'use strict'
const { app, BrowserWindow, shell, Menu, ipcMain, Notification } = require('electron')
const { spawn } = require('node:child_process')
const path = require('node:path')
const fs = require('node:fs')
const http = require('node:http')
let autoUpdater = null
try {
  autoUpdater = require('electron-updater').autoUpdater
} catch {
  autoUpdater = null
}

// Keep Chromium's profile out of the Go backend's data directory.
app.setPath('userData', path.join(app.getPath('appData'), 'ezlol-app'))

const PORT = Number(process.env.EZLOL_PORT || 7331)
const URL = `http://127.0.0.1:${PORT}`
const DEV_UI = process.env.EZLOL_DEV || '' // e.g. http://localhost:5173 to proxy the UI to Vite

let backend = null
let win = null

const BIN = process.platform === 'win32' ? 'ezlol.exe' : 'ezlol'
function backendPath() {
  if (app.isPackaged) return path.join(process.resourcesPath, BIN)
  return path.join(__dirname, '..', 'bin', BIN)
}

// Reuse an already-running ezlol (e.g. started from a terminal) instead of failing on the port.
function ping() {
  return new Promise((resolve) => {
    const req = http.get(`${URL}/api/status`, (res) => {
      res.resume()
      resolve(res.statusCode === 200)
    })
    req.on('error', () => resolve(false))
    req.setTimeout(500, () => {
      req.destroy()
      resolve(false)
    })
  })
}

async function startBackend() {
  if (await ping()) return
  const bin = backendPath()
  if (!fs.existsSync(bin)) throw new Error(`backend not found at ${bin}; run make build`)
  const args = ['-no-open', '-addr', `127.0.0.1:${PORT}`]
  if (DEV_UI) args.push('-dev', DEV_UI)
  const env = { ...process.env }
  if (process.platform === 'darwin') {
    const ocr = app.isPackaged ? path.join(process.resourcesPath, 'ezlol-ocr') : path.join(__dirname, '..', 'bin', 'ezlol-ocr')
    if (fs.existsSync(ocr)) env.EZLOL_OCR = ocr
  }
  backend = spawn(bin, args, { stdio: ['ignore', 'pipe', 'pipe'], env })
  backend.stdout.on('data', (d) => process.stdout.write(d))
  backend.stderr.on('data', (d) => process.stderr.write(d))
  backend.on('exit', (code) => {
    backend = null
    if (!app.isQuitting) console.error(`ezlol backend exited with ${code}`)
  })
  for (let i = 0; i < 100; i++) {
    if (await ping()) return
    await new Promise((r) => setTimeout(r, 100))
  }
  throw new Error('backend did not come up')
}

function createWindow() {
  win = new BrowserWindow({
    width: 1280,
    height: 860,
    minWidth: 900,
    minHeight: 600,
    title: 'ezlol',
    backgroundColor: '#010a13',
    titleBarStyle: 'hiddenInset',
    trafficLightPosition: { x: 14, y: 16 },
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  })
  win.loadURL(URL)
  // Open external links (Riot dev portal etc.) in the system browser, never inside the app.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//.test(url)) shell.openExternal(url)
    return { action: 'deny' }
  })
  win.webContents.on('will-navigate', (e, url) => {
    if (!url.startsWith(URL)) {
      e.preventDefault()
      shell.openExternal(url)
    }
  })
  win.on('closed', () => (win = null))
}

ipcMain.handle('notify', (_e, title, body) => {
  try {
    if (Notification.isSupported()) new Notification({ title: String(title), body: String(body), silent: true }).show()
    if (process.platform === 'darwin' && app.dock) app.dock.bounce('critical')
  } catch (err) {
    console.error('notify failed', err)
  }
  return true
})

// In-place updates: Windows (NSIS) installs on quit; macOS needs a signed app for
// electron-updater, so unsigned mac builds fall back to the download banner in the UI.
function setupUpdates() {
  if (!autoUpdater || !app.isPackaged) return
  autoUpdater.autoDownload = true
  autoUpdater.autoInstallOnAppQuit = true
  const send = (payload) => win && win.webContents.send('update-state', payload)
  autoUpdater.on('checking-for-update', () => send({ status: 'checking' }))
  autoUpdater.on('update-available', (i) => send({ status: 'downloading', version: i.version }))
  autoUpdater.on('update-not-available', () => send({ status: 'none' }))
  autoUpdater.on('update-downloaded', (i) => send({ status: 'downloaded', version: i.version }))
  autoUpdater.on('error', (e) => send({ status: 'error', error: String(e && e.message ? e.message : e) }))
  const check = () => autoUpdater.checkForUpdates().catch(() => {})
  setTimeout(check, 15000)
  setInterval(check, 6 * 60 * 60 * 1000)
}
ipcMain.handle('install-update', () => {
  if (autoUpdater) autoUpdater.quitAndInstall()
  return true
})

app.whenReady().then(async () => {
  Menu.setApplicationMenu(
    Menu.buildFromTemplate([
      { role: 'appMenu' },
      { role: 'editMenu' },
      { role: 'viewMenu' },
      { role: 'windowMenu' },
    ]),
  )
  try {
    await startBackend()
  } catch (err) {
    const { dialog } = require('electron')
    dialog.showErrorBox('ezlol', String(err.message || err))
    app.quit()
    return
  }
  createWindow()
  setupUpdates()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })
})

app.on('before-quit', () => {
  app.isQuitting = true
  if (backend) backend.kill('SIGTERM')
})

app.on('window-all-closed', () => {
  // Keep the watcher alive in the dock so queue pops are still accepted with the window closed.
  if (process.platform !== 'darwin') app.quit()
})
