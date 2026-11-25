/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent
  export default component
}

declare module 'topojson-client'

// noVNC ships plain ES modules without type declarations; only the RFB
// surface ProxmoxVncConsole.vue uses is described here.
declare module '@novnc/novnc' {
  export interface RfbOptions {
    shared?: boolean
    credentials?: { username?: string; password?: string; target?: string }
    wsProtocols?: string[]
  }
  export default class RFB extends EventTarget {
    constructor(target: HTMLElement, urlOrChannel: string | WebSocket, options?: RfbOptions)
    scaleViewport: boolean
    resizeSession: boolean
    clipViewport: boolean
    viewOnly: boolean
    focusOnClick: boolean
    background: string
    disconnect(): void
    sendCtrlAltDel(): void
    focus(): void
  }
}
