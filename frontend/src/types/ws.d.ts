export namespace wsType {
    interface Event {
        type: string
        event: any
        data: any
    }

    interface Handle {
        type: string
        event: any
    }

    interface Message {
        code: number
        message: string
    }

    interface Clipboard {
        content: string
    }
}