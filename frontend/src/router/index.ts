import {createRouter, createWebHashHistory} from "vue-router"

const routes = [
  {
    path: "/",
    name: "layout",
    component: () => import("@/components/layout/Index.vue"),
    redirect: "/index",
    children: [
      {
        path: "/operations",
        name: "operations",
        meta: {keepAlive: false},
        component: () => import("@/views/operations.vue"),
      },
      {
        path: "/index",
        name: "index",
        meta: {keepAlive: true},
        component: () => import("@/views/index.vue"),
      },
      {
        path: "/plugins",
        name: "plugins",
        meta: {keepAlive: true},
        component: () => import("@/views/plugins.vue"),
      },
      {
        path: "/tasks",
        name: "tasks",
        meta: {keepAlive: true},
        component: () => import("@/views/tasks.vue"),
      },
      {
        path: "/setting",
        name: "setting",
        meta: {keepAlive: false},
        component: () => import("@/views/setting.vue"),
      },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// Observe navigation before app.use(router), including lazy component failures.
const initialNavigation = router.isReady().then(
  () => true,
  () => false,
)

export const waitForInitialRoute = async () => {
  let timeout: ReturnType<typeof setTimeout> | undefined
  try {
    await Promise.race([
      initialNavigation.then(async (ready) => {
        if (!ready) await router.replace(router.options.history.location || "/")
      }),
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => reject(new Error("Initial page loading timed out")), 10000)
      }),
    ])
  } finally {
    clearTimeout(timeout)
  }
}

export default router
