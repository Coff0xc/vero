// Package webui 通过 embed 打包前端构建产物(dist), 供单一二进制分发。
// 前端源码在 web/, vite 构建输出到 internal/webui/dist(见 web/vite.config.ts 的 outDir)。
// embed 与被嵌内容内聚在本包目录, 避免 Go 扫描前端 node_modules。
package webui

import "embed"

//go:embed all:dist
var Dist embed.FS
