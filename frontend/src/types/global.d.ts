interface Window {
    $loadingBar?: import('naive-ui').LoadingBarProviderInst;
    $dialog?: import('naive-ui').DialogProviderInst;
    $message?: import('naive-ui').MessageProviderInst;
    $notification?: import('naive-ui').NotificationProviderInst;
    $ws?: WebSocket;
}

declare module '*.vue' {
    import {App, defineComponent} from 'vue'
    const component: ReturnType<typeof defineComponent> & {
        install(app: App): void
    }
    export default component
}
