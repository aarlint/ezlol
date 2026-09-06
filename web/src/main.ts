import { createApp } from 'vue'
import App from './App.vue'
import { autoscroll } from './autoscroll'
import './style.css'

if ((window as unknown as { ezlol?: { electron?: boolean } }).ezlol?.electron || /Electron/.test(navigator.userAgent)) document.body.classList.add('electron')
createApp(App).directive('autoscroll', autoscroll).mount('#app')
