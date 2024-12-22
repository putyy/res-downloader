import {defineStore} from "pinia"
import {ref} from "vue"
import type {wsType} from "@/types/ws"
import {EventsOn} from "../../wailsjs/runtime"

export const useEventStore = defineStore('ws-store', () => {
    const handles = ref<any>({})

    const init = ()=>{
        EventsOn("event", (res: any) => {
            const data = JSON.parse(res)
            if (handles.value.hasOwnProperty(data.type)) {
                handles.value[data.type](data.data)
            } else {
                console.log("找不到该类型的消息处理器")
            }
        })
    }

    const addHandle = (handle: wsType.Handle) => {
        handles.value[handle.type] = handle.event
    }

    return {
        init, addHandle
    }
})