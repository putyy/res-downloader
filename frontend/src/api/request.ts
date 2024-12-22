import axios from 'axios';
import type {AxiosResponse, InternalAxiosRequestConfig} from 'axios';

interface RequestOptions {
    url: string;
    method: 'get' | 'post' | 'put' | 'delete'; // 根据需要扩展
    params?: Record<string, any>;
    data?: Record<string, any>;
}

const instance = axios.create({
    baseURL: "/",
});

instance.interceptors.request.use(
    (config: InternalAxiosRequestConfig<any>) => {
        return config;
    },
    (error) => {
        return Promise.reject(error);
    }
);

instance.interceptors.response.use(
    (response: AxiosResponse) => {
        return response.data;
    },
    (error) => {
        return Promise.reject(error);
    }
);

const request = ({url, method, params, data}: RequestOptions): Promise<any> => {
    return instance({url, method, params, data});
};

export default request;