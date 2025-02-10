export namespace appType {
    interface App {
        AppName: string
        Version: string
        Description: string
        Copyright: string
    }

    interface Config {
        Theme: string
        Host: string
        Port: string
        Quality: number
        SaveDirectory: string
        FilenameLen: number
        FilenameTime: boolean
        UpstreamProxy: string
        OpenProxy: boolean
        DownloadProxy: boolean
        AutoProxy: boolean
        WxAction: boolean
        TaskNumber: number
        UserAgent: string
    }

    interface MediaInfo {
        Id: string
        Url: string
        UrlSign: string
        CoverUrl: string
        Size: string
        Domain: string
        Classify: string
        Suffix: string
        SavePath: string
        Status: string
        DecodeKey: string
        Description: string
        ContentType: string
        OtherData: {[key: string]: string}
    }

    interface DownloadProgress {
        Id: string
        SavePath: string
        Status: string
        Message: string
    }

    interface Message {
        code: number
        message: string
    }

    interface Handle {
        type: string
        event: any
    }
}