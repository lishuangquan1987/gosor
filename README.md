# gosor

[![Go Reference](https://pkg.go.dev/badge/github.com/lishuangquan1987/gosor.svg)](https://pkg.go.dev/github.com/lishuangquan1987/gosor)
[![CI](https://github.com/lishuangquan1987/gosor/actions/workflows/test.yml/badge.svg)](https://github.com/lishuangquan1987/gosor/actions/workflows/test.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

gosor 是 [pyOTDR](https://github.com/sid5432/pyOTDR) 解析核心的 Go 翻译：零依赖地解析
OTDR 光纤测试仪（Telcordia/Bellcore GR-196）生成的 **.sor** 文件，得到测量参数、
关键事件与完整反射/背向散射曲线。

纯 Go、无 cgo、无第三方依赖，`go get` 即用。

## 特性

- 解析 GR-196 **format 2**（"Map" 头，主流仪器如 EXFO FTB/FTBx 均为此格式）的全部标准块：
  Map、GenParams、SupParams、FxdParams、KeyEvents、DataPts、Cksum
- 兼容尝试解析 **format 1**（旧版无 "Map" 头的文件）
- 输出类型化数值（不丢失精度），并按 pyOTDR 语义完成折射率修正：
  采样间隔 × 光速 ÷ 折射率 → 每点分辨率（米）与量程（km）
- 曲线点 `[]Point{X km, Y dB}`，支持 STV / AFL / RAW 三种 Y 轴偏移方式
- 顺序读取累积 CRC-16/CCITT-FALSE 并与文件存储校验和比对
- 未知私有块（如 EXFO 私有块）自动整块跳过，不影响解析

## 安装

```sh
go get github.com/lishuangquan1987/gosor
```

要求 Go 1.25+。

## 快速开始

```go
package main

import (
	"fmt"
	"log"

	"github.com/lishuangquan1987/gosor/sor"
)

func main() {
	f, err := sor.ParseFile("measurement.sor")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("波长:", f.Fxd.WavelengthNM, "nm")   // 1310.0
	fmt.Println("分辨率:", f.Fxd.Resolution, "m")     // 每点米数（折射率修正后）
	fmt.Println("量程:", f.Fxd.AdjustedRangeKM, "km")
	fmt.Println("模块:", f.Sup.Module)                // 如 FTB-7400E-NS802-3
	fmt.Println("总损耗:", f.Keys.Summary.TotalLoss, "dB")
	fmt.Println("ORL:", f.Keys.Summary.ORL, "dB")
	for _, e := range f.Keys.Events {
		fmt.Printf("事件 @%.3f km  %s  熔接 %.3f dB\n", e.DistanceKM, e.Type, e.SpliceLoss)
	}
	for _, pt := range f.Trace {
		_ = pt // pt.X 单位 km，pt.Y 单位 dB
	}
}
```

需要自定义 Y 轴偏移方式时：

```go
f, err := sor.ParseFileWith("measurement.sor", sor.Options{OffsetMode: "AFL"})
```

## 命令行工具

```sh
go run github.com/lishuangquan1987/gosor/cmd/sordump@latest <file.sor> [out.json] [out.csv]
```

输出 JSON 摘要（全部参数与事件）与曲线 CSV（X km / Y dB）。

## 解析结果（sor.File）

| 字段 | 说明 |
|---|---|
| `Format` / `Version` | GR-196 格式版本（1/2，"2.00"） |
| `Blocks` | Map 块表：名称、版本、大小、文件内位置 |
| `Gen` | 通用参数：光缆/光纤 ID、光纤类型、波长、测点 A/B、建设条件等 |
| `Sup` | 仪器参数：供应商、OTDR、模块与序列号 |
| `Fxd` | 固定参数：测量时间、波长、脉宽、采样间隔、折射率、门限、分辨率、量程等 |
| `Keys` | 关键事件：事件列表（距离/类型/斜率/熔接/反射损耗）+ 汇总（总损耗、ORL） |
| `Data` | 数据点块头信息：点数、缩放系数、Y 偏移方式 |
| `Trace` | 曲线 `[]Point`：X 单位 km，Y 单位 dB |
| `Checksum` / `ChecksumOurs` / `ChecksumMatch` | 文件存储校验和 / 顺序 CRC 计算值 / 是否一致 |

## 与 pyOTDR 的对应关系

| pyOTDR (Python) | gosor (Go) |
|---|---|
| read.py | sor/parse.go |
| parts.py | sor/reader.go（含顺序 CRC） |
| mapblock.py | sor/parse.go |
| genparams.py / supparams.py | sor/blocks.go |
| fxdparams.py | sor/blocks.go |
| keyevents.py | sor/blocks.go |
| datapts.py | sor/blocks.go |
| cksum.py | sor/cksum.go |

设计差异：gosor 保存类型化数值而非展示用格式化字符串，数值精度不低于 pyOTDR
（例如平均时间 0.5 s 不会被 `"%.0f"` 显示成 0）。

## 已知局限

- 部分厂商（如 EXFO）使用私有校验和算法，`ChecksumMatch` 可能为 `false`；
  这与 pyOTDR 行为一致，不影响任何数据解析。
- format 1 的 Map 布局按 pyOTDR 的处理方式尽力解析，未在真实 1.x 文件上广泛验证。

## 测试

测试不依赖任何二进制样例文件——在内存中合成合法 SOR 字节流，
覆盖 format 2 / format 1 两条路径与 CRC 校验：

```sh
go test ./...
```

## 许可证

[GPL-3.0-or-later](LICENSE)。本项目衍生自 [pyOTDR](https://github.com/sid5432/pyOTDR)，
遵循其 GNU GPL v3+ 许可证发布。

## 致谢

- [pyOTDR](https://github.com/sid5432/pyOTDR) —— 原始 Python 实现与 GR-196 格式知识
- [Telcordia GR-196](https://www.telcordia.com) —— SOR 文件格式规范
