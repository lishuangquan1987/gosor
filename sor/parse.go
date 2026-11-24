package sor

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options 解析选项。
type Options struct {
	// OffsetMode 曲线 Y 值偏移方式：
	//   "STV"（默认，pyOTDR 默认）：最小读数平移到零（ymax - x）* fs
	//   "AFL"：最大读数平移到零（近似）
	//   "RAW"：取负 -x * fs
	OffsetMode string
}

// ParseFile 解析 SOR 文件。
func ParseFile(path string) (*File, error) {
	return ParseFileWith(path, Options{})
}

// ParseFileWith 按选项解析 SOR 文件。
func ParseFileWith(path string, opt Options) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := ParseWith(data, opt)
	if err != nil {
		return nil, err
	}
	f.Filename = filepath.Base(path)
	return f, nil
}

// Parse 解析 SOR 字节内容（对应 pyOTDR read.sorparse）。
func Parse(data []byte) (*File, error) { return ParseWith(data, Options{}) }

// ParseWith 按选项解析 SOR 字节内容（对应 pyOTDR read.sorparse）。
func ParseWith(data []byte, opt Options) (*File, error) {
	r := newReader(data)
	f := &File{Format: 1, Data: DataPts{OffsetMode: offsetMode(opt.OffsetMode), XScaling: 1}}

	// ---- Map 块（对应 mapblock.process）----
	r.reset() // 对应 fh.seek(0)
	tt := r.getString()
	if tt == "Map" {
		f.Format = 2
	} else {
		f.Format = 1
		r.reset() // 对应 format 1 的 rewind（重置 CRC）
	}
	f.Version = fmt.Sprintf("%.2f", float64(r.getUint(2))*0.01)
	f.Map.NBytes = uint32(r.getUint(4))
	f.Map.NBlocks = int(r.getUint(2)) - 1

	startpos := int64(f.Map.NBytes)
	for i := 0; i < f.Map.NBlocks; i++ {
		name := r.getString()
		ver := fmt.Sprintf("%.2f", float64(r.getUint(2))*0.01)
		size := uint32(r.getUint(4))
		f.Blocks = append(f.Blocks, Block{
			Name: name, Version: ver, Size: size, Pos: startpos, Order: i,
		})
		startpos += int64(size)
	}

	// ---- 各块处理（对应 read.sorparse 的按 order 派发）----
	for _, b := range f.Blocks {
		var err error
		// 对应 pyOTDR 各处理器的 fh.seek(startpos)：顺序文件下是无操作，
		// 异常文件则跳到块声明的位置（跳过的字节不参与 CRC，与 pyOTDR 一致）。
		r.seek(int(b.Pos))
		switch b.Name {
		case "GenParams":
			err = genparams(r, f)
		case "SupParams":
			err = supparams(r, f)
		case "FxdParams":
			err = fxdparams(r, f)
		case "DataPts":
			err = datapts(r, f)
		case "KeyEvents":
			err = keyevents(r, f)
		case "Cksum":
			err = cksum(r, f)
		default:
			// 对应 parts.slurp：未知块（如 EXFO 私有块）整块读入以维持 CRC 顺序
			r.seek(int(b.Pos))
			r.read(int(b.Size))
		}
		if err != nil {
			return f, err
		}
	}
	return f, nil
}

func offsetMode(s string) string {
	switch s {
	case "AFL", "RAW":
		return s
	default:
		return "STV"
	}
}

// ---- 块头校验（format 2 各块以 "块名\0" 开头）----

func expectHeader(r *reader, name string) error {
	want := name + "\x00"
	got := string(r.read(len(want)))
	if got != want {
		return fmt.Errorf("%s: incorrect header %q", name, got)
	}
	return nil
}
