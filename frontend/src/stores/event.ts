import {defineStore} from "pinia"
import {EventsOn} from "../../wailsjs/runtime"
import {appType} from "@/types/app"

export const useEventStore = defineStore('ws-store', () => {
    const handles = new Map<string, Set<(data: any) => void>>()
    let initialized = false

    const init = () => {
        if (initialized) return
        initialized = true
        EventsOn("event", (res: any) => {
            const data = JSON.parse(res)
            for (const handler of handles.get(data.type) ?? []) {
                handler(data.data)
            }
        })
    }

    const addHandle = (handle: appType.Handle) => {
        const listeners = handles.get(handle.type) ?? new Set()
        listeners.add(handle.event)
        handles.set(handle.type, listeners)
        return () => {
            listeners.delete(handle.event)
            if (listeners.size === 0) handles.delete(handle.type)
        }
    }

    return {
        init, addHandle
    }
})