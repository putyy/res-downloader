import {computed, h, type Ref} from "vue"
import {type DataTableRowKey, NButton, NIcon, NImage, NInput, NPopover, NTooltip} from "naive-ui"
import {SearchOutline} from "@vicons/ionicons5"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {formatSize} from "@/func"
import {primaryURL, resourceDomain, resourcePreviewURL, resourceSize} from "@/services/resources"
import Action from "@/components/Action.vue"
import ActionDesc from "@/components/ActionDesc.vue"
import ShowOrEdit from "@/components/ShowOrEdit.vue"
import ResourceTypeCell from "./ResourceTypeCell.vue"
import {executionMessage, shouldShowExecution} from "@/services/operations"
import type {OperationExecution} from "@/types/operations"
import OperationStatus from "./OperationStatus.vue"

interface ResourceColumnOptions {
  t: (key: string, values?: Record<string, unknown>) => string
  classify: Ref<any[]>
  pluginResourceKinds: Ref<appType.ResourceKindDefinition[]>
  resourceKindLabel: (definition: appType.ResourceKindDefinition) => string
  checkedRowKeys: Ref<DataTableRowKey[]>
  resourceDepths: Ref<ReadonlyMap<string, number>>
  descriptionSearch: Ref<string>
  urlSearch: Ref<string>
  previewRow: Ref<appType.ResourceView | undefined>
  showPreview: Ref<boolean>
  downloadStatuses: Ref<Record<string, string>>
  executions: Ref<Record<string, OperationExecution>>
  downloadState: (row: appType.ResourceView) => appType.ResourceDownloadState | undefined
  filterKinds: Ref<string[]>
  hasCapability: (row: appType.ResourceView, capability: string) => boolean
  canDownload: (row: appType.ResourceView) => boolean
  download: (row: appType.ResourceView, index: number) => void
  updateDescription: (id: string, value: string) => void
  resourceActions: (row: appType.ResourceView) => appType.DisplayResourceAction[]
  dataAction: (row: appType.ResourceView, index: number, type: string) => void
}

export const useResourceTableColumns = (options: ResourceColumnOptions) => {
  const columns = computed<any[]>(() => [
    {type: "selection"},
    {
      title: () => {
        if (options.checkedRowKeys.value.length > 0) {
          return h("span", {class: "text-app-accent font-semibold"}, options.t("index.choice") + `(${options.checkedRowKeys.value.length})`)
        }
        return searchTitle(options.t("index.domain"), options.urlSearch, options.t("index.search_description"))
      },
      key: "domain",
      className: "resource-tree-column",
      tree: true,
      width: 160,
      ellipsis: true,
      // Cell padding owns indentation; the table's native indent is zero.
      cellProps: (row: appType.ResourceView) => ({
        style: {paddingLeft: `calc(var(--n-td-padding) + ${(options.resourceDepths.value.get(row.id) ?? 0) * 24}px)`},
      }),
      render: (row: appType.ResourceView) => {
        const domain = resourceDomain(row)
        const details = primaryURL(row)?.trim() || row.source?.pageUrl?.trim() || domain
        const trigger = () => h("span", {class: "cursor-default"}, domain || "—")
        return details ? h(NTooltip, {trigger: "hover", placement: "top"}, {trigger, default: () => details}) : trigger()
      },
    },
    {
      title: options.t("index.type"),
      key: "primaryType",
      width: 80,
      filterOptions: Array.from(options.classify.value).slice(1),
      filterMultiple: true,
      // Keep the filter menu; the page applies collection-aware matching once.
      filter: true,
      filterOptionValues: options.filterKinds.value,
      render: (row: appType.ResourceView) => {
        const item = options.classify.value.find((item) => item.value === row.primaryType)
        const definition = options.pluginResourceKinds.value.find((item) => item.id === row.kind)
        const label = definition ? options.resourceKindLabel(definition) : item ? item.label : row.primaryType || row.kind
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
      title: options.t("index.preview"),
      key: "preview",
      width: 80,
      render: (row: appType.ResourceView) => {
        const canPreview = options.hasCapability(row, "preview") && !!row.preview?.renderer
        if (canPreview && row.preview?.renderer === "image") {
          return h(
            "div",
            {
              class: "bg-app-surface-muted flex h-[76px] w-full items-center justify-center overflow-hidden",
            },
            h(NImage, {
              width: 72,
              height: 72,
              objectFit: "contain",
              lazy: true,
              src: resourcePreviewURL(row),
            }),
          )
        }
        return h(
          NButton,
          {
            strong: true,
            tertiary: true,
            type: "primary",
            size: "small",
            style: {margin: "2px"},
            onClick: () => {
              if (!canPreview) return
              options.previewRow.value = row
              options.showPreview.value = true
            },
          },
          {default: () => options.t(canPreview ? "index.preview" : "index.preview_tip")},
        )
      },
    },
    {
      title: options.t("index.status"),
      key: "download.state",
      width: 100,
      render: (row: appType.ResourceView, index: number) => {
        const download = options.downloadState(row)
        const command = options.executions.value[row.id]
        if (command && shouldShowExecution(command, download)) {
          return h(OperationStatus, {execution: command})
        }
        const effectiveStatus = displayStatus(row, download)
        let type: "primary" | "success" | "warning" = "primary"
        if (effectiveStatus === "done") type = "success"
        else if (effectiveStatus === "pending" || effectiveStatus === "partial") type = "warning"
        return h(
          NButton,
          {
            tertiary: true,
            type,
            size: "small",
            style: {margin: "2px"},
            onClick: () => {
              if (download?.outputPath && effectiveStatus === "done") appApi.openFolder({filePath: download.outputPath})
              else if (effectiveStatus === "ready" && options.canDownload(row)) options.download(row, index)
            },
          },
          {
            default: () => (effectiveStatus === "running" ? runningStatusLabel(row, download, options.t) : options.downloadStatuses.value[effectiveStatus]),
          },
        )
      },
    },
    {
      title: () => searchTitle(options.t("index.description"), options.descriptionSearch, options.t("index.search_description")),
      key: "title",
      width: 150,
      render: (row: appType.ResourceView) =>
        h(ShowOrEdit, {
          value: row.title || "",
          onUpdateValue: (value: string) => options.updateDescription(row.id, value),
        }),
    },
    {
      title: options.t("index.resource_size"),
      key: "size",
      width: 120,
      sorter: (left: appType.ResourceView, right: appType.ResourceView) => resourceSize(left) - resourceSize(right),
      render: (row: appType.ResourceView) => formatSize(resourceSize(row)),
    },
    {
      title: options.t("index.save_path"),
      key: "download.outputPath",
      render: (row: appType.ResourceView) => {
        const download = options.downloadState(row)
        const command = options.executions.value[row.id]
        if (command && shouldShowExecution(command, download)) {
          let message = executionMessage(command, options.t)
          if (!command.syncUnavailable && command.state === "running" && command.progress != null && !/[%％]/.test(message)) {
            message += ` · ${Math.floor(command.progress)}%`
          }
          return h("span", {class: "text-app-muted ellipsis-2", title: message}, message)
        }
        if (download?.state === "completed" && download.outputPath) {
          return h(
            "a",
            {
              href: "#",
              class: "text-app-accent ellipsis-2",
              onClick: (event: MouseEvent) => {
                event.preventDefault()
                appApi.openFolder({filePath: download.outputPath})
              },
            },
            download.outputPath,
          )
        }
        if (download && download.state !== "ready") {
          const label = options.t(`tasks.${download.state === "failed" ? "status_failed" : download.state}`)
          // Download byte percentage is not the media-processing percentage.
          const detail = download.state === "processing" ? "" : download.message
          const message = detail && detail !== label ? `${label} · ${detail}` : label
          return h("span", {class: "text-app-muted ellipsis-2", title: message}, message)
        }
        return ""
      },
    },
    {
      key: "actions",
      width: 190,
      render: (row: appType.ResourceView, index: number) =>
        h(Action, {
          key: row.id,
          row,
          download: options.downloadState(row),
          index,
          pluginActions: options.resourceActions(row),
          onAction: options.dataAction,
        }),
      title: () => h(ActionDesc),
    },
  ])

  return {columns}
}

const runningStatusLabel = (row: appType.ResourceView, download: appType.ResourceDownloadState | undefined, t: ResourceColumnOptions["t"]) => {
  if ((row.traits ?? []).includes("live")) return t("index.recording")
  const raw = String(download?.message || "").trim()
  return raw || t("index.running")
}

const displayStatus = (row: appType.ResourceView, download: appType.ResourceDownloadState | undefined): string => {
  if (row.state === "partial") return "partial"
  const state = download?.state || "ready"
  if (state === "pending" || state === "paused") return "pending"
  if (["resolving", "downloading", "processing", "pausing"].includes(state)) return "running"
  if (state === "completed") return "done"
  if (state === "failed" || state === "interrupted") return "error"
  return "ready"
}

const searchTitle = (title: string, value: Ref<string>, placeholder: string) =>
  h("div", {class: "flex items-center"}, [
    title,
    h(
      NPopover,
      {
        style: "--wails-draggable:no-drag",
        trigger: "click",
        placement: "bottom",
        showArrow: true,
      },
      {
        trigger: () =>
          h(
            NIcon,
            {
              size: "18",
              class: `ml-1 cursor-pointer ${value.value ? "text-app-accent" : "text-app-muted"}`,
              onClick: (event: MouseEvent) => event.stopPropagation(),
            },
            h(SearchOutline),
          ),
        default: () =>
          h("div", {class: "p-2 w-64"}, [
            h(
              NInput,
              {
                value: value.value,
                "onUpdate:value": (next: string) => (value.value = next),
                placeholder,
                clearable: true,
              },
              {prefix: () => h(NIcon, {component: SearchOutline})},
            ),
          ]),
      },
    ),
  ])
