import request from './request'

export function getPackageJson() {
    return request.get("https://github.com/putyy/res-downloader/raw/main/package.json")
}