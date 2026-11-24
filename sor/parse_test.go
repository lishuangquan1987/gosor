package sor

import (
	"encoding/binary"
	"math"
	"testing"
)

// 本包测试不依赖任何外部样例文件：用构建器在内存中合成合法的 SOR 字节流，
// 覆盖 format 2（"Map" 头）与 format 1 两条解析路径，并校验 CRC-16/CCITT-FALSE。

// ---- 字节流构建器 ----

type sorBuilder struct{ b []byte }

func (w *sorBuilder) raw(s string) { w.b = append(w.b, s...) }
func (w *sorBuilder) str(s string) { w.b = append(w.b, s+"\x00"...) }
func (w *sorBuilder) u16(v uint16) {
	var x [2]byte
	binary.LittleEndian.PutUint16(x[:], v)
	w.raw(string(x[:]))
}
func (w *sorBuilder) u32(v uint32) {
	var x [4]byte
	binary.LittleEndian.PutUint32(x[:], v)
	w.raw(string(x[:]))
}
func (w *sorBuilder) i16(v int16) { w.u16(uint16(v)) }
func (w *sorBuilder) i32(v int32) { w.u32(uint32(v)) }

type namedBlock struct {
	name string
	body []byte
}

// assemble 按给定数据块组装文件：map 头 + 块表 + 顺序块 + 正确的 Cksum。
// withMapHeader 为 true 时是 format 2（文件以 "Map\0" 开头，块带名字头）；
// 否则是 format 1（无 "Map\0"，块体不带名字头）。Cksum 始终作为最后一个
// 块登记进 map 并写入正确的 CRC-16/CCITT-FALSE。
func assemble(version uint16, blocks []namedBlock, withMapHeader bool) []byte {
	bodies := make([][]byte, len(blocks))
	for i, blk := range blocks {
		if withMapHeader {
			var h sorBuilder
			h.str(blk.name)
			bodies[i] = append(h.b, blk.body...)
		} else {
			bodies[i] = blk.body
		}
	}
	cksumSize := uint32(2) // format 1：Cksum 块只有存储的 u16
	if withMapHeader {
		cksumSize = 8 // "Cksum\0" + u16
	}

	var m sorBuilder
	if withMapHeader {
		m.str("Map")
	}
	m.u16(version)
	nbytesAt := len(m.b)
	m.u32(0)                           // map nbytes 占位
	m.u16(uint16(len(blocks) + 1 + 1)) // 数据块 + Cksum
	for i, blk := range blocks {
		m.str(blk.name)
		m.u16(version)
		m.u32(uint32(len(bodies[i])))
	}
	m.str("Cksum")
	m.u16(version)
	m.u32(cksumSize)
	binary.LittleEndian.PutUint32(m.b[nbytesAt:], uint32(len(m.b)))

	var w sorBuilder
	w.b = append(w.b, m.b...)
	for _, body := range bodies {
		w.b = append(w.b, body...)
	}
	if withMapHeader {
		w.str("Cksum") // format 2 的 Cksum 块带名字头
	}
	w.u16(crc16Update(0xFFFF, w.b)) // 存储校验和 = 其前全部字节的 CRC
	return w.b
}

// buildFormat2 合成一个完整的最小 format 2 文件：
// GenParams + SupParams + FxdParams + KeyEvents + DataPts + Cksum，4 个数据点。
func buildFormat2() []byte {
	var gen sorBuilder
	gen.raw("EN") // language
	gen.str("")   // cable ID
	gen.str("")   // fiber ID
	gen.u16(652)  // fiber type → G.652
	gen.u16(1310) // wavelength nm
	gen.str("")   // location A
	gen.str("")   // location B
	gen.str("")   // cable code
	gen.raw("BC") // build condition
	gen.i32(0)    // user offset
	gen.i32(0)    // user offset distance
	gen.str("")   // operator
	gen.str("")   // comments

	var sup sorBuilder
	sup.str("ACME")
	sup.str("")
	sup.str("")
	sup.str("FTB-7400E")
	sup.str("123456")
	sup.str("")
	sup.str("")

	var fxd sorBuilder
	fxd.u32(1721114531) // date/time
	fxd.raw("mt")       // unit
	fxd.u16(13100)      // wavelength ×0.1 → 1310.0 nm
	fxd.i32(0)          // acquisition offset
	fxd.i32(0)          // acquisition offset distance
	fxd.u16(1)          // pulse width entries
	fxd.u16(100)        // pulse width ns
	fxd.u32(250000)     // sample spacing ×1e-8 → 0.0025 µs
	fxd.u32(4)          // num data points
	fxd.u32(146710)     // index ×1e-5 → 1.4671
	fxd.i16(0)          // BC ×-0.1 → 0 dB
	fxd.u32(1)          // num averages
	fxd.u16(30)         // averaging time ×0.1 → 3.0 s
	fxd.u32(0)          // range raw ×2e-5
	fxd.i32(0)          // acquisition range distance
	fxd.i32(0)          // front panel offset
	fxd.u16(47865)      // noise floor level
	fxd.i16(1000)       // noise floor scaling factor
	fxd.u16(0)          // power offset first point
	fxd.u16(20)         // loss thr ×0.001 → 0.020 dB
	fxd.u16(65535)      // refl thr ×-0.001 → -65.535 dB
	fxd.u16(5000)       // EOT thr ×0.001 → 5.000 dB
	fxd.raw("ST")       // trace type
	fxd.i32(0)          // X1
	fxd.i32(0)          // Y1
	fxd.i32(0)          // X2
	fxd.i32(0)          // Y2

	var key sorBuilder
	key.u16(1)          // 1 个事件
	key.u16(1)          // event id
	key.u32(100000)     // distance ×factor
	key.i16(320)        // slope ×0.001 → 0.320 dB/km
	key.i16(-50)        // splice ×0.001 → -0.050 dB
	key.i32(-44580)     // reflection ×0.001 → -44.580 dB
	key.raw("0A9999LS") // 事件类型（8 字节）
	key.u32(0)          // end of prev ×factor
	key.u32(100000)     // start of curr ×factor
	key.u32(105000)     // end of curr ×factor
	key.u32(150000)     // start of next ×factor
	key.u32(100500)     // peak ×factor
	key.str("")         // comments
	key.i32(5000)       // total loss ×0.001 → 5.000 dB
	key.i32(0)          // loss start ×factor
	key.u32(2500000)    // loss end ×factor
	key.u16(35127)      // ORL ×0.001 → 35.127 dB
	key.i32(0)          // ORL start ×factor
	key.u32(2500000)    // ORL finish ×factor

	var dat sorBuilder
	dat.u32(4)    // num data points
	dat.i16(1)    // num traces
	dat.u32(4)    // num data points (again)
	dat.u16(1000) // scaling factor ÷1000 → 1.0
	dat.u16(1000) // 原始值：ymax
	dat.u16(900)
	dat.u16(800)
	dat.u16(700) // ymin

	return assemble(200, []namedBlock{
		{"GenParams", gen.b},
		{"SupParams", sup.b},
		{"FxdParams", fxd.b},
		{"KeyEvents", key.b},
		{"DataPts", dat.b},
	}, true)
}

// buildFormat1 合成一个最小 format 1 文件：GenParams + FxdParams + Cksum。
func buildFormat1() []byte {
	var gen sorBuilder
	gen.raw("EN")
	gen.str("") // cable ID
	gen.str("") // fiber ID
	gen.u16(1310)
	gen.str("") // location A
	gen.str("") // location B
	gen.str("") // cable code
	gen.raw("BC")
	gen.i32(0)  // user offset
	gen.str("") // operator
	gen.str("") // comments

	var fxd sorBuilder
	fxd.u32(1721114531)
	fxd.raw("mt")
	fxd.u16(13100)
	fxd.i32(0)   // acquisition offset
	fxd.u16(1)   // pulse width entries
	fxd.u16(100) // pulse width ns
	fxd.u32(250000)
	fxd.u32(4)
	fxd.u32(146710)
	fxd.i16(0)  // BC
	fxd.u32(1)  // num averages
	fxd.u32(0)  // range raw
	fxd.i32(0)  // front panel offset
	fxd.u16(0)  // noise floor level
	fxd.i16(0)  // noise floor scaling factor
	fxd.u16(0)  // power offset first point
	fxd.u16(20) // loss thr
	fxd.u16(65535)
	fxd.u16(5000)

	return assemble(100, []namedBlock{
		{"GenParams", gen.b},
		{"FxdParams", fxd.b},
	}, false)
}

// ---- 断言辅助 ----

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s: got %.12f want %.12f (tol %g)", name, got, want, tol)
	}
}

func assertEq(t *testing.T, name string, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v want %v", name, got, want)
	}
}

// ---- 测试 ----

// 与 pyOTDR 一致的派生量：折射率修正与时间-距离换算。
func testDXkm() float64   { return 0.0025 * Sol / 1.4671 } // km/点
func testFactor() float64 { return 1e-4 * Sol / 1.4671 }   // u32 → km

func TestParseFormat2(t *testing.T) {
	f, err := Parse(buildFormat2())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertEq(t, "Format", f.Format, 2)
	assertEq(t, "Version", f.Version, "2.00")
	assertEq(t, "Map.NBlocks", f.Map.NBlocks, 6) // 5 个数据块 + Cksum
	assertEq(t, "block count", len(f.Blocks), 6)
	wantNames := []string{"GenParams", "SupParams", "FxdParams", "KeyEvents", "DataPts", "Cksum"}
	for i, b := range f.Blocks {
		assertEq(t, "block order", b.Name, wantNames[i])
		assertEq(t, "block version", b.Version, "2.00")
		assertEq(t, "block order idx", b.Order, i)
	}
	if len(f.Blocks) > 0 {
		assertEq(t, "first block pos", f.Blocks[0].Pos, int64(f.Map.NBytes))
		for i := 1; i < len(f.Blocks); i++ {
			want := f.Blocks[i-1].Pos + int64(f.Blocks[i-1].Size)
			assertEq(t, "block pos chain", f.Blocks[i].Pos, want)
		}
	}

	// GenParams
	assertEq(t, "language", f.Gen.Language, "EN")
	assertEq(t, "fiber type code", f.Gen.FiberTypeCode, uint16(652))
	assertEq(t, "fiber type", f.Gen.FiberType, "G.652 (standard SMF)")
	assertEq(t, "gen wavelength", f.Gen.WavelengthNM, uint16(1310))
	assertEq(t, "build condition", f.Gen.BuildCondition, "BC (as-built)")

	// SupParams
	assertEq(t, "supplier", f.Sup.Supplier, "ACME")
	assertEq(t, "module", f.Sup.Module, "FTB-7400E")
	assertEq(t, "module S/N", f.Sup.ModuleSN, "123456")

	// FxdParams
	assertEq(t, "date/time", f.Fxd.DateTimeSec, int64(1721114531))
	assertEq(t, "unit", f.Fxd.Unit, "mt")
	assertNear(t, "wavelength", f.Fxd.WavelengthNM, 1310.0, 1e-12)
	assertEq(t, "pulse width", f.Fxd.PulseWidthNS, uint16(100))
	assertNear(t, "sample spacing", f.Fxd.SampleSpacingUS, 0.0025, 1e-12)
	assertEq(t, "num data points", f.Fxd.NumDataPoints, uint32(4))
	assertNear(t, "index", f.Fxd.Index, 1.4671, 1e-12)
	assertNear(t, "BC", f.Fxd.BC, 0, 1e-12)
	assertEq(t, "num averages", f.Fxd.NumAverages, uint32(1))
	assertNear(t, "averaging time", f.Fxd.AveragingTimeSec, 3.0, 1e-12)
	assertNear(t, "loss thr", f.Fxd.LossThr, 0.020, 1e-12)
	assertNear(t, "refl thr", f.Fxd.ReflThr, -65.535, 1e-12)
	assertNear(t, "EOT thr", f.Fxd.EOTThr, 5.000, 1e-12)
	assertEq(t, "trace type", f.Fxd.TraceType, "ST")
	assertNear(t, "resolution", f.Fxd.Resolution, testDXkm()*1000, 1e-9)
	assertNear(t, "adjusted range", f.Fxd.AdjustedRangeKM, testDXkm()*4, 1e-9)

	// DataPts
	assertEq(t, "num traces", f.Data.NumTraces, int16(1))
	assertNear(t, "scaling factor", f.Data.ScalingFactor, 1.0, 1e-12)
	assertNear(t, "max before offset", f.Data.MaxBeforeOffset, 1.0, 1e-12)
	assertNear(t, "min before offset", f.Data.MinBeforeOffset, 0.7, 1e-12)
	assertEq(t, "trace length", len(f.Trace), 4)
	wantY := []float64{0, 0.1, 0.2, 0.3} // STV：(ymax-raw)×fs
	for i, pt := range f.Trace {
		assertNear(t, "trace x", pt.X, testDXkm()*float64(i), 1e-12)
		assertNear(t, "trace y", pt.Y, wantY[i], 1e-12)
	}

	// KeyEvents
	assertEq(t, "num events", f.Keys.NumEvents, 1)
	if len(f.Keys.Events) != 1 {
		t.Fatalf("events: got %d want 1", len(f.Keys.Events))
	}
	e := f.Keys.Events[0]
	assertEq(t, "event id", e.ID, uint16(1))
	assertEq(t, "event type", e.Type, "0A9999LS {manual} loss/drop/gain")
	assertNear(t, "event distance", e.DistanceKM, 100000*testFactor(), 1e-9)
	assertNear(t, "event slope", e.Slope, 0.320, 1e-12)
	assertNear(t, "event splice", e.SpliceLoss, -0.050, 1e-12)
	assertNear(t, "event refl", e.ReflLoss, -44.580, 1e-12)
	assertNear(t, "event end of prev", e.EndOfPrevKM, 0, 1e-12)
	assertNear(t, "event start of curr", e.StartOfCurrKM, 100000*testFactor(), 1e-9)
	assertNear(t, "event end of curr", e.EndOfCurrKM, 105000*testFactor(), 1e-9)
	assertNear(t, "event start of next", e.StartOfNextKM, 150000*testFactor(), 1e-9)
	assertNear(t, "event peak", e.PeakKM, 100500*testFactor(), 1e-9)
	s := f.Keys.Summary
	assertNear(t, "total loss", s.TotalLoss, 5.000, 1e-12)
	assertNear(t, "loss start", s.LossStart, 0, 1e-12)
	// 汇总位置值按 pyOTDR 语义做 6 位小数舍入
	assertNear(t, "loss end", s.LossEnd, pyRound(2500000*testFactor(), 6), 1e-12)
	assertNear(t, "ORL", s.ORL, 35.127, 1e-12)
	assertNear(t, "ORL finish", s.ORLFinish, pyRound(2500000*testFactor(), 6), 1e-12)

	// Cksum：构建器写入了正确校验和
	assertEq(t, "checksum match", f.ChecksumMatch, true)
	assertEq(t, "checksum", f.Checksum, f.ChecksumOurs)
}

func TestParseFormat1(t *testing.T) {
	f, err := Parse(buildFormat1())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertEq(t, "Format", f.Format, 1)
	assertEq(t, "Version", f.Version, "1.00")
	assertEq(t, "Map.NBlocks", f.Map.NBlocks, 3) // 2 个数据块 + Cksum
	assertEq(t, "language", f.Gen.Language, "EN")
	assertEq(t, "gen wavelength", f.Gen.WavelengthNM, uint16(1310))
	assertEq(t, "build condition", f.Gen.BuildCondition, "BC (as-built)")
	assertNear(t, "index", f.Fxd.Index, 1.4671, 1e-12)
	assertNear(t, "resolution", f.Fxd.Resolution, testDXkm()*1000, 1e-9)
	assertEq(t, "trace type (format 1 无此字段)", f.Fxd.TraceType, "")
	assertEq(t, "no DataPts block", len(f.Trace), 0)
	assertEq(t, "num events", f.Keys.NumEvents, 0)
	assertEq(t, "checksum match", f.ChecksumMatch, true)
}

func TestParseRejectsMultipleTraces(t *testing.T) {
	data := buildFormat2()
	// DataPts 的 num traces 字段改为 2。注意 map 块表里也有 "DataPts\0" 字样，
	// 必须定位最后一次出现（即块头本身）。
	needle := []byte("DataPts\x00")
	pos := -1
	for i := 0; i+len(needle) <= len(data); i++ {
		match := true
		for j := range needle {
			if data[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			pos = i // 持续更新 → 取最后一次出现
		}
	}
	if pos < 0 {
		t.Fatal("DataPts header not found")
	}
	// 头后依次：N u32、traces i16 → 覆盖 traces 为 2
	tracesAt := pos + len(needle) + 4
	binary.LittleEndian.PutUint16(data[tracesAt:], 2)

	if _, err := Parse(data); err == nil {
		t.Error("multiple traces should be rejected with an error")
	}
}

func TestCRC16(t *testing.T) {
	// CRC-16/CCITT-FALSE 标准测试向量："123456789" → 0x29B1
	if got := crc16Update(0xFFFF, []byte("123456789")); got != 0x29B1 {
		t.Errorf("crc16: got %04X want 29B1", got)
	}
}
