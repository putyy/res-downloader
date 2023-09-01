const localStorageCache = {
    set: function set(key: string, value: any, time?: number) {
        if (!localStorage) {
            return false
        }
        if (!time || isNaN(time)) {
            time = 1200
        }
        try {
            let expireDate =  -1
            if (time !== -1) {
                // @ts-ignore
                expireDate = (new Date() - 1) + time * 1000
            }
            localStorage.setItem(key, JSON.stringify({val: value, exp: expireDate}))
        } catch (e) {
        }
        return true
    },
    get: function get(key: string) {
        try {
            if (!localStorage) {
                return null
            }
            let value = localStorage.getItem(key)
            // @ts-ignore
            let result = JSON.parse(value)
            // @ts-ignore
            let now = new Date() - 1
            if (!result) {
                return null
            }// 缓存不存在
            if (result.exp !== -1 && now > result.exp) {
                // 缓存过期
                this.del(key)
                return null
            }
            return result.val
        } catch (e) {
            this.del(key)
            return null
        }
    },
    del: function del(key: string) {
        if (!localStorage) {
            return false
        }
        localStorage.removeItem(key)
        return true
    },
    // 清空所有缓存
    delAll: function delAll() {
        if (!localStorage) {
            return false
        }
        localStorage.clear()
        return true
    }
}

export default localStorageCache
