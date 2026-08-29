import type {AxiosResponse, InternalAxiosRequestConfig} from 'axios'
import axios from 'axios'

interface RequestOptions {
    url: string
    method: 'get' | 'post' | 'put' | 'delete'
    params?: Record<string, any>
    data?: Record<string, any>
    timeout?: number
}

const instance = axios.create({
    baseURL: "/",
    timeout: 180000
})

instance.interceptors.request.use(
    (config: InternalAxiosRequestConfig<any>) => {
        if (window.$apiToken) {
            config.headers.set('Authorization', `Bearer ${window.$apiToken}`)
        }
        return config
    },
    (error) => {
        return Promise.reject(error)
    }
)

instance.interceptors.response.use(
    (response: AxiosResponse) => {
        return response.data
    },
    (error) => {
        return Promise.reject(error);
    }
)

const request = ({url, method, params, data, timeout}: RequestOptions): Promise<any> => {
    return instance({url, method, params, data, timeout, baseURL: window.$baseUrl})
}

export default request
