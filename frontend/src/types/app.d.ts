export namespace appType {
    interface App {
        AppName: string
        Version: string
        Description: string
        Copyright: string
    }

    interface MimeMap {
        Type: string
        Suffix: string
    }

    interface Config {
        Theme: string
        Locale: string
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
        DownNumber: number
        UserAgent: string
        UseHeaders: string
        InsertTail: boolean
        MimeMap: { [key: string]: MimeMap }
        Rule: string
    }

    interface MediaInfo {
        Id: string
        Url: string
        UrlSign: string
        CoverUrl: string
        Size: number
        Domain: string
        Classify: string
        Suffix: string
        SavePath: string
        Status: string
        DecodeKey: string
        Description: string
        ContentType: string
        OtherData: { [key: string]: string }
        Comments?: CommentItem[]
        CommentMeta?: CommentMeta
    }

    type CommentStatus = 'idle' | 'queued' | 'running' | 'success' | 'partial_success' |
        'no_comments' | 'identity_unavailable' | 'timeout' | 'target_mismatch' |
        'request_failed' | 'login_expired' | 'parse_failed' | 'interrupted'

    interface CommentItem {
        nickName: string
        content: string
        likeCount: number
        createdAt: number
        replyCount: number
        region: string
    }

    interface CommentMeta {
        status: CommentStatus
        requestId?: string
        code?: string
        updatedAt: number
        fetchedAt?: number
        totalCount?: number
        targetVerified?: boolean
    }

    interface CommentTaskStatus {
        requestId: string
        resId: string
        state: 'queued' | 'running' | 'completed' | 'failed'
        code?: string
        count?: number
        updatedAt: number
    }

    interface CommentResult {
        requestId: string
        objectId: string
        resId: string
        urlSign: string
        comments: CommentItem[]
        status: 'success' | 'partial_success' | 'no_comments'
        totalCount: number
        targetVerified: boolean
        fetchedAt: number
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

    interface Res<T = any> {
        code: number;
        message: string;
        data: T;  // T will be the specific type of your data
    }
}
