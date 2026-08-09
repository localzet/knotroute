import type { Snapshot, NetworkInput, CAInput } from './types.js';

declare global {
  interface Window {
    go?: { main?: { App?: Record<string, (...args: unknown[]) => Promise<unknown>> } };
    runtime?: { EventsOn?: (name:string, cb:(...args:unknown[])=>void)=>()=>void };
  }
}
const api = () => window.go?.main?.App;
async function call<T>(name:string, ...args:unknown[]):Promise<T> {
  const fn = api()?.[name];
  if (!fn) throw new Error(`Desktop bridge ${name} is unavailable`);
  return await fn(...args) as T;
}
export const bridge = {
  snapshot: () => call<Snapshot>('GetSnapshot'),
  toggleNetwork: () => call<Snapshot>('ToggleNetwork'),
  browserIntegration: (v:boolean) => call<Snapshot>('SetBrowserIntegration', v),
  saveProfile: (name:string,bio:string) => call<Snapshot>('SaveProfile',name,bio),
  addContact: (node:string,alias:string) => call<Snapshot>('AddContact',node,alias),
  sendMessage: (user:string,body:string) => call<Snapshot>('SendMessage',user,body),
  createPost: (text:string,tags:string[]) => call<Snapshot>('CreatePost',text,tags),
  refreshFeed: (user:string) => call<Snapshot>('RefreshContactFeed',user),
  saveNetwork: (input:NetworkInput) => call<Snapshot>('SaveNetwork',input),
  saveCA: (input:CAInput) => call<Snapshot>('SaveCA',input),
  rotateCA: () => call<Snapshot>('RotateCA',true),
  installCA: () => call<Snapshot>('InstallCA'),
  exportCA: () => call<string>('ExportCA'),
  startup: (v:boolean) => call<Snapshot>('SetStartup',v),
  openURL: (url:string) => call<void>('OpenURL',url),
  openData: () => call<void>('OpenDataDir'),
  diagnostics: () => call<string>('WriteDiagnostics'),
  copy: (text:string) => call<void>('CopyText',text),
  onSnapshot: (cb:(s:Snapshot)=>void) => window.runtime?.EventsOn?.('knotroute:snapshot', (...args) => { const s=args[0] as Snapshot; if(s) cb(s); }) ?? (()=>{})
};
