'use strict'
const { contextBridge, ipcRenderer } = require('electron')
contextBridge.exposeInMainWorld('ezlol', {
  platform: process.platform,
  electron: true,
  notify: (title, body) => ipcRenderer.invoke('notify', String(title), String(body)),
  onUpdate: (fn) => ipcRenderer.on('update-state', (_e, st) => fn(st)),
  installUpdate: () => ipcRenderer.invoke('install-update'),
  checkUpdate: () => ipcRenderer.invoke('check-update'),
  // Desktop settings (login item, tray) live in the main process.
  getAppSettings: () => ipcRenderer.invoke('app-settings:get'),
  setAppSettings: (patch) => ipcRenderer.invoke('app-settings:set', patch && typeof patch === 'object' ? patch : {}),
  onAppSettings: (fn) => ipcRenderer.on('app-settings', (_e, s) => fn(s)),
})
