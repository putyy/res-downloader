import {defineStore} from 'pinia'
import {ref} from 'vue'

export const useUpdateStore = defineStore('update-ui', () => {
    const openRequest = ref(0)
    const openDialog = () => { openRequest.value++ }
    return {openRequest, openDialog}
})
