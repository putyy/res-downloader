<script setup lang="ts">
import {ref, onMounted} from "vue"
import {ipcRenderer} from 'electron'
import {onUnmounted} from "@vue/runtime-core"
import {ElMessage} from "element-plus"
import localStorageCache from "../common/localStorage"
import {ElLoading} from 'element-plus'

const tableData = ref<{
  url_sign: string,
  url: string,
  size: any,
  platform: string,
  progress_bar: any,
  save_path: string,
  downing: boolean
}[]>([])

const resType = ref({
  video: true,
  audio: true,
  image: false,
  m3u8: false
})


const isInitApp = ref(false)

const toSize = (size: number) => {
  if (size > 1048576) {
    return (size / 1048576).toFixed(2) + "MB"
  }
  if (size > 1024) {
    return (size / 1024).toFixed(2) + "KB"
  }
  return size + 'b'
}

onMounted(() => {
  let resTypeCache = localStorageCache.get("res-type")
  if (resTypeCache) {
    resType.value = resTypeCache
  }

  let tableDataCache = localStorageCache.get("res-table-data")
  if (tableDataCache) {
    tableData.value = tableDataCache
  }

  ipcRenderer.invoke('invoke_app_is_init').then((isInit: boolean) => {
    if (!isInit) {
      isInitApp.value = true
      ipcRenderer.invoke('invoke_init_app')
    }
  })

  const loading = ElLoading.service({
    lock: true,
    text: 'Loading',
    background: 'rgba(0, 0, 0, 0.7)',
  })

  ipcRenderer.invoke('invoke_start_proxy').then(() => {
    loading.close()
    ipcRenderer.on('on_get_queue', (res, data) => {
      // @ts-ignore
      if (resType.value.hasOwnProperty(data.type_str) && resType.value[data.type_str]) {
        tableData.value.push(data)
        localStorageCache.set("res-table-data", tableData.value, -1)
      }
      return
    })
  }).catch((err) => {
    // console.log('invoke_start_proxy err', err)
    ElMessage({
      message: err,
      type: 'warning',
    })
    loading.close()
  })
})

onUnmounted(() => {
  ipcRenderer.removeListener('on_get_queue', (res) => {
    // console.log(res)
  })

  ipcRenderer.invoke('invoke_close_proxy').then((res) => {
  })
  localStorageCache.set("res-table-data", tableData.value, -1)
  localStorageCache.set("res-type", resType.value, -1)
})


const handleDown = async (index: number, row: any, high: boolean) => {

  let save_dir = localStorageCache.get("save_dir")

  if (!save_dir) {
    ElMessage({
      message: '请设置保存目录',
      type: 'warning'
    })
    return
  }

  let loading = ElLoading.service({
    lock: true,
    text: 'Loading',
    background: 'rgba(0, 0, 0, 0.7)',
  })

  let result = await ipcRenderer.invoke('invoke_file_exists', {
    save_path: save_dir,
    url: (high && row.high_url) ? row.high_url : row.url,
  })

  if (result.is_file) {
    tableData.value[index].progress_bar = "100%"
    tableData.value[index].save_path = result.fileName
    ElMessage({
      message: "文件已存在(" + result.fileName + ")",
      type: 'warning',
    })
    loading.close()
    return
  }

  ipcRenderer.invoke('invoke_down_file', {
    index: index,
    data: Object.assign({}, tableData.value[index]),
    save_path: save_dir,
    high: high
  }).then((res) => {
    if (res !== false) {
      tableData.value[index].progress_bar = "100%"
      tableData.value[index].save_path = res.fullFileName
    }else{
      ElMessage({
        message: "下载失败",
        type: 'warning',
      })
    }
    loading.close()
  }).catch((err) => {
    // console.log('invoke_down_file err', err)
    ElMessage({
      message: "下载失败",
      type: 'warning',
    })
    loading.close()
  })

}

const handlePreview = (index: number, row: any) => {
  // console.log('row.down_url',row)
  ipcRenderer.invoke('invoke_resources_preview', {url: row.down_url}).catch(() => {
  })
}

const handleClear = () => {
  tableData.value = []
  localStorageCache.del("res-table-data")
}

const handleCopy = (text: string) => {
    let el = document.createElement('input')
    el.setAttribute('value', text)
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    ElMessage({
      message: "复制成功",
      type: 'success',
    })
}

const handleDel = (index: number)=>{
  let arr = tableData.value
  arr.splice(index, 1);
  tableData.value = arr
}

const openDir = ()=>{
  let save_dir = localStorageCache.get("save_dir")

  if (!save_dir) {
    ElMessage({
      message: '目录不存在',
      type: 'warning'
    })
    return
  }

  ipcRenderer.invoke('invoke_open_dir', {
    dir: save_dir
  })
}

const handleInitApp = () => {
  ipcRenderer.invoke('invoke_app_is_init').then((isInit: boolean) => {
    if (isInit) {
      isInitApp.value = false
    } else {
      ipcRenderer.invoke('invoke_init_app')
    }
  })
}

</script>

<template lang="pug">
el-container.container
  el-header
    el-row
      div
        el-button(v-if="isInitApp" @click="handleInitApp")
          el-icon
            Promotion
          p 安装检测(如果看到此按钮说明软件安装未完成则需要手动点击此按钮)
        el-button(@click="handleClear")
          el-icon
            Delete
          p 清空
        el-button(@click="resType.video=!resType.video" :type="resType.video ? 'primary' : 'info'" ) 视频
        el-button(@click="resType.audio=!resType.audio" :type="resType.audio ? 'primary' : 'info'" ) 音频
        el-button(@click="resType.image=!resType.image" :type="resType.image ? 'primary' : 'info'" ) 图片
        el-button(@click="resType.m3u8=!resType.m3u8" :type="resType.m3u8 ? 'primary' : 'info'" ) m3u8
        a(style="color: red") &nbsp;&nbsp;&nbsp;点击左边选项，选择需要拦截的资源类型
  el-main
    el-table(:data="tableData" max-height="100%" stripe style="max-content")
      el-table-column(label="预览" show-overflow-tooltip width="350px" )
        template(#default="scope" )
          div.show_res
            video(v-if="scope.row.type_str === 'video'" :src="scope.row.down_url" controls preload="none" style="width: 100%;height: auto;") 您的浏览器不支持 video 标签。
            img.img(v-if="scope.row.type_str === 'image'" :src="scope.row.down_url")
            audio(v-if="scope.row.type_str === 'audio'" controls preload="none")
              source(:src="scope.row.down_url" :type="scope.row.type")
      el-table-column(prop="type_str" label="类型" show-overflow-tooltip)
      el-table-column(prop="platform" label="主机地址")
      el-table-column(prop="size" label="资源大小")
      el-table-column(prop="save_path" label="保存目录")
      el-table-column(label="操作")
        template(#default="scope")
          template(v-if="scope.row.type_str !== 'm3u8'" )
            el-button(v-if="!scope.row.save_path" link type="primary" @click="handleDown(scope.$index, scope.row, false)") 下载
            el-button(v-if="!scope.row.save_path && scope.row.high_url !='' " link type="primary" @click="handleDown(scope.$index, scope.row, true)") 高清下载
            el-button(link type="primary" @click="handlePreview(scope.$index, scope.row)") 窗口预览
          el-button(link type="primary" @click="handleCopy(scope.row.down_url)") 复制链接
          el-button(link type="primary" @click="handleDel(scope.$index)") 删 除
          el-button(link type="primary" @click="openDir()") 打开目录
</template>

<style scoped lang="less">
.container {
  padding: 0.5rem;

  .el-button {
    margin: 0.1rem;
  }

  .el-button p {
    padding-left: 0.2rem;
  }

  .el-row {
    display: flex;
    justify-content: space-between;
  }

  header {
    height: unset !important;
  }

  .el-form-item {
    display: flex;
    justify-content: center;
    align-items: center;

    .select-dir {
      background: #fff5f5;
    }
  }

  .show_res{
    .img{
      width: 100px;
      height: auto;
      max-height: 200px;
    }
  }
}
</style>
