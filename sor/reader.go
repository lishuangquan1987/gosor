package sor

import (
	"encoding/binary"
	"math"
)

// reader 顺序读取器：游标 + 随读取累积的 CRC16-CCITT-FALSE。
// 对应 pyOTDR parts.FH 的语义：CRC 按“被读取的字节”累积；seek(0) 会重置 CRC，
// 其余 seek 只移动游标。SOR 文件解析流程中块按顺序整块读取，无跳跃。
type reader struct {
	data []byte
	pos  int
	crc  uint16
}

func newReader(data []byte) *reader {
	return &reader{data: data, crc: 0xFFFF}
}

// reset 等价于 FH.seek(0)：游标归零并重置 CRC。
func (r *reader) reset() {
	r.pos = 0
	r.crc = 0xFFFF
}

func (r *reader) seek(pos int) { r.pos = pos }

// read 读取 n 字节并累积 CRC；不足 n 字节时返回剩余全部（对应 Python 文件读取的短读语义）。
func (r *reader) read(n int) []byte {
	if n < 0 {
		n = 0
	}
	if r.pos+n > len(r.data) {
		n = len(r.data) - r.pos
	}
	if n < 0 {
		n = 0
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	r.crc = crc16Update(r.crc, b)
	return b
}

// getString 读取以 \0 结尾的字符串（\0 计入读取并参与 CRC）。
func (r *reader) getString() string {
	i := r.pos
	for i < len(r.data) && r.data[i] != 0 {
		i++
	}
	if i >= len(r.data) {
		// 无终止符：读到 EOF
		b := r.read(len(r.data) - r.pos)
		return string(b)
	}
	b := r.read(i - r.pos + 1) // 含 \0
	return string(b[:len(b)-1])
}

func (r *reader) getUint(n int) uint64 {
	b := r.read(n)
	switch n {
	case 2:
		return uint64(binary.LittleEndian.Uint16(b))
	case 4:
		return uint64(binary.LittleEndian.Uint32(b))
	case 8:
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

func (r *reader) getSigned(n int) int64 {
	b := r.read(n)
	switch n {
	case 2:
		return int64(int16(binary.LittleEndian.Uint16(b)))
	case 4:
		return int64(int32(binary.LittleEndian.Uint32(b)))
	case 8:
		return int64(binary.LittleEndian.Uint64(b))
	}
	return 0
}

func (r *reader) getFloat(n int) float64 {
	b := r.read(n)
	switch n {
	case 4:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
	case 8:
		return math.Float64frombits(binary.LittleEndian.Uint64(b))
	}
	return 0
}
