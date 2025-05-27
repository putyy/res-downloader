<template>
  <div class="h-full flex flex-col p-5 overflow-y-auto [&::-webkit-scrollbar]:hidden">
    <div class="pb-2 z-40">
      <NSpace>
        <NButton v-if="isProxy" secondary type="primary" @click.stop="close" style="--wails-draggable:no-drag">
          {{ t("index.close_grab") }}
        </NButton>
        <NButton v-else tertiary type="tertiary" @click.stop="open" style="--wails-draggable:no-drag">
          {{ t("index.open_grab") }}
        </NButton>
        <NButton tertiary type="error" @click.stop="clear" style="--wails-draggable:no-drag">
          {{ t("index.clear_list") }}
        </NButton>
        <NSelect style="min-width: 100px;--wails-draggable:no-drag" :placeholder="t('index.grab_type')"
                 v-model:value="resourcesType" multiple clearable :max-tag-count="3" :options="classify"></NSelect>
        <NButton tertiary type="info" @click.stop="batchDown" style="--wails-draggable:no-drag">
          {{ t("index.batch_download") }}
        </NButton>
        <NButton tertiary type="info" @click.stop="batchImport" style="--wails-draggable:no-drag">
          {{ t("index.batch_export") }}
        </NButton>
        <NButton tertiary type="info" @click.stop="showImport=true" style="--wails-draggable:no-drag">
          {{ t("index.batch_import") }}
        </NButton>
      </NSpace>
    </div>
    <div class="flex-1">
      <NDataTable
          :columns="columns"
          :data="data"
          :bordered="false"
          :max-height="tableHeight"
          :row-key="rowKey"
          :virtual-scroll="true"
          :header-height="48"
          :height-for-row="()=> 48"
          :checked-row-keys="checkedRowKeysValue"
          @update:checked-row-keys="handleCheck"
          style="--wails-draggable:no-drag"
      />
    </div>
    <Preview v-model:showModal="showPreviewRow" :previewRow="previewRow"/>
    <ShowLoading :loadingText="loadingText" :isLoading="loading"/>
    <ImportJson v-model:showModal="showImport" @submit="handleImport"/>
    <Password v-model:showModal="showPassword" @submit="handlePassword"/>
  </div>
</template>

<script lang="ts" setup>
import {NButton, NImage, NTooltip} from "naive-ui"
import {computed, h, onMounted, ref, reactive, watch} from "vue"
import type {appType} from "@/types/app"

import type {DataTableRowKey, ImageRenderToolbarProps} from "naive-ui"
import Preview from "@/components/Preview.vue"
import ShowLoading from "@/components/ShowLoading.vue"
// @ts-ignore
import {getDecryptionArray} from '@/assets/js/decrypt.js'
import {useIndexStore} from "@/stores"
import appApi from "@/api/app"
import ResAction from "@/components/ResAction.vue"
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {BrowserOpenURL, ClipboardSetText} from "../../wailsjs/runtime"
import Password from "@/components/Password.vue"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const eventStore = useEventStore()
const isProxy = computed(() => {
  return store.isProxy
})
const data = ref<any[]>([])
const store = useIndexStore()
const tableHeight = computed(() => {
  return store.globalConfig.Locale === "zh" ? store.tableHeight - 130 : store.tableHeight - 151
})
const resourcesType = ref<string[]>(["all"])

const classifyAlias: { [key: string]: any } = {
  image: computed(() => t("index.image")),
  audio: computed(() => t("index.audio")),
  video: computed(() => t("index.video")),
  m3u8: computed(() => t("index.m3u8")),
  live: computed(() => t("index.live")),
  xls: computed(() => t("index.xls")),
  doc: computed(() => t("index.doc")),
  pdf: computed(() => t("index.pdf")),
  font: computed(() => t("index.font"))
}

const dwStatus = computed<any>(() => {
  return {
    ready: t("common.ready"),
    running: t("common.running"),
    error: t("common.error"),
    done: t("common.done"),
    handle: t("common.handle")
  }
})

const classify = ref([
  {
    value: "all",
    label: computed(() => t("index.all")),
  },
])

const columns = ref<any[]>([
  {
    type: "selection",
  },
  {
    title: computed(() => t("index.domain")),
    key: "Domain",
  },
  {
    title: computed(() => t("index.type")),
    key: "Classify",
    filterOptions: computed(() => Array.from(classify.value).slice(1)),
    filterMultiple: true,
    filter: (value: string, row: appType.MediaInfo) => {
      return !!~row.Classify.indexOf(String(value))
    },
    render: (row: appType.MediaInfo) => {
      for (const key in classify.value) {
        if (classify.value[key].value === row.Classify) {
          return classify.value[key].label;
        }
      }
      return row.Classify;
    }
  },
  {
    title: computed(() => t("index.preview")),
    key: "Url",
    width: 120,
    render: (row: appType.MediaInfo) => {
      if (row.Classify === "image") {
        return h(NImage, {
          maxWidth: "80px",
          lazy: true,
          "render-toolbar": renderToolbar,
          src: row.Url
        })
      }
      return [
        h(
            NButton,
            {
              strong: true,
              tertiary: true,
              type: "info",
              size: "small",
              style: {
                margin: "2px"
              },
              onClick: () => {
                if (row.Classify === "audio" || row.Classify === "video" || row.Classify === "m3u8" || row.Classify === "live") {
                  previewRow.value = row
                  showPreviewRow.value = true
                }
              }
            },
            {
              default: () => {
                if (row.Classify === "audio" || row.Classify === "video" || row.Classify === "m3u8" || row.Classify === "live") {
                  return t("index.preview")
                }
                return t("index.preview_tip")
              }
            }
        ),
      ]
    }
  },
  {
    title: computed(() => t("index.status")),
    key: "Status",
    render: (row: appType.MediaInfo) => {
      return dwStatus[row.Status as keyof typeof dwStatus]
    }
  },
  {
    title: computed(() => t("index.description")),
    key: "Description",
    width: 150,
    render: (row: appType.MediaInfo, index: number) => {
      return h(NTooltip, {trigger: 'hover', placement: 'top'}, {
        trigger: () => h("div", {}, row.Description.length > 16 ? row.Description.substring(0, 16) + "..." : row.Description),
        default: () => h("div", {
          style: {
            "max-width": " 400px",
            "white-space": "normal",
            "word-wrap": "break-word"
          }
        }, row.Description)
      });
    }
  },
  {
    title: computed(() => t("index.resource_size")),
    key: "Size"
  },
  {
    title: computed(() => t("index.save_path")),
    key: "SavePath",
    render(row: appType.MediaInfo, index: number) {
      return h("a",
          {
            href: "javascript:;",
            style: {
              color: "#5a95d0"
            },
            onClick: () => {
              if (row.SavePath && row.Status === "done") {
                appApi.openFolder({filePath: row.SavePath})
              }
            }
          },
          row.SavePath
      )
    }
  },
  {
    title: computed(() => t("index.operation")),
    key: "actions",
    render(row: appType.MediaInfo, index: number) {
      return h(ResAction, {key: index, row: row, index: index, onAction: dataAction})
    }
  }
])
const checkedRowKeysValue = ref<DataTableRowKey[]>([])
const showPreviewRow = ref(false)
const previewRow = ref<appType.MediaInfo>()
const loading = ref(false)
const loadingText = ref("")
const showImport = ref(false)
const showPassword = ref(false)
let isOpenProxy = false
let downIndex = 0

onMounted(() => {
  try {
    loading.value = true
    handleInstall().then((is: boolean) => {
      loading.value = false
    })
  }catch (e) {
    window.$message?.error(JSON.stringify(e), {duration: 5000})
  }

  buildClassify()

  const temp = localStorage.getItem("resources-type")
  if (temp) {
    resourcesType.value = JSON.parse(temp).res
  } else {
    appApi.setType(resourcesType.value)
  }

  const cache = localStorage.getItem("resources-data")
  if (cache) {
    data.value = JSON.parse(cache)
  }

  eventStore.addHandle({
    type: "newResources",
    event: (res: appType.MediaInfo) => {
      data.value.push(res)
      localStorage.setItem("resources-data", JSON.stringify(data.value))
    }
  })

  eventStore.addHandle({
    type: "downloadProgress",
    event: (res: { Id: string, SavePath: string, Status: string, Message: string }) => {
      switch (res.Status) {
        case "running":
          loading.value = true
          loadingText.value = res.Message
          break
        case "done":
          setTimeout(()=>{
            loading.value = false
          }, 100)
          if (data.value[downIndex]?.Id === res.Id) {
            data.value[downIndex].SavePath = res.SavePath
            data.value[downIndex].Status = "done"
          } else {
            for (const i in data.value) {
              if (data.value[i].Id === res.Id) {
                data.value[i].SavePath = res.SavePath
                data.value[i].Status = "done"
                break
              }
            }
          }
          localStorage.setItem("resources-data", JSON.stringify(data.value))
          window?.$message?.success(t("index.download_success"))
          break
        case "error":
          setTimeout(()=>{
            loading.value = false
          }, 100)
          window?.$message?.error(res.Message)
          break
      }
    }
  })
})

watch(() => {
  return store.globalConfig.MimeMap
}, () => {
  buildClassify()
})

watch(resourcesType, (n, o) => {
  localStorage.setItem("resources-type", JSON.stringify({res: resourcesType.value}))
  appApi.setType(resourcesType.value)
})

const buildClassify = () => {
  const mimeMap = store.globalConfig.MimeMap ?? {}
  const seen = new Set()
  classify.value = [
    {value: "all", label: computed(() => t("index.all"))},
    ...Object.values(mimeMap)
        .filter(({Type}) => {
          if (seen.has(Type)) return false;
          seen.add(Type);
          return true;
        })
        .map(({Type}) => ({
          value: Type,
          label: classifyAlias[Type] ?? Type,
        })),
  ]
}

const dataAction = (row: appType.MediaInfo, index: number, type: string) => {
  switch (type) {
    case "down":
      download(row, index);
      break;
    case "copy":
      ClipboardSetText(row.Url).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "json":
      ClipboardSetText(encodeURIComponent(JSON.stringify(row))).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "open":
      BrowserOpenURL(row.Url)
      break;
    case "decode":
      decodeWxFile(row, index)
      break;
    case "delete":
      appApi.delete({sign: row.UrlSign}).then(() => {
        let arr = data.value
        arr.splice(index, 1);
        data.value = arr
        localStorage.setItem("resources-data", JSON.stringify(data.value))
      })
      break
  }
}

const renderToolbar = ({nodes}: ImageRenderToolbarProps) => {
  return [
    nodes.rotateCounterclockwise,
    nodes.rotateClockwise,
    nodes.resizeToOriginalSize,
    nodes.zoomOut,
    nodes.zoomIn,
    nodes.close
  ]
}

const rowKey = (row: appType.MediaInfo) => {
  return row.Id
}

const handleCheck = (rowKeys: DataTableRowKey[]) => {
  checkedRowKeysValue.value = rowKeys
}

const batchDown = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  for (let i = 0; i < data.value.length; i++) {
    if (checkedRowKeysValue.value.includes(data.value[i].Id) && data.value[i].Classify != "live" && data.value[i].Classify != "m3u8") {
      download(data.value[i], i)
      await checkVariable()
    }
  }
}

const batchImport = () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  loadingText.value = t("common.loading")
  loading.value = true
  let jsonData = []
  for (let i = 0; i < data.value.length; i++) {
    jsonData.push(encodeURIComponent(JSON.stringify(data.value[i])))
  }
  appApi.batchImport({content: jsonData.join("\n")}).then((res: appType.Res) => {
    loading.value = false
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t("index.import_success"))
    window?.$message?.info(t("index.save_path") + "：" + res.data?.file_name, {
      duration: 5000
    })
  })

}

const uint8ArrayToBase64 = (bytes: any) => {
  let binary = '';
  const len = bytes.byteLength;
  for (let i = 0; i < len; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return window.btoa(binary);
}

async function checkVariable() {
  return new Promise((resolve) => {
    const interval = setInterval(() => {
      if (!loading.value) {
        clearInterval(interval)
        resolve(true)
      }
    }, 600);
  });
}

const download = (row: appType.MediaInfo, index: number) => {
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  loadingText.value = "ready"
  loading.value = true
  downIndex = index
  if (row.DecodeKey) {
    appApi.download({
      ...row,
      decodeStr: uint8ArrayToBase64(getDecryptionArray(row.DecodeKey))
    }).then((res: appType.Res) => {
      if (res.code === 0) {
        loading.value = false
        window?.$message?.error(res.message)
      }
    })
  } else {
    appApi.download({...row, decodeStr: ""}).then((res: appType.Res) => {
      if (res.code === 0) {
        loading.value = false
        window?.$message?.error(res.message)
      }
    })
  }
}

const open = () => {
  isOpenProxy = true
  store.openProxy().then((res: appType.Res) => {
    if (res.code === 1) {
      return
    }

    if (["darwin", "linux"].includes(store.envInfo.platform)) {
      showPassword.value = true
    } else {
      window.$message?.error(res.message)
    }
  })
}

const close = () => {
  store.unsetProxy()
}

const clear = () => {
  data.value = []
  localStorage.setItem("resources-data", "")
  appApi.clear()
}

const decodeWxFile = (row: appType.MediaInfo, index: number) => {
  if (!row.DecodeKey) {
    window?.$message?.error(t("index.video_decode_no"))
    return
  }
  appApi.openFileDialog().then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    if (res.data.file) {
      loadingText.value = t("index.video_decode_loading")
      loading.value = true
      appApi.wxFileDecode({
        ...row,
        filename: res.data.file,
        decodeStr: uint8ArrayToBase64(getDecryptionArray(row.DecodeKey))
      }).then((res: appType.Res) => {
        loading.value = false
        if (res.code === 0) {
          window?.$message?.error(res.message)
          return
        }
        data.value[index].SavePath = res.data.save_path
        data.value[index].Status = "done"
        localStorage.setItem("resources-data", JSON.stringify(data.value))
        window?.$message?.success(t("index.video_decode_success"))
      })
    }
  })
}

const handleImport = (content: string) => {
  if (!content) {
    window?.$message?.error(t("view.import_empty"))
    return
  }
  content.split("\n").forEach((line, index) => {
    try {
      let res = JSON.parse(decodeURIComponent(line))
      if (res && res?.Id) {
        res.Id = res.Id + Math.floor(Math.random() * 100000)
        res.SavePath = ""
        res.Status = "ready"
        data.value.unshift(res)
      }
    } catch (e) {
      console.log(e)
    }
  });
  localStorage.setItem("resources-data", JSON.stringify(data.value))
  showImport.value = false
}

const handlePassword = async (password: string, isCache: boolean) => {
  const res = await appApi.setSystemPassword({password, isCache})
  if (res.code === 0) {
    window.$message?.error(res.message)
    return
  }

  if (isOpenProxy) {
    showPassword.value = false
    store.openProxy()
    return
  }

  handleInstall().then((is: boolean) => {
    if (is) {
      showPassword.value = false
    }
  })
}

const handleInstall = async () => {
  isOpenProxy = false
  const res = await appApi.install()
  if (res.code === 1) {
    store.globalConfig.AutoProxy && store.openProxy()
    return true
  }

  window.$message?.error(res.message, {duration: 5000})

  if (store.envInfo.platform === "windows" && res.message.includes("Access is denied")) {
    window.$message?.error(t("index.win_install_tip"))
  } else if (["darwin", "linux"].includes(store.envInfo.platform)) {
    showPassword.value = true
  }
  return false
}
</script>