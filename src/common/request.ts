import axios from 'axios'
import {ElMessage} from 'element-plus'
import {hexMD5} from "./md5"
import localStorageCache from "./localStorage"
import _ from "lodash"

class RequestService {
    private axios: any;
    private requestList: any;
    constructor() {
        let that = this
        that.requestList = []
        that.axios = axios.create({
            timeout: 60000, // 请求超时时间毫秒
        })

        // 请求拦截
        that.axios.interceptors.request.use(
            function (config: any) {
                if (config.url.slice(0, 8) !== "https://") {
                    config.url = import.meta.env.VITE_APP_API + "/" + config.url
                }
                return config
            },
            function (error: any) {
                return Promise.reject(error)
            }
        )

        // 响应拦截
        that.axios.interceptors.response.use(
            function (response: any) {
                return response
            },
            function (error: any) {
                // console.log(error)
                return Promise.reject(error)
            }
        )
    }


    get(url: string, data?: any) {
        return this.axios.get(url, {params: data}).catch((err:any)=>{
            console.log('get-err', err)
        })
    }

    post(url: string, data: any, isHandle?: any) {
        isHandle = isHandle || true
        if (isHandle){
            data = Object.keys(data).map(item => {
                let value = data[item];
                if (_.isArray(value) || _.isObject(value)) {
                    value = JSON.stringify(value)
                }
                return encodeURIComponent(item) + '=' + encodeURIComponent(value)
            }).join('&');
        }
        return this.axios.post(url, data).catch((err:any)=>{
            console.log('post-err', err)
        })
    }

    axiosObj() {
        return this.axios
    }
}

const request = new RequestService()
export default request
