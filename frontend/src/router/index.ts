import {createRouter, createWebHashHistory} from 'vue-router'

const routes = [
    {
        path: "/",
        name: "layout",
        component: () => import("@/components/layout/Index.vue"),
        redirect: "/index",
        children: [
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
        ]
    },
]

const router = createRouter({
    history: createWebHashHistory(),
    routes
})

export default router
