<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag"
      preset="card"
      class="w-[640px]"
      title="导入数据"
  >
    <NForm
        size="medium"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        style="--wails-draggable:no-drag"
    >
      <NFormItem>
        <NInput type="textarea" v-model:value="content" rows="8" :autosize="false" placeholder="添加多个时，请确保每行只有一个(每个链接回车换行)"></NInput>
      </NFormItem>
      <NFormItem>
        <NButton strong secondary type="success" @click="emits('submit', content)" class="w-20">提交</NButton>
      </NFormItem>
    </NForm>
  </NModal>
</template>
<script setup lang="ts">
import {ref} from "vue"

const content = ref("")
const props = defineProps<{
  showModal: boolean
}>()

const emits = defineEmits(["update:showModal", "submit"])
const changeShow = (value: boolean) => emits("update:showModal", value)
</script>