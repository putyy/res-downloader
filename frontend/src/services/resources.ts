import type {appType} from '@/types/app'

export const visitResource = (item: appType.ResourceView, visitor: (item: appType.ResourceView) => void) => {
    visitor(item)
    ;(item.children ?? []).forEach(child => visitResource(child, visitor))
}

export const resourceSome = (item: appType.ResourceView, predicate: (item: appType.ResourceView) => boolean): boolean =>
    predicate(item) || (item.children ?? []).some(child => resourceSome(child, predicate))

export const findResourceInTree = (items: appType.ResourceView[], id: string): appType.ResourceView | undefined => {
    for (const item of items) {
        let found: appType.ResourceView | undefined
        visitResource(item, child => {
            if (!found && child.id === id) found = child
        })
        if (found) return found
    }
}

export const mergeResourceRuntime = (incoming: appType.ResourceView, current?: appType.ResourceView): appType.ResourceView => {
    const merged = Object.assign({}, current ?? {}, incoming)
    if (current?.download) merged.download = current.download
    const children = (incoming.children ?? []).map(child =>
        mergeResourceRuntime(child, current?.children?.find(item => item.id === child.id)),
    )
    for (const previous of current?.children ?? []) {
        if (!children.some(child => child.id === previous.id)) children.push(previous)
    }
    merged.children = children
    return merged
}

export const removeResourceFromTree = (items: appType.ResourceView[], id: string): boolean => {
    const rootIndex = items.findIndex(item => item.id === id)
    if (rootIndex >= 0) {
        items.splice(rootIndex, 1)
        return true
    }
    const removeChild = (parent: appType.ResourceView): boolean => {
        const children = parent.children ?? []
        const childIndex = children.findIndex(child => child.id === id)
        if (childIndex >= 0) {
            children.splice(childIndex, 1)
            return true
        }
        return children.some(removeChild)
    }
    return items.some(removeChild)
}

export const primaryTrack = (row: appType.ResourceView) => {
    const tracks = Array.isArray(row.tracks) ? row.tracks : []
    return tracks.find(track => track.role === 'primary' || track.role === 'video') || tracks[0]
}

export const primaryURL = (row: appType.ResourceView): string => {
    const own = primaryTrack(row)?.url || ''
    if (own) return own
    for (const child of row.children ?? []) {
        const childURL = primaryURL(child)
        if (childURL) return childURL
    }
    return ''
}

export const resourcePreviewURL = (row: appType.ResourceView): string => {
    const query = new URLSearchParams()
    if (row.id) query.set('id', row.id)
	if (window.$apiToken) query.set('access_token', window.$apiToken)
    return window?.$baseUrl + '/api/preview?' + query.toString()
}

export const resourceDomain = (row: appType.ResourceView): string => {
    if (row.source?.domain) return row.source.domain
    try {
        return new URL(primaryURL(row)).hostname
    } catch {
        return ''
    }
}

export const resourceSize = (row: appType.ResourceView): number => {
    if ((row.children?.length ?? 0) > 0) return row.children!.reduce((total, child) => total + resourceSize(child), 0)
    return (row.tracks ?? []).reduce((total, track) => total + (track.size ?? 0), 0)
}

export const exportableResource = (row: appType.ResourceView): object => {
    const {download: _download, children, ...resource} = row
    return {...resource, ...(children?.length ? {children: children.map(exportableResource)} : {})}
}
