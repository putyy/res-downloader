import type {AppThemeName} from '@/themes'

export namespace appType {
    interface App {
        AppName: string
        Version: string
        Description: string
        Copyright: string
    }

    interface Config {
        Theme: AppThemeName
        Locale: string
        Host: string
        Port: string
        SaveDirectory: string
        FilenameTemplate: string
        FilenameConflict: 'rename' | 'overwrite' | 'skip'
        UpstreamProxy: string
        OpenProxy: boolean
        DownloadProxy: boolean
        FFmpegPath: string
        FFprobePath: string
        AutoProxy: boolean
        TaskNumber: number
        DownNumber: number
        UserAgent: string
        UseHeaders: string
        InsertTail: boolean
        InterceptionPolicies: InterceptionPolicy[]
    }

    interface InterceptionPolicy {
        id: string
        name: string
        enabled: boolean
        domains: string[]
        exclude?: string[]
        action: 'mitm' | 'pass'
    }

    interface MediaToolStatus {
        available: boolean
        path?: string
        version?: string
        error?: string
    }

    interface MediaEngineStatus {
        ffmpeg: MediaToolStatus
        ffprobe: MediaToolStatus
        checkedAt: number
    }

    interface CertificateStatus {
        fingerprintSha256?: string
        desktop: { installed: boolean; fingerprintSha1?: string; checkedAt: number; error?: string }
        migration: { version: number; status: string; message?: string; checkedAt: number }
        needsPhoneCleanup: boolean
        error?: string
    }

    interface ResourceSource {
        pluginId: string
        pluginVersion?: string
        pluginDigest?: string
        pageUrl?: string
        domain?: string
    }

    interface ResourceDownloadState {
        taskId?: string
        state: 'ready' | 'pending' | 'resolving' | 'downloading' | 'processing' | 'pausing' | 'paused' | 'completed' | 'failed' | 'cancelled' | 'interrupted'
        outputPath?: string
        message?: string
        downloaded?: number
        total?: number
    }

    interface ResourceView {
        id: string
        groupKey?: string
        parentGroupKey?: string
        parentId?: string
        dedupeKey?: string
        kind?: string
        primaryType?: 'video' | 'audio' | 'image' | 'document' | 'archive' | 'collection' | 'other'
        traits?: string[]
        technical?: ResourceTechnical
        lifecycle?: ResourceLifecycle
        title?: string
        coverUrl?: string
        tracks?: ResourceTrack[]
        requiredTracks?: string[]
        state?: 'partial' | 'ready'
        capabilities?: string[]
        preview?: PreviewSpec
        metadata?: { [key: string]: any }
        actions?: ResourceAction[]
        source?: ResourceSource
        children?: ResourceView[]
        download?: ResourceDownloadState
    }

    interface ResourceTrack {
        id: string
        role: string
        executor?: string
        url: string
        captureKey?: string
        mime?: string
        extension?: string
        size?: number
        quality?: string
        width?: number
        height?: number
        bitrate?: number
        codecs?: string
        headers?: { [key: string]: string }
        nonPersistentHeaders?: string[]
        processors?: DownloadStep[]
    }

    interface ResourceTechnical {
        mime?: string
        container?: string
        codecs?: string
        duration?: number
    }

    interface ResourceLifecycle {
        schemaVersion: number
        discoveredAt: number
        updatedAt: number
        expiresAt?: number
        lastResolvedAt?: number
        availability: 'available' | 'expired' | 'pluginMissing' | 'pluginIncompatible' | 'needsRefresh' | 'unavailable'
        unavailableReason?: string
    }

    interface DownloadTaskRecord {
        id: string
        resourceId: string
        parentId?: string
        resource: {
            id: string
            title?: string
            coverUrl?: string
            kind?: string
            primaryType?: string
            source?: { pluginId?: string; pluginVersion?: string }
        }
        pluginId?: string
        pluginVersion?: string
        items?: {
            resource: { id: string; title?: string; primaryType?: string };
            state?: string;
            outputPath?: string
        }[]
        state: 'pending' | 'resolving' | 'downloading' | 'processing' | 'pausing' | 'paused' | 'completed' | 'failed' | 'cancelled' | 'interrupted'
        resumable?: boolean
        recording?: boolean
        step?: string
        attempts: number
        resumes?: number
        downloaded?: number
        total?: number
        outputPath?: string
        error?: string
        createdAt: number
        updatedAt: number
        startedAt?: number
        finishedAt?: number
    }

    type DownloadTaskBatchAction = 'pause' | 'resume' | 'cancel' | 'retry' | 'delete'

    interface DownloadTaskBatchResult {
        id: string
        success: boolean
        error?: string
        task?: DownloadTaskRecord
    }

    interface DownloadTaskBatchResponse {
        succeeded: number
        failed: number
        results: DownloadTaskBatchResult[]
    }

    interface PreviewSpec {
        renderer: 'image' | 'audio' | 'video' | 'pdf' | 'text' | string
        mode?: string
        mime?: string
        codecs?: string
        trackId?: string
    }

    interface ResourceAction {
        id: string
        label?: string
        data?: { [key: string]: any }
    }

    interface PluginActionDefinition {
        kind: 'process-file' | string
        processor: string
        inputExtensions?: string[]
        outputExtension?: string
        locales?: { [locale: string]: PluginLocale }
    }

    interface DisplayResourceAction {
        id: string
        label: string
        description?: string
    }

    interface DownloadStep {
        type: string
        options?: { [key: string]: any }
    }

    interface PluginManifest {
        id: string
        name: string
        author?: PluginAuthor
        version: string
        apiVersion: number
        runtime: string
        priority?: number
        enabled?: boolean
        resourceKinds?: ResourceKindDefinition[]
        settingsSchema?: { [key: string]: any }
        processors?: { [key: string]: PluginProcessorDefinition }
        actions?: { [key: string]: PluginActionDefinition }
        locales?: { [locale: string]: PluginLocale }
        permissions?: PluginPermissions
        pageScripts?: PluginPageScript[]
        requires?: { ffmpeg?: string }
    }

    interface PluginPermissions {
        domains?: string[]
        capabilities?: string[]
        bodyLimit?: number
    }

    interface PluginPageScript {
        id: string
        entry: string
        match: Array<{ host: string; path?: string; url?: string }>
        runAt?: 'document-start'
        frames?: 'top' | 'all'
        bridge?: boolean
    }

    interface PluginAuthor {
        name?: string
        url?: string
    }

    interface PluginLocale {
        name?: string
        description?: string
    }

    interface ResourceKindDefinition {
        id: string
        icon?: string
        color?: string
        locales?: { [locale: string]: PluginLocale }
    }

    interface CaptureRule {
        id: string
        name?: string
        enabled: boolean
        priority?: number
        match: CaptureRuleMatch
        resource: CaptureRuleResource
    }

    interface CaptureRuleMatch {
        mime?: string[]
        url?: string[]
        contentDisposition?: string[]
        status?: number[]
        minSize?: number
        maxSize?: number
    }

    interface CaptureRuleResource {
        kind: string
        role?: string
        extension?: string
        executor?: 'http-file' | 'hls'
        capabilities?: string[]
        previewRenderer?: string
        previewMode?: string
    }

    interface PluginProcessorDefinition {
        runtime: string
        entry: string
        apiVersion: number
    }

    interface PluginStatus {
        manifest: PluginManifest
        path?: string
        source: 'builtin' | 'official' | 'community'
        loaded: boolean
        error?: string
        builtin: boolean
        bundled?: boolean
        reloadedAt?: number
        digest?: string
        rollbackAvailable?: boolean
        health?: PluginRuntimeHealth
    }

    interface PluginRuntimeHealth {
        consecutiveErrors?: number
        totalErrors?: number
        slowCalls?: number
        pausedUntil?: number
        lastError?: string
        lastDurationMs?: number
    }

    interface PluginStoreIndex {
        schemaVersion: number
        generatedAt: string
        topic: string
        extensions: PluginStoreEntry[]
    }

    interface PluginStoreEntry {
        id?: string
        name: string
        description?: string
        repository: string
        repositoryUrl: string
        homepage?: string
        owner: string
        ownerAvatarUrl?: string
        stars?: number
        forks?: number
        license?: string
        updatedAt?: string
        source: 'official' | 'community'
        status: 'available' | 'unavailable'
        statusMessage?: string
        manifest?: PluginManifest
        release?: PluginStoreRelease
    }

    interface PluginStoreRelease {
        version: string
        tag: string
        publishedAt?: string
        notesUrl?: string
        archiveUrl: string
        acceleratedUrl?: string
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
