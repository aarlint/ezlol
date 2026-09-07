'use strict'
const { contextBridge, ipcRenderer } = require('electron')
contextBridge.exposeInMainWorld('ezlol', {
  platform: process.platform,
  electron: true,
  notify: (title, body) => ipcRenderer.invoke('notify', String(title), String(body)),
  onUpdate: (fn) => ipcRenderer.on('update-state', (_e, st) => fn(st)),
  installUpdate: () => ipcRenderer.invoke('install-update'),
  checkUpdate: () => ipcRenderer.invoke('check-update'),
})
