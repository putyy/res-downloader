<template>
  <n-modal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag"
      preset="dialog"
      :title="t('components.password_title')"
      content=""
      :show-icon="false"
      :mask-closable="false"
      :close-on-esc="false"
      class="rounded-lg"
  >
    <div>
      <div class="text-red-500 text-base">
        {{ t("components.password_tip") }}
      </div>
      <div class="mt-3">
        <n-input
            v-model:value="formValue.password"
            type="password"
            :placeholder="t('components.password_placeholder')"
            class="w-full"
        />
      </div>
      <div class="mt-3 text-base">
        <label>{{ t("components.password_cache") }}</label>
        <NSwitch class="pl-1" v-model:value="formValue.cache" :aria-placeholder="t('components.password_cache')"/>
      </div>
    </div>
    <template #action>
      <n-button type="primary" @click="submit">{{ t("common.submit") }}</n-button>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import {reactive} from 'vue'
import {NButton, NInput, NModal} from 'naive-ui'
import {useI18n} from 'vue-i18n'

const {t} = useI18n()

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
    window.$message?.error(t("components.password_empty"))
    return
  }
  emits('submit', formValue.password, formValue.cache)
}
</script>