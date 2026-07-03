# Wintun 驱动

Windows 版本需要 wintun.dll 才能创建 TUN 设备。

## 获取方式

1. 从官方下载: https://www.wintun.net/
2. 下载对应架构的 wintun.dll (x64/ARM64)
3. 将 wintun.dll 放入此目录

## 自动释放

程序启动时会自动将嵌入的 wintun.dll 释放到程序目录。
如果未嵌入，程序会尝试从系统 PATH 查找。

## 嵌入方法

将 wintun.dll 复制到此目录后，重新编译：

```bash
cp wintun.dll build/windows/
wails build
```

注意：由于 wintun.dll 是二进制文件，需要在 go.mod 中确保 embed 支持。
