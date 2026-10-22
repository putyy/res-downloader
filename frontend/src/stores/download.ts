import {defineStore} from "pinia"
import {computed, reactive, ref} from "vue"
import appApi from "@/api/app"
import type {appType} from "@/types/app"
import {useEventStore} from "@/stores/event"
import {isActiveTask} from "@/components/task/taskState"

type Task = appType.DownloadTaskRecord
const compareTasks = (left: Task, right: Task) => right.createdAt - left.createdAt || left.id.localeCompare(right.id)

export const useDownloadStore = defineStore("download-store", () => {
  const records = reactive(new Map<string, Task>())
  const orderedIds = ref<string[]>([])
  const creating = reactive(new Set<string>())
  const loading = ref(false)
  let loaded = false
  let loadingRequest: Promise<void> | undefined
  // Preserve updates and removals received while a list snapshot is in flight.
  let duringLoad: Map<string, Task | null> | undefined

  const tasks = computed(() => orderedIds.value.map((id) => records.get(id)!).filter(Boolean))
  const resourceTasks = computed(() => {
    const latest = new Map<string, Task>()
    for (const task of tasks.value) {
      const current = latest.get(task.resourceId)
      // Retrying or resuming an older task makes it the resource's active owner.
      if (!current || (!isActiveTask(current) && isActiveTask(task))) latest.set(task.resourceId, task)
    }
    return latest
  })
  const upsert = (task: Task, fromEvent = false) => {
    const previous = records.get(task.id)
    // Events can advance twice in one millisecond; equal-time HTTP snapshots
    // must not replace a state already received through the event stream.
    if (previous && (previous.updatedAt > task.updatedAt || (!fromEvent && previous.updatedAt === task.updatedAt))) return
    duringLoad?.set(task.id, task)
    records.set(task.id, task)
    if (!previous || previous.createdAt !== task.createdAt) {
      orderedIds.value = [...records.values()].sort(compareTasks).map((item) => item.id)
    }
  }
  const remove = (id: string) => {
    duringLoad?.set(id, null)
    records.delete(id)
    orderedIds.value = orderedIds.value.filter((current) => current !== id)
  }
  const load = (refresh = false): Promise<void> => {
    if (loadingRequest) return loadingRequest
    if (loaded && !refresh) return Promise.resolve()
    loading.value = true
    const changes = new Map<string, Task | null>()
    duringLoad = changes
    loadingRequest = (async () => {
      try {
        const response: appType.Res<Task[]> = await appApi.downloadTasks()
        if (response.code !== 1) throw new Error(response.message)
        const snapshot = new Map((response.data ?? []).map((task) => [task.id, task]))
        for (const [id, task] of records) {
          const incoming = snapshot.get(id)
          if (incoming && incoming.updatedAt <= task.updatedAt) snapshot.set(id, task)
        }
        for (const [id, task] of changes) {
          if (!task) snapshot.delete(id)
          else if (!snapshot.has(id) || snapshot.get(id)!.updatedAt <= task.updatedAt) snapshot.set(id, task)
        }
        records.clear()
        for (const [id, task] of snapshot) records.set(id, task)
        orderedIds.value = [...snapshot.values()].sort(compareTasks).map((task) => task.id)
        loaded = true
      } finally {
        duringLoad = undefined
        loading.value = false
        loadingRequest = undefined
      }
    })()
    return loadingRequest
  }
  const create = async (resourceId: string) => {
    if (creating.has(resourceId)) return
    creating.add(resourceId)
    try {
      const response: appType.Res<Task> = await appApi.createDownload({id: resourceId})
      if (response.code !== 1) throw new Error(response.message)
      upsert(response.data)
    } finally {
      creating.delete(resourceId)
    }
  }

  // The application store owns one subscription for both kept-alive pages.
  const events = useEventStore()
  events.addHandle({type: "downloadTaskUpdated", event: (task: Task) => upsert(task, true)})
  events.addHandle({type: "downloadTaskRemoved", event: (event: {id: string}) => remove(event.id)})

  return {tasks, resourceTasks, creating, loading, load, create, upsert, remove}
})
