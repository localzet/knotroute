const api = () => window.go?.main?.App;
async function call(name, ...args) {
    const fn = api()?.[name];
    if (!fn)
        throw new Error(`Desktop bridge ${name} is unavailable`);
    return await fn(...args);
}
export const bridge = {
    snapshot: () => call('GetSnapshot'),
    toggleNetwork: () => call('ToggleNetwork'),
    browserIntegration: (v) => call('SetBrowserIntegration', v),
    saveProfile: (name, bio) => call('SaveProfile', name, bio),
    addContact: (node, alias) => call('AddContact', node, alias),
    sendMessage: (user, body) => call('SendMessage', user, body),
    createPost: (text, tags) => call('CreatePost', text, tags),
    refreshFeed: (user) => call('RefreshContactFeed', user),
    saveNetwork: (input) => call('SaveNetwork', input),
    saveCA: (input) => call('SaveCA', input),
    rotateCA: () => call('RotateCA', true),
    installCA: () => call('InstallCA'),
    exportCA: () => call('ExportCA'),
    startup: (v) => call('SetStartup', v),
    openURL: (url) => call('OpenURL', url),
    openData: () => call('OpenDataDir'),
    diagnostics: () => call('WriteDiagnostics'),
    copy: (text) => call('CopyText', text),
    onSnapshot: (cb) => window.runtime?.EventsOn?.('knotroute:snapshot', (...args) => { const s = args[0]; if (s)
        cb(s); }) ?? (() => { })
};
