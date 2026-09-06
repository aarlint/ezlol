'use strict'
const { contextBridge, ipcRenderer } = require('electron')
contextBridge.exposeInMainWorld('ezlol', {
  platform: process.platform,
  electron: true,
  setOverlay: (on) => ipcRenderer.invoke('overlay', !!on),
  notify: (title, body) => ipcRenderer.invoke('notify', String(title), String(body)),
  onToggleOverlay: (fn) => ipcRenderer.on('toggle-overlay', () => fn()),
})
