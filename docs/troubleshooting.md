## 视频号拦截了一大堆 找不到想要的
> 设置里面关闭全量拦截，将视频转发好友后打开

## 某某网址拦截不了？
> 本软件实现原理 & 初衷如下，并非万能的，所以有一些应用拦截不了很正常
```
本工具通过代理方式实现网络抓包，并筛选可用资源。与 Fiddler、Charles、浏览器 DevTools 原理类似，但对资源进行了更友好的筛选、展示和处理，大幅度降低了使用门槛，更适合大众用户使用。
```

## 软件打不开了？之前可以打开
> 删除对应目录, 然后重启
```
## Mac执行
rm -rf /Users/$(whoami)/Library/Preferences/res-downloader

## Windows手动删除以下目录，Administrator为用户名 通常如下：
C:\Users\Administrator\AppData\Roaming\res-downloader

## Linux手动删除以下目录
/home/user/.config/res-downloader/home/user/.config/res-downloader
```

## 某应用只支持手机打开 如何拦截？
> 这里需要注意的是 应用使用http协议通讯才能拦截，且安卓7.0以上系统不再信任用户CA证书 所以没法拦截，解决方案自行查找，
```
1. 将手机和电脑处于同一个网络
2. 在手机端安装res-downloader的证书
3. 将手机网络代理设置为res-downloader的代理
4. 正常使用
```

## Mac 提示“已损坏，无法打开”, 打开命令行执行如下命令：
> sudo xattr -d com.apple.quarantine /Applications/res-downloader.app

## 打开本软件，无法正常拦截获取
> 检查系统证书是否安装  
> 关闭网络防火墙  
> 系统代理是否正确设置(代理地址：127.0.0.1 端口：8899)

## 关闭软件后无法正常上网
> 手动关闭系统代理设置

## 链接不是私密链接
> 通常是证书未正确安装，最新版证书下载：软件左下角 ？点击后有下载地址  
> 根据自己系统进行安装证书操作(不懂的自行百度)，手动安装需安装到受信任的根证书  

- Mac手动安装证书(V3+版本支持)，打开终端复制以下命令 粘贴到终端回车 按照提示输入密码，完成后再打开软件：
```shell
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain /Users/$(whoami)/Library/Preferences/res-downloader/cert.crt && touch /Users/$(whoami)/Library/Preferences/res-downloader/install.lock && echo "安装完成"
```

## 拦截不到小程序中的资源
清理微信缓存，删除小程序后，重新打开
> 设置->存储空间->缓存

## 只拦截打开的视频号视频
关闭全量拦截，打开视频号视频详情，通常分享好友后打开的页面属于详情页

## 拦截视频号账号视频
打开对应作者个人主页，浏览即可

## 下载慢、大视频下载失败
推荐使用如下工具加速下载，视频号可以下载完成后再到对应视频操作项选择 “视频解密” 按钮  
> [Neat Download Manager](https://www.neatdownloadmanager.com/index.php/en/)、[Motrix](https://motrix.app/download)等软件进行下载

## 直播流: 预览和录制：
> [使用obs进行预览和录制 使用教程自行百度， 点击下载obs]( https://obsproject.com/)

## m3u8: 预览和下载：
> [在线下载](https://m3u8-down.gowas.cn/)、[在线预览](https://m3u8play.com/)

## 安装证书后还会提示安装
使用命令行打开本软件，查看 “lockfile:” 这串字符后面的锁文件路径，然后创建该文件即可  
例如 mac系统下终端执行如下命令即可创建  
> touch /Users/你的用户名/Library/Preferences/res-downloader/install.lock

## 更多问题 请前往github进行[反馈](https://github.com/putyy/res-downloader/issues)