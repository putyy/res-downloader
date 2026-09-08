<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      preset="card"
      class="w-[640px] [--wails-draggable:no-drag]"
      :title="t('index.batch_import')"
  >
    <NForm
        size="medium"
        label-placement="left"
        label-width="auto"
        require-mark-placement="right-hanging"
        class="[--wails-draggable:no-drag]"
    >
      <NFormItem>
        <NInput type="textarea" v-model:value="content" rows="8" :autosize="false" :placeholder="t('index.import_placeholder')"></NInput>
      </NFormItem>
      <NFormItem>
        <NButton strong secondary type="success" @click="emits('submit', content)" class="w-20">{{ t('common.submit') }}</NButton>
      </NFormItem>
    </NForm>
  </NModal>
</template>
<script setup lang="ts">
import {ref} from "vue"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const content = ref("")
const props = defineProps<{
  showModal: boolean
}>()

const emits = defineEmits(["update:showModal", "submit"])
const changeShow = (value: boolean) => emits("update:showModal", value)
</script>
