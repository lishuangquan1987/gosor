package sor

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 金标准一致性测试：对 testdata/ 中每个 <name>.sor，
// 与 pyOTDR（Python 原版）生成的 <name>-dump.json 与 <name>-trace.dat 对比。

type pyBlock struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Size    int    `json:"size"`
	Pos     int    `json:"pos"`
	Order   int    `json:"order"`
}

type pyDump struct {
	Filename string `json:"filename"`
	Format   int    `json:"format"`
	Version  string `json:"version"`
	MapBlock struct {
		NBytes  int `json:"nbytes"`
		NBlocks int `json:"nblocks"`
	} `json:"mapblock"`
	Blocks    map[string]pyBlock `json:"blocks"`
	GenParams map[string]any     `json:"GenParams"`
	SupParams map[string]any     `json:"SupParams"`
	FxdParams map[string]any     `json:"FxdParams"`
	DataPts   map[string]any     `json:"DataPts"`
	KeyEvents map[string]any     `json:"KeyEvents"`
	Cksum     map[string]any     `json:"Cksum"`
}

func goldenNames(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		n := e.Name()
		if strings.HasSuffix(n, ".sor") {
			names = append(names, strings.TrimSuffix(n, ".sor"))
		}
	}
	if len(names) == 0 {
		t.Fatal("no .sor fixtures in testdata")
	}
	return names
}

// pyStr 取 Python dump 中的字符串字段。
func pyStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// pyNum 取数值字段（int 或 float）。
func pyNum(t *testing.T, m map[string]any, key string) float64 {
	t.Helper()
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.Fields(v)[0]), 64)
		if err != nil {
			t.Fatalf("pyNum %q: %v", key, err)
		}
		return f
	}
	t.Fatalf("pyNum %q: missing", key)
	return 0
}

// assertNear 近似相等断言。
func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s: got %.12f want %.12f (tol %g)", name, got, want, tol)
	}
}

func assertNearStr(t *testing.T, name, got, want string, tol float64) {
	t.Helper()
	g, err1 := strconv.ParseFloat(strings.Fields(strings.TrimSpace(got))[0], 64)
	w, err2 := strconv.ParseFloat(strings.Fields(strings.TrimSpace(want))[0], 64)
	if err1 != nil || err2 != nil {
		t.Errorf("%s: parse %q vs %q", name, got, want)
		return
	}
	assertNear(t, name, g, w, tol)
}

func TestGoldenParity(t *testing.T) {
	for _, name := range goldenNames(t) {
		name := name
		t.Run(name, func(t *testing.T) {
			sorPath := filepath.Join("testdata", name+".sor")
			f, err := ParseFile(sorPath)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join("testdata", name+"-dump.json"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var g pyDump
			if err := json.Unmarshal(raw, &g); err != nil {
				t.Fatalf("parse golden json: %v", err)
			}

			// ---- 头部 ----
			if f.Filename != g.Filename {
				t.Errorf("filename: %q vs %q", f.Filename, g.Filename)
			}
			if f.Format != g.Format {
				t.Errorf("format: %d vs %d", f.Format, g.Format)
			}
			if f.Version != g.Version {
				t.Errorf("version: %q vs %q", f.Version, g.Version)
			}
			if int(f.Map.NBytes) != g.MapBlock.NBytes {
				t.Errorf("map nbytes: %d vs %d", f.Map.NBytes, g.MapBlock.NBytes)
			}
			if f.Map.NBlocks != g.MapBlock.NBlocks {
				t.Errorf("map nblocks: %d vs %d", f.Map.NBlocks, g.MapBlock.NBlocks)
			}

			// ---- 块表 ----
			if len(f.Blocks) != len(g.Blocks) {
				t.Fatalf("blocks: %d vs %d", len(f.Blocks), len(g.Blocks))
			}
			for _, b := range f.Blocks {
				gb, ok := g.Blocks[b.Name]
				if !ok {
					t.Errorf("block %q missing in golden", b.Name)
					continue
				}
				if b.Version != gb.Version {
					t.Errorf("block %s version: %q vs %q", b.Name, b.Version, gb.Version)
				}
				if int(b.Size) != gb.Size {
					t.Errorf("block %s size: %d vs %d", b.Name, b.Size, gb.Size)
				}
				if int(b.Pos) != gb.Pos {
					t.Errorf("block %s pos: %d vs %d", b.Name, b.Pos, gb.Pos)
				}
				if b.Order != gb.Order {
					t.Errorf("block %s order: %d vs %d", b.Name, b.Order, gb.Order)
				}
			}

			// ---- GenParams ----
			gp := g.GenParams
			if f.Gen.Language != pyStr(gp, "language") {
				t.Errorf("language: %q vs %q", f.Gen.Language, pyStr(gp, "language"))
			}
			if f.Gen.FiberType != pyStr(gp, "fiber type") {
				t.Errorf("fiber type: %q vs %q", f.Gen.FiberType, pyStr(gp, "fiber type"))
			}
			assertNear(t, "gen wavelength", float64(f.Gen.WavelengthNM), pyNum(t, gp, "wavelength"), 1e-9)
			if f.Gen.BuildCondition != pyStr(gp, "build condition") {
				t.Errorf("build condition: %q vs %q", f.Gen.BuildCondition, pyStr(gp, "build condition"))
			}

			// ---- SupParams ----
			sp := g.SupParams
			for _, kv := range [][2]string{
				{"Supplier", "supplier"}, {"OTDR", "OTDR"}, {"OTDRSN", "OTDR S/N"},
				{"Module", "module"}, {"ModuleSN", "module S/N"},
				{"Software", "software"}, {"Other", "other"},
			} {
				got, want := map[string]string{
					"Supplier": f.Sup.Supplier, "OTDR": f.Sup.OTDR, "OTDRSN": f.Sup.OTDRSN,
					"Module": f.Sup.Module, "ModuleSN": f.Sup.ModuleSN,
					"Software": f.Sup.Software, "Other": f.Sup.Other,
				}[kv[0]], pyStr(sp, kv[1])
				if got != want {
					t.Errorf("sup %s: %q vs %q", kv[0], got, want)
				}
			}

			// ---- FxdParams ----
			fp := g.FxdParams
			assertNear(t, "wavelength", f.Fxd.WavelengthNM, pyNum(t, fp, "wavelength"), 1e-9)
			assertNear(t, "index", f.Fxd.Index, pyNum(t, fp, "index"), 1e-12)
			assertNear(t, "resolution", f.Fxd.Resolution, pyNum(t, fp, "resolution"), 1e-9)
			assertNear(t, "adjusted range", f.Fxd.AdjustedRangeKM, pyNum(t, fp, "range"), 1e-9)
			assertNear(t, "sample spacing", f.Fxd.SampleSpacingUS, pyNum(t, fp, "sample spacing"), 1e-12)
			assertNear(t, "pulse width", float64(f.Fxd.PulseWidthNS), pyNum(t, fp, "pulse width"), 1e-9)
			assertNear(t, "BC", f.Fxd.BC, pyNum(t, fp, "BC"), 1e-9)
			assertNear(t, "loss thr", f.Fxd.LossThr, pyNum(t, fp, "loss thr"), 1e-9)
			assertNear(t, "refl thr", f.Fxd.ReflThr, pyNum(t, fp, "refl thr"), 1e-9)
			assertNear(t, "EOT thr", f.Fxd.EOTThr, pyNum(t, fp, "EOT thr"), 1e-9)
			if _, ok := fp["averaging time"]; ok {
				// pyOTDR 以 "%.0f" 显示平均时间（银行家舍入，0.5s 显示成 0），按其显示精度比较
				assertNear(t, "averaging time", pyRound(f.Fxd.AveragingTimeSec, 0), pyNum(t, fp, "averaging time"), 1e-9)
			}
			assertNear(t, "num data points", float64(f.Fxd.NumDataPoints), pyNum(t, fp, "num data points"), 0.5)
			assertNear(t, "num averages", float64(f.Fxd.NumAverages), pyNum(t, fp, "num averages"), 0.5)
			assertNear(t, "noise floor level", float64(f.Fxd.NoiseFloorLevel), pyNum(t, fp, "noise floor level"), 0.5)
			assertNear(t, "noise floor scaling factor", float64(f.Fxd.NoiseFloorScalingFactor), pyNum(t, fp, "noise floor scaling factor"), 0.5)
			if got, want := f.Fxd.Unit, pyStr(fp, "unit"); got != strings.Fields(want)[0] {
				t.Errorf("unit: %q vs %q", got, want)
			}
			if got, want := f.Fxd.TraceType, pyStr(fp, "trace type"); !strings.HasPrefix(want, got) {
				t.Errorf("trace type: %q vs %q", got, want)
			}
			// date/time：比对秒数（dump 格式 "Tue Jul 16 ... (1721114531 sec)"）
			if s := pyStr(fp, "date/time"); s != "" {
				if i := strings.Index(s, "("); i >= 0 {
					secStr := strings.Fields(s[i+1:])[0]
					want, _ := strconv.ParseFloat(secStr, 64)
					assertNear(t, "date/time", float64(f.Fxd.DateTimeSec), want, 0.5)
				}
			}

			// ---- DataPts ----
			dp := g.DataPts
			assertNear(t, "data num points", float64(f.Data.NumDataPoints), pyNum(t, dp, "num data points"), 0.5)
			assertNear(t, "data num points 2", float64(f.Data.NumDataPoints2), pyNum(t, dp, "num data points 2"), 0.5)
			assertNear(t, "num traces", float64(f.Data.NumTraces), pyNum(t, dp, "num traces"), 0.5)
			assertNear(t, "scaling factor", f.Data.ScalingFactor, pyNum(t, dp, "scaling factor"), 1e-12)
			assertNear(t, "max before offset", f.Data.MaxBeforeOffset, pyNum(t, dp, "max before offset"), 1e-9)
			assertNear(t, "min before offset", f.Data.MinBeforeOffset, pyNum(t, dp, "min before offset"), 1e-9)

			// ---- KeyEvents ----
			ke := g.KeyEvents
			assertNear(t, "num events", float64(f.Keys.NumEvents), pyNum(t, ke, "num events"), 0.5)
			for j := 0; j < f.Keys.NumEvents; j++ {
				ev, ok := ke[fmt.Sprintf("event %d", j+1)].(map[string]any)
				if !ok {
					t.Fatalf("event %d missing in golden", j+1)
				}
				e := f.Keys.Events[j]
				assertNearStr(t, fmt.Sprintf("ev%d distance", j+1), fmt.Sprintf("%.3f", e.DistanceKM), pyStr(ev, "distance"), 1e-9)
				assertNearStr(t, fmt.Sprintf("ev%d slope", j+1), fmt.Sprintf("%.3f", e.Slope), pyStr(ev, "slope"), 1e-9)
				assertNearStr(t, fmt.Sprintf("ev%d splice", j+1), fmt.Sprintf("%.3f", e.SpliceLoss), pyStr(ev, "splice loss"), 1e-9)
				assertNearStr(t, fmt.Sprintf("ev%d refl", j+1), fmt.Sprintf("%.3f", e.ReflLoss), pyStr(ev, "refl loss"), 1e-9)
				if e.Type != pyStr(ev, "type") {
					t.Errorf("ev%d type: %q vs %q", j+1, e.Type, pyStr(ev, "type"))
				}
			}
			if sum, ok := ke["Summary"].(map[string]any); ok {
				assertNear(t, "total loss", f.Keys.Summary.TotalLoss, pyNum(t, sum, "total loss"), 1e-9)
				assertNear(t, "ORL", f.Keys.Summary.ORL, pyNum(t, sum, "ORL"), 1e-9)
				assertNear(t, "loss start", f.Keys.Summary.LossStart, pyNum(t, sum, "loss start"), 1e-6)
				assertNear(t, "loss end", f.Keys.Summary.LossEnd, pyNum(t, sum, "loss end"), 1e-6)
				assertNear(t, "ORL start", f.Keys.Summary.ORLStart, pyNum(t, sum, "ORL start"), 1e-6)
				assertNear(t, "ORL finish", f.Keys.Summary.ORLFinish, pyNum(t, sum, "ORL finish"), 1e-6)
			}

			// ---- Cksum ----
			ck := g.Cksum
			assertNear(t, "checksum", float64(f.Checksum), pyNum(t, ck, "checksum"), 0.5)
			assertNear(t, "checksum ours", float64(f.ChecksumOurs), pyNum(t, ck, "checksum_ours"), 0.5)
			pyMatch, _ := ck["match"].(bool)
			if f.ChecksumMatch != pyMatch {
				t.Errorf("checksum match: %v vs %v", f.ChecksumMatch, pyMatch)
			}

			// ---- 曲线 trace.dat（%f 六位小数 → 容差 5e-7）----
			rawTr, err := os.ReadFile(filepath.Join("testdata", name+"-trace.dat"))
			if err != nil {
				t.Fatalf("read trace: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(rawTr)), "\n")
			if len(lines) != len(f.Trace) {
				t.Fatalf("trace lines: %d vs %d", len(lines), len(f.Trace))
			}
			for i, ln := range lines {
				fields := strings.Fields(ln)
				if len(fields) != 2 {
					t.Fatalf("trace line %d: %q", i, ln)
				}
				x, _ := strconv.ParseFloat(fields[0], 64)
				y, _ := strconv.ParseFloat(fields[1], 64)
				if math.Abs(f.Trace[i].X-x) > 5e-7 || math.Abs(f.Trace[i].Y-y) > 5e-7 {
					t.Fatalf("trace[%d]: got (%.9f, %.9f) want (%.9f, %.9f)", i, f.Trace[i].X, f.Trace[i].Y, x, y)
				}
			}
		})
	}
}

func TestCRC16(t *testing.T) {
	// "123456789" → CRC-16/CCITT-FALSE = 0x29B1（标准测试值）
	got := crc16Update(0xFFFF, []byte("123456789"))
	if got != 0x29B1 {
		t.Errorf("crc16: got %04X want 29B1", got)
	}
}
