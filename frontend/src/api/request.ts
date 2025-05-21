import type {AxiosResponse, InternalAxiosRequestConfig} from 'axios'
import axios from 'axios'
import {useIndexStore} from "@/stores";
import {computed} from "vue";

interface RequestOptions {
    url: string
    method: 'get' | 'post' | 'put' | 'delete'
    params?: Record<string, any>
    data?: Record<string, any>
}

const instance = axios.create({
    baseURL: "/",
    timeout: 180000
})

instance.interceptors.request.use(
    (config: InternalAxiosRequestConfig<any>) => {
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

const request = ({url, method, params, data}: RequestOptions): Promise<any> => {
    return instance({url, method, params, data, baseURL: window.$baseUrl})
}

export default request