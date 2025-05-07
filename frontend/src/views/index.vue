<template>
  <div class="flex flex-col p-5">
    <div class="pb-2 z-40">
      <NSpace>
        <NButton v-if="isProxy" secondary type="primary" @click.stop="close" style="--wails-draggable:no-drag">关闭代理</NButton>
        <NButton v-else tertiary type="tertiary" @click.stop="open" style="--wails-draggable:no-drag">开启代理</NButton>
        <NButton tertiary type="error" @click.stop="clear" style="--wails-draggable:no-drag">清空列表</NButton>
        <NSelect style="min-width: 100px;--wails-draggable:no-drag" placeholder="拦截类型" v-model:value="resourcesType" multiple clearable :max-tag-count="3" :options="classify"></NSelect>
        <NButton tertiary type="info" @click.stop="batchDown" style="--wails-draggable:no-drag">批量下载</NButton>
        <NButton tertiary type="info" @click.stop="batchImport" style="--wails-draggable:no-drag">批量导出</NButton>
        <NButton tertiary type="info" @click.stop="showImport=true" style="--wails-draggable:no-drag">批量导入</NButton>
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
import {computed, h, onMounted, ref, watch} from "vue"
import type {appType} from "@/types/app"

import type {DataTableRowKey, ImageRenderToolbarProps} from "naive-ui"
import Preview from "@/components/Preview.vue"
import ShowLoading from "@/components/ShowLoading.vue"
// @ts-ignore
import {getDecryptionArray} from '@/assets/js/decrypt.js'
import {useIndexStore} from "@/stores"
import appApi from "@/api/app"
import {DwStatus} from "@/const"
import ResAction from "@/components/ResAction.vue"
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {BrowserOpenURL, ClipboardSetText} from "../../wailsjs/runtime"
import Password from "@/components/Password.vue"

const eventStore = useEventStore()
const isProxy = computed(() => {
  return store.isProxy
})
const data = ref<any[]>([])
const store = useIndexStore()
const tableHeight = computed(() => {
  return store.tableHeight - 132
})
const resourcesType = ref<string[]>(["all"])
const classifyAlias: {[key: string]: string} = {
  image: "图片",
  audio: "音频",
  video: "视频",
  m3u8: "m3u8",
  live: "直播流",
  xls: "表格",
  doc: "文档",
  pdf: "pdf",
  font: "字体"
}
const classify = ref([
  {
    value: "all",
    label: "全部",
  },
])

const columns = ref<any[]>([
  {
    type: "selection",
  },
  {
    title: "域",
    key: "Domain",
  },
  {
    title: "类型",
    key: "Classify",
    filterOptions: Array.from(classify.value).slice(1),
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
    title: "预览",
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
                  return "预览"
                }
                return "暂不支持预览"
              }
            }
        ),
      ]
    }
  },
  {
    title: "状态",
    key: "Status",
    render: (row: appType.MediaInfo) => {
      return DwStatus[row.Status as keyof typeof DwStatus]
    }
  },
  {
    title: "描述",
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
    title: "资源大小",
    key: "Size"
  },
  {
    title: "保存路径",
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
    title: "操作",
    key: "actions",
    render(row: appType.MediaInfo, index: number) {
      return h(ResAction, {key: index, row: row, index: index, onAction: dataAction})
    }
  }
])
const downIndex = ref(0)
const checkedRowKeysValue = ref<DataTableRowKey[]>([])
const showPreviewRow = ref(false)
const previewRow = ref<appType.MediaInfo>()
const loading = ref(false)
const loadingText = ref("")
const showImport = ref(false)
const showPassword = ref(false)

onMounted(() => {
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
          break;
        case "done":
          loading.value = false
          if (data.value[downIndex.value]?.Id === res.Id) {
            data.value[downIndex.value].SavePath = res.SavePath
            data.value[downIndex.value].Status = "done"
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
          window?.$message?.success("下载成功")
          break;
        case "error":
          loading.value = false
          window?.$message?.error(res.Message)
          break;
      }
    }
  })
})

watch(()=>{
  return store.globalConfig.MimeMap
}, ()=>{
  buildClassify()
})

watch(resourcesType, (n, o) => {
  localStorage.setItem("resources-type", JSON.stringify({res: resourcesType.value}))
  appApi.setType(resourcesType.value)
})

const buildClassify = ()=>{
  const mimeMap = store.globalConfig.MimeMap ?? {}
  const seen = new Set()
  classify.value = [
    {value: "all", label: "全部"},
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
          window?.$message?.success("复制成功")
        } else {
          window?.$message?.error("复制失败")
        }
      })
      break
    case "json":
      ClipboardSetText(encodeURIComponent(JSON.stringify(row))).then((is: boolean) => {
        if (is) {
          window?.$message?.success("复制成功")
        } else {
          window?.$message?.error("复制失败")
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
    window?.$message?.error("请设置保存位置")
    return
  }
  for (let i = 0; i < data.value.length; i++) {
    if (checkedRowKeysValue.value.includes(data.value[i].Id) && data.value[i].Classify != "live" && data.value[i].Classify != "m3u8") {
      download(data.value[i], i)
      await checkVariable()
    }
  }
}

const batchImport = ()=>{
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error('请选择需要导出的数据')
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error("请设置保存目录")
    return
  }
  loadingText.value = "导出中"
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
    window?.$message?.success("导出成功")
    window?.$message?.info("文件路径：" + res.data?.file_name, {
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
    window?.$message?.error("请设置保存位置")
    return
  }
  loadingText.value = "ready"
  loading.value = true
  downIndex.value = index
  if (row.DecodeKey) {
    appApi.download({...row, decodeStr: uint8ArrayToBase64(getDecryptionArray(row.DecodeKey))}).then((res: appType.Res) => {
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
  appApi.openSystemProxy().then((res: appType.Res) => {
    if (res.code === 0 ){
      if (store.envInfo.platform === "darwin") {
        showPassword.value = true
        return
      }
      window?.$message?.error(res.message)
      return
    }
    store.updateProxyStatus(res.data)
  })
}

const close = () => {
  appApi.unsetSystemProxy().then((res: appType.Res) => {
    if (res.code === 0 ){
      window?.$message?.error(res.message)
      return
    }
    store.updateProxyStatus(res.data)
  })
}

const clear = () => {
  data.value = []
  localStorage.setItem("resources-data", "")
  appApi.clear()
}

const decodeWxFile = (row: appType.MediaInfo, index: number) => {
  if (!row.DecodeKey) {
    window?.$message?.error("无法解密")
    return
  }
  appApi.openFileDialog().then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    if (res.data.file) {
      loadingText.value = "解密中"
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
        window?.$message?.success("解密成功")
      })
    }
  })
}

const handleImport = (content: string)=>{
  content.split("\n").forEach((line, index) => {
    try {
      let res = JSON.parse(decodeURIComponent(line))
      if (res && res?.Id) {
        res.Id = res.Id + Math.floor(Math.random() * 100000)
        res.SavePath = ""
        res.Status = "ready"
        data.value.unshift(res)
      }
    }catch (e) {
      console.log(e)
    }
  });
  localStorage.setItem("resources-data", JSON.stringify(data.value))
  showImport.value = false
}

const handlePassword = (password: string)=>{
  appApi.setSystemPassword({password: password}).then((res: appType.Res)=>{
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    open()
  })
}
</script>