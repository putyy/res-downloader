import {createMemoryHistory, createRouter} from 'vue-router'
// @ts-ignore
import localStorageCache from "./common/localStorage"

const routes = [
    {
        path: '/',
        component: () => import('./components/layout/Index.vue'),
        // 重定向
        redirect: {name: 'Index'},
        // 子路由
        children: [
            {
                path: '/index',
                name: 'Index',
                meta: {keepAlive: true},
                component: () => import('./views/Index.vue'),
            },
            {
                path: '/about',
                name: 'about',
                meta: {keepAlive: true},
                component: () => import('./views/About.vue'),
            },
            {
                path: '/setting',
                name: 'Setting',
                meta: {keepAlive: true},
                component: () => import('./views/Setting.vue'),
            },
        ]
    },
]

const router = createRouter({
    history: createMemoryHistory(),
    routes: routes,
})

router.beforeEach((to, from, next) => {
    next()
})

export default router
