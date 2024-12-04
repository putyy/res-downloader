## res-downloader
### 爱享素材下载器【[加入群聊](https://qm.qq.com/q/mfDMSpCxQ4)】
🎯 基于 [electron-vite-vue](https://github.com/electron-vite/electron-vite-vue.git)  
📦 操作简单、可获取不同类型的资源  
🖥️ 支持Windows、Mac、Linux  
🌐 支持视频、音频、图片、m3u8、直播流等常见网络资源  
💪 支持微信视频号、小程序、抖音、快手、小红书、酷狗音乐、qq音乐等网络资源下载  
👼 支持设置代理以获取特殊网络下的资源  

## 软件下载
🆕 [github下载](https://github.com/putyy/res-downloader/releases)  
🆕 [蓝奏云下载 密码:9vs5](https://wwjv.lanzoum.com/b04wgtfyb)

## 使用方法
> 0. 安装时一定要同意安装证书文件、使用时一定要允许网络访问
> 1. 打开本软件 软件首页左上角点击 “启动代理”
> 2. 软件首页选择要获取的资源类型（默认选中的全部）
> 3. 打开要捕获的源， 如：视频号、网页、小程序等等
> 4. 返回软件首页即可看到资源列表

## 软件截图
![](public/show.webp)

## 常见问题
m3u8: 预览和下载：
> [下载](https://m3u8-down.gowas.cn/) [预览](https://m3u8play.com/)

直播流: 预览和录制：
> [使用obs进行预览和录制 使用教程自行百度]( https://obsproject.com/)

下载慢、大视频下载失败(最新版本已内置aria2下载器)
> 推荐使用如下工具加速下载，视频号可以下载完成后再到对应视频操作项选择 “视频解密(视频号)” 按钮
>> [Neat Download Manager](https://www.neatdownloadmanager.com/index.php/en/)、[Motrix](https://motrix.app/download)等软件进行下载

打开本软件，无法正常拦截获取
> 检查系统代理是否正确设置 代理地址：127.0.0.1 端口：8899

关闭软件后无法正常上网
> 手动关闭系统代理设置

打开本软件后无法上网
> 手动删除安装标识锁文件，之后再打开软件会进行检查证书是否正确安装
>> MAC: /Users/你的用户名称/.res-downloader@putyy/res-downloader-installed.lock  
> 
>> Win: C:\Users\Admin\.res-downloader@putyy/res-downloader-installed.lock

其他问题  
[github](https://github.com/putyy/res-downloader/issues)  、 [爱享论坛](https://s.gowas.cn/d/4089)  

## 实现 & 初衷
通过代理网络抓包拦截响应，筛选出有用的资源， 同fiddler、charles等抓包软件、浏览器F12打开控制也能达到目的，只不过这些软件需要手动进行筛选，对于小白用户上手还是有点难度，本软件对部分资源做了特殊处理，更适合大众用户，所以就有了本项目。

## 免责声明
本软件用于学习研究使用，若因使用本软件造成的一切法律责任均与本人无关！
