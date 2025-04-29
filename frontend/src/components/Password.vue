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
      class="rounded-lg"
  >
    <n-form>
      <div class="text-red-500 text-base">
        本操作需要管理员授权，仅对本次运行期间有效！
      </div>
      <n-form-item path="password" label="">
        <n-input
            v-model:value="password"
            type="password"
            placeholder="请输入你的电脑密码"
            class="w-full"
        />
      </n-form-item>
    </n-form>
    <template #action>
      <div class="flex justify-end gap-4">
        <n-button @click="emits('update:showModal', false)">取消操作</n-button>
        <n-button type="primary" @click="emits('submit', password)">确认操作</n-button>
      </div>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import {ref, computed} from 'vue'
import {NModal, NForm, NFormItem, NInput, NButton} from 'naive-ui'

const props = defineProps({
  showModal: Boolean,
})
const password = ref("")

const emits = defineEmits(["update:showModal", "submit"])
const changeShow = (value: boolean) => emits("update:showModal", value)
</script>