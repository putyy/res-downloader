import {computed, h, type Ref, unref} from 'vue'
import {type DataTableRowKey, NButton, NDataTable, NIcon, NImage, NInput, NPopover, NTooltip,} from 'naive-ui'
import {SearchOutline} from '@vicons/ionicons5'
import type {appType} from '@/types/app'
import appApi from '@/api/app'
import {formatSize} from '@/func'
import {primaryURL, resourceDomain, resourcePreviewURL, resourceSize} from '@/services/resources'
import Action from '@/components/Action.vue'
import ActionDesc from '@/components/ActionDesc.vue'
import ShowOrEdit from '@/components/ShowOrEdit.vue'
import ResourceTypeCell from './ResourceTypeCell.vue'
import {pageCommandMessage, shouldShowPageCommand} from '@/services/pageCommands'
import PageCommandStatus from './PageCommandStatus.vue'

interface ResourceColumnOptions {
    t: (key: string, values?: Record<string, unknown>) => string
    classify: Ref<any[]>
    pluginResourceKinds: Ref<appType.ResourceKindDefinition[]>
    resourceKindLabel: (definition: appType.ResourceKindDefinition) => string
    checkedRowKeys: Ref<DataTableRowKey[]>
    descriptionSearch: Ref<string>
    urlSearch: Ref<string>
    previewRow: Ref<appType.ResourceView | undefined>
    showPreview: Ref<boolean>
    downloadStatuses: Ref<Record<string, string>>
    pageCommands: Ref<Record<string, appType.PageCommandStatus>>
    rowKey: (row: appType.ResourceView) => DataTableRowKey
    hasCapability: (row: appType.ResourceView, capability: string) => boolean
    canDownload: (row: appType.ResourceView) => boolean
    download: (row: appType.ResourceView, index: number) => void
    updateDescription: (id: string, value: string) => void
    resourceActions: (row: appType.ResourceView) => appType.DisplayResourceAction[]
    dataAction: (row: appType.ResourceView, index: number, type: string) => void
}

export const useResourceTableColumns = (options: ResourceColumnOptions) => {
    const columns = computed<any[]>(() => [
        {
            type: 'expand',
            width: 20,
            expandable: (row: appType.ResourceView) => (row.children?.length ?? 0) > 0,
            renderExpand: (row: appType.ResourceView) => h(NDataTable, {
                class: 'resource-child-table',
                columns: childColumns.value,
                data: row.children ?? [],
                rowKey: options.rowKey,
                bordered: false,
                singleLine: false,
                size: 'small',
                maxHeight: 360,
                style: {padding: '4px 12px 8px 28px'},
            }),
        },
        {type: 'selection'},
        {
            title: () => {
                if (options.checkedRowKeys.value.length > 0) {
                    return h('span', {class: 'text-app-accent font-semibold'}, options.t('index.choice') + `(${options.checkedRowKeys.value.length})`)
                }
                return searchTitle(
                    options.t('index.domain'),
                    options.urlSearch,
                    options.t('index.search_description'),
                )
            },
            key: 'domain',
            width: 90,
            render: (row: appType.ResourceView) => h(NTooltip, {trigger: 'hover', placement: 'top'}, {
                trigger: () => h('span', {class: 'cursor-default'}, resourceDomain(row)),
                default: () => primaryURL(row),
            }),
        },
        {
            title: options.t('index.type'),
            key: 'primaryType',
            width: 80,
            filterOptions: Array.from(options.classify.value).slice(1),
            filterMultiple: true,
            filter: (value: string, row: appType.ResourceView) =>
                row.primaryType === String(value) || row.kind === String(value),
            render: (row: appType.ResourceView) => {
                const item = options.classify.value.find(item => item.value === row.primaryType)
                const definition = options.pluginResourceKinds.value.find(item => item.id === row.kind)
                const label = definition
                    ? options.resourceKindLabel(definition)
                    : item ? unref(item.label) : row.primaryType || row.kind
                return h(ResourceTypeCell, {
                    label,
                    primaryType: row.primaryType,
                    kind: row.kind,
                    traits: row.traits || [],
                    color: definition?.color,
                })
            },
        },
        {
            title: options.t('index.preview'),
            key: 'preview',
            width: 80,
            render: (row: appType.ResourceView) => {
                const canPreview = options.hasCapability(row, 'preview') && !!row.preview?.renderer
                if (canPreview && row.preview?.renderer === 'image') {
                    return h('div', {
                        class: 'bg-app-surface-muted flex h-[76px] w-full items-center justify-center overflow-hidden',
                    }, h(NImage, {
                        width: 72,
                        height: 72,
                        objectFit: 'contain',
                        lazy: true,
                        src: resourcePreviewURL(row),
                    }))
                }
                return h(NButton, {
                    strong: true,
                    tertiary: true,
                    type: 'primary',
                    size: 'small',
                    style: {margin: '2px'},
                    onClick: () => {
                        if (!canPreview) return
                        options.previewRow.value = row
                        options.showPreview.value = true
                    },
                }, {default: () => options.t(canPreview ? 'index.preview' : 'index.preview_tip')})
            },
        },
        {
            title: options.t('index.status'),
            key: 'download.state',
            width: 100,
            render: (row: appType.ResourceView, index: number) => {
                const command = options.pageCommands.value[row.id]
                if (command && shouldShowPageCommand(command, row.download)) {
                    return h(PageCommandStatus, {command})
                }
                const effectiveStatus = displayStatus(row)
                let type: 'primary' | 'success' | 'warning' = 'primary'
                if (effectiveStatus === 'done') type = 'success'
                else if (effectiveStatus === 'pending' || effectiveStatus === 'partial') type = 'warning'
                return h(NButton, {
                    tertiary: true,
                    type,
                    size: 'small',
                    style: {margin: '2px'},
                    onClick: () => {
                        if (row.download?.outputPath && effectiveStatus === 'done') appApi.openFolder({filePath: row.download.outputPath})
                        else if (effectiveStatus === 'ready' && options.canDownload(row)) options.download(row, index)
                    },
                }, {
                    default: () => effectiveStatus === 'running'
                        ? runningStatusLabel(row, options.t)
                        : options.downloadStatuses.value[effectiveStatus],
                })
            },
        },
        {
            title: () => searchTitle(
                options.t('index.description'),
                options.descriptionSearch,
                options.t('index.search_description'),
            ),
            key: 'title',
            width: 150,
            render: (row: appType.ResourceView) => h(ShowOrEdit, {
                value: row.title || '',
                onUpdateValue: (value: string) => options.updateDescription(row.id, value),
            }),
        },
        {
            title: options.t('index.resource_size'),
            key: 'size',
            width: 120,
            sorter: (left: appType.ResourceView, right: appType.ResourceView) => resourceSize(left) - resourceSize(right),
            render: (row: appType.ResourceView) => formatSize(resourceSize(row)),
        },
        {
            title: options.t('index.save_path'),
            key: 'download.outputPath',
            render: (row: appType.ResourceView) => {
                const command = options.pageCommands.value[row.id]
                if (command && shouldShowPageCommand(command, row.download)) {
                    let message = pageCommandMessage(command, options.t)
                    if (!command.syncUnavailable && command.state === 'running' && command.progress != null && !/[%％]/.test(message)) {
                        message += ` · ${Math.floor(command.progress)}%`
                    }
                    return h('span', {class: 'text-app-muted ellipsis-2', title: message}, message)
                }
                const download = row.download
                if (download?.state === 'completed' && download.outputPath) {
                    return h('a', {
                        href: '#', class: 'text-app-accent ellipsis-2',
                        onClick: (event: MouseEvent) => {
                            event.preventDefault()
                            appApi.openFolder({filePath: download.outputPath})
                        },
                    }, download.outputPath)
                }
                if (download && download.state !== 'ready') {
                    const label = options.t(`tasks.${download.state === 'failed' ? 'status_failed' : download.state}`)
                    // Download byte percentage is not the media-processing percentage.
                    const detail = download.state === 'processing' ? '' : download.message
                    const message = detail && detail !== label ? `${label} · ${detail}` : label
                    return h('span', {class: 'text-app-muted ellipsis-2', title: message}, message)
                }
                return ''
            },
        },
        {
            key: 'actions',
            width: 130,
            render: (row: appType.ResourceView, index: number) => h(Action, {
                key: row.id,
                row,
                index,
                pluginActions: options.resourceActions(row),
                onAction: options.dataAction,
            }),
            title: () => h(ActionDesc),
        },
    ])

    const childColumns = computed(() => columns.value.filter(column => column.type !== 'expand' && column.type !== 'selection'))
    return {columns, childColumns}
}

const runningStatusLabel = (
    row: appType.ResourceView,
    t: ResourceColumnOptions['t'],
) => {
    if ((row.traits ?? []).includes('live')) return t('index.recording')
    const raw = String(row.download?.message || '').trim()
    const byteProgress = /^(\d+)\s*B$/i.exec(raw)
    if (byteProgress) return formatSize(Number(byteProgress[1]))
    return raw || t('index.running')
}

const displayStatus = (row: appType.ResourceView): string => {
    if (row.state === 'partial') return 'partial'
    const state = row.download?.state || 'ready'
    if (state === 'pending' || state === 'paused') return 'pending'
    if (['resolving', 'downloading', 'processing', 'pausing'].includes(state)) return 'running'
    if (state === 'completed') return 'done'
    if (state === 'failed' || state === 'interrupted') return 'error'
    return 'ready'
}

const searchTitle = (
    title: string,
    value: Ref<string>,
    placeholder: string,
) => h('div', {class: 'flex items-center'}, [
    title,
    h(NPopover, {
        style: '--wails-draggable:no-drag',
        trigger: 'click',
        placement: 'bottom',
        showArrow: true,
    }, {
        trigger: () => h(NIcon, {
            size: '18',
            class: `ml-1 cursor-pointer ${value.value ? 'text-app-accent' : 'text-app-muted'}`,
            onClick: (event: MouseEvent) => event.stopPropagation(),
        }, h(SearchOutline)),
        default: () => h('div', {class: 'p-2 w-64'}, [
            h(NInput, {
                value: value.value,
                'onUpdate:value': (next: string) => value.value = next,
                placeholder,
                clearable: true,
            }, {prefix: () => h(NIcon, {component: SearchOutline})}),
        ]),
    }),
])
