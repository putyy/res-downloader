<template>
  <n-modal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag"
      preset="dialog"
      title="管理员授权"
      content=""
      :show-icon="false"
      :mask-closable="false"
      :close-on-esc="false"
      class="rounded-lg"
  >
    <div>
      <div class="text-red-500 text-base">
        本次输入的密码仅在本次运行期间有效，用于安装证书或设置系统代理！
      </div>
      <div class="mt-3">
        <n-input
            v-model:value="formValue.password"
            type="password"
            placeholder="请输入你的电脑密码"
            class="w-full"
        />
      </div>
      <div class="mt-3 text-base">
        <label>是否缓存</label>
        <NSwitch class="pl-1" v-model:value="formValue.cache" aria-placeholder="是否缓存"/>
      </div>
    </div>
    <template #action>
      <n-button type="primary" @click="submit">确认</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import {reactive} from 'vue'
import {NButton, NInput, NModal} from 'naive-ui'

defineProps({
  showModal: Boolean,
})

const formValue = reactive({
  password: "",
  cache: false,
})

const emits = defineEmits(["update:showModal", "submit"])
const changeShow = (value: boolean) => emits("update:showModal", value)

const submit = () => {
  if (!formValue.password) {
    window.$message?.error("密码不能为空")
    return
  }
  emits('submit', formValue.password, formValue.cache)
}
</script>