## Mac
```bash
wails build -platform "darwin/universal"
create-dmg 'build/bin/res-downloader.app' --overwrite ./build/bin
mv -f "build/bin/res-downloader $(jq -r '.info.productVersion' wails.json).dmg" "build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_mac.dmg"
```

## Windows
```bash
wails build -f -nsis -platform "windows/amd64" -webview2 Embed -skipbindings && mv -f "build/bin/res-downloader-amd64-installer.exe" "build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_win_amd64.exe"
wails build -f -nsis -platform "windows/arm64" -webview2 Embed -skipbindings && mv -f "build/bin/res-downloader-arm64-installer.exe" "build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_win_arm64.exe"

```

## Linux

###  docker方式
> x86_64
```bash
docker build --network host -f build/linux/dockerfile -t res-downloader-amd-linux .
docker run -it --name res-downloader-amd-build --network host --privileged -v ./:/www/res-downloader res-downloader-amd-linux /bin/bash
# 容器内
cd /www/res-downloader
wails build -platform "linux/amd64" -s -skipbindings

# 打包debian
cp build/bin/res-downloader build/linux/Debian/usr/local/bin/
echo "$(cat build/linux/Debian/DEBIAN/.control | sed -e "s/{{Version}}/$(jq -r '.info.productVersion' wails.json)/g")" > build/linux/Debian/DEBIAN/control
dpkg-deb --build ./build/linux/Debian build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_linux_amd64.deb

# 打包AppImage
cp build/bin/res-downloader build/linux/AppImage/usr/bin/

# 复制WebKit相关文件
pushd build/linux/AppImage
find /usr/lib* -name WebKitNetworkProcess -exec mkdir -p $(dirname '{}') \; -exec cp --parents '{}' "." \; || true
find /usr/lib* -name WebKitWebProcess -exec mkdir -p $(dirname '{}') \; -exec cp --parents '{}' "." \; || true
find /usr/lib* -name libwebkit2gtkinjectedbundle.so -exec mkdir -p $(dirname '{}') \; -exec cp --parents '{}' "." \; || true
popd

wget -O ./build/bin/appimagetool-x86_64.AppImage https://github.com/AppImage/AppImageKit/releases/download/13/appimagetool-x86_64.AppImage 
chmod +x ./build/bin/appimagetool-x86_64.AppImage
./build/bin/appimagetool-x86_64.AppImage build/linux/AppImage build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_linux_amd64.AppImage

mv -f build/bin/res-downloader build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_linux_amd64
```

> arm64
```bash
# arm
docker build --platform linux/arm64 --network host -f build/linux/dockerfile -t res-downloader-arm-linux .
docker run --platform linux/arm64 -it --name res-downloader-arm-build --network host --privileged -v ./:/www/res-downloader res-downloader-arm-linux /bin/bash
# 容器内
cd /www/res-downloader
wails build -platform "linux/arm64" -s -skipbindings

# 打包debian
cp build/bin/res-downloader build/linux/Debian/usr/local/bin/
echo "$(cat build/linux/Debian/DEBIAN/.control | sed -e "s/{{Version}}/$(jq -r '.info.productVersion' wails.json)/g")" > build/linux/Debian/DEBIAN/control
dpkg-deb --build ./build/linux/Debian build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_linux_arm64.deb

mv -f build/bin/res-downloader build/bin/res-downloader_$(jq -r '.info.productVersion' wails.json)_linux_arm64
```

### Arch Linux 

[![Packaging status](https://repology.org/badge/vertical-allrepos/res-downloader.svg)](https://repology.org/project/res-downloader/versions)

```bash
yay -Syu res-downloader 
```
### Linux 本地编译

```bash
git clone https://github.com/putyy/res-downloader.git
cd res-downloader
# -- GO Proxy --
# 如果国内编译时 go 下载慢或报错，可以设置 go 国内代理加速，否则不用设置
export GO111MODULE=on
export GOPROXY=https://goproxy.cn,direct
# -- Go Proxy --
wails build
cd build
sudo install -Dvm755 bin/res-downloader -t /usr/bin
sudo install -Dvm644 appicon.png /usr/share/icons/hicolor/512x512/apps/res-downloader.png
sudo install -Dvm644 build/linux/Arch/res-downloader.desktop /usr/share/applications/res-downloader.desktop 
```
