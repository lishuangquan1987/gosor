package sor

import (
	"fmt"
	"strconv"
)

// ---- pyOTDR genparams.py ----

// fiberType 解码光纤类型（pyOTDR genparams.fiber_type）。
func fiberType(val uint16) string {
	switch val {
	case 651:
		return "G.651 (50um core multimode)"
	case 652:
		return "G.652 (standard SMF)"
	case 653:
		return "G.653 (dispersion-shifted fiber)"
	case 654:
		return "G.654 (1550nm loss-minimzed fiber)"
	case 655:
		return "G.655 (nonzero dispersion-shifted fiber)"
	default:
		return fmt.Sprintf("%d (unknown)", val)
	}
}

// buildCondition 解码建设条件（pyOTDR genparams.build_condition）。
func buildCondition(s string) string {
	switch s {
	case "BC":
		return "BC (as-built)"
	case "CC":
		return "CC (as-current)"
	case "RC":
		return "RC (as-repaired)"
	case "OT":
		return "OT (other)"
	default:
		return s + " (unknown)"
	}
}

func genparams(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "GenParams"); err != nil {
			return err
		}
	}
	g := &f.Gen
	g.Language = string(r.read(2))
	if f.Format == 1 {
		// pyOTDR genparams.process1：v1 无 fiber type / user offset distance
		g.CableID = r.getString()
		g.FiberID = r.getString()
		g.WavelengthNM = uint16(r.getUint(2))
		g.LocationA = r.getString()
		g.LocationB = r.getString()
		g.CableCode = r.getString()
		bc := string(r.read(2))
		g.BuildConditionCode = bc
		g.BuildCondition = buildCondition(bc)
		g.UserOffset = int32(r.getSigned(4))
		g.Operator = r.getString()
		g.Comments = r.getString()
		return nil
	}
	g.CableID = r.getString()
	g.FiberID = r.getString()
	g.FiberTypeCode = uint16(r.getUint(2))
	g.FiberType = fiberType(g.FiberTypeCode)
	g.WavelengthNM = uint16(r.getUint(2))
	g.LocationA = r.getString()
	g.LocationB = r.getString()
	g.CableCode = r.getString()
	bc := string(r.read(2))
	g.BuildConditionCode = bc
	g.BuildCondition = buildCondition(bc)
	g.UserOffset = int32(r.getSigned(4))
	g.UserOffsetDistance = int32(r.getSigned(4))
	g.Operator = r.getString()
	g.Comments = r.getString()
	return nil
}

// ---- pyOTDR supparams.py ----

func supparams(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "SupParams"); err != nil {
			return err
		}
	}
	s := &f.Sup
	s.Supplier = r.getString()
	s.OTDR = r.getString()
	s.OTDRSN = r.getString()
	s.Module = r.getString()
	s.ModuleSN = r.getString()
	s.Software = r.getString()
	s.Other = r.getString()
	return nil
}

// ---- pyOTDR fxdparams.py ----

// pyRound 复现 pyOTDR 的“格式化字符串再转回浮点”语义（位级一致）：
// Python 中 index 经 "%.6f" 格式化后再 float()，sample spacing 经 str() 往返。
func pyRound(v float64, digits int) float64 {
	s := strconv.FormatFloat(v, 'f', digits, 64)
	v2, _ := strconv.ParseFloat(s, 64)
	return v2
}

// pyShortest 复现 Python str(float) 的最短往返语义。
func pyShortest(v float64) float64 {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	v2, _ := strconv.ParseFloat(s, 64)
	return v2
}

func fxdparams(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "FxdParams"); err != nil {
			return err
		}
	}
	x := &f.Fxd
	x.DateTimeSec = int64(r.getUint(4))
	x.Unit = string(r.read(2))
	x.WavelengthNM = float64(r.getUint(2)) * 0.1
	x.AcquisitionOffset = int32(r.getSigned(4))
	if f.Format == 2 {
		x.AcquisitionOffsetDist = int32(r.getSigned(4))
	}
	x.NumPulseWidthEntries = uint16(r.getUint(2))
	if x.NumPulseWidthEntries > 1 {
		// pyOTDR 遇到多脉宽条目直接退出；此处返回错误
		return fmt.Errorf("cannot handle multiple pulse width entries (%d)", x.NumPulseWidthEntries)
	}
	x.PulseWidthNS = uint16(r.getUint(2))
	x.SampleSpacingUS = float64(r.getUint(4)) * 1e-8
	x.NumDataPoints = uint32(r.getUint(4))
	x.Index = pyRound(float64(r.getUint(4))*1e-5, 6)
	x.BC = float64(r.getSigned(2)) * -0.1
	x.NumAverages = uint32(r.getUint(4))
	if f.Format == 2 {
		x.AveragingTimeSec = float64(r.getUint(2)) * 0.1
	}
	x.RangeKM = float64(r.getUint(4)) * 2e-5
	if f.Format == 2 {
		x.AcquisitionRangeDist = int32(r.getSigned(4))
	}
	x.FrontPanelOffset = int32(r.getSigned(4))
	x.NoiseFloorLevel = uint16(r.getUint(2))
	x.NoiseFloorScalingFactor = int16(r.getSigned(2))
	x.PowerOffsetFirstPoint = uint16(r.getUint(2))
	x.LossThr = float64(r.getUint(2)) * 0.001
	x.ReflThr = float64(r.getUint(2)) * -0.001 // pyOTDR：无符号读 × -0.001
	x.EOTThr = float64(r.getUint(2)) * 0.001
	if f.Format == 2 {
		x.TraceType = string(r.read(2))
		x.X1 = int32(r.getSigned(4))
		x.Y1 = int32(r.getSigned(4))
		x.X2 = int32(r.getSigned(4))
		x.Y2 = int32(r.getSigned(4))
	}

	// corrections（pyOTDR fxdparams._process_fields 末尾）：折射率修正
	ss := pyShortest(x.SampleSpacingUS)
	dx := ss * Sol / x.Index // km
	x.AdjustedRangeKM = dx * float64(x.NumDataPoints)
	x.Resolution = dx * 1000.0 // 米/点
	return nil
}

// ---- pyOTDR datapts.py ----

func datapts(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "DataPts"); err != nil {
			return err
		}
	}
	d := &f.Data
	n := int(r.getUint(4))
	d.NumDataPoints = uint32(n)
	if d.NumDataPoints != f.Fxd.NumDataPoints {
		// pyOTDR 仅告警；此处记录但不中断
		_ = d.NumDataPoints
	}
	d.NumTraces = int16(r.getSigned(2))
	if d.NumTraces > 1 {
		return fmt.Errorf("cannot handle multiple traces (%d)", d.NumTraces)
	}
	d.NumDataPoints2 = uint32(r.getUint(4))
	sf := float64(r.getUint(2)) / 1000.0
	d.ScalingFactor = sf

	// 特殊机型：OFL250 的 X 轴偏差 10 倍（pyOTDR datapts._process_data）
	if f.Sup.OTDR == "OFL250" {
		d.XScaling = 0.1
	}

	vals := make([]uint16, n)
	ymin, ymax := uint64(0xFFFF), uint64(0)
	for i := 0; i < n; i++ {
		v := r.getUint(2)
		vals[i] = uint16(v)
		if uint64(v) < ymin {
			ymin = uint64(v)
		}
		if uint64(v) > ymax {
			ymax = uint64(v)
		}
	}
	fs := 0.001 * d.ScalingFactor
	d.MaxBeforeOffset = pyRound(float64(ymax)*fs, 3)
	d.MinBeforeOffset = pyRound(float64(ymin)*fs, 3)

	// 转换为 dB 曲线（pyOTDR datapts._process_data）
	dx := f.Fxd.Resolution // 米
	xscaling := d.XScaling
	trace := make([]Point, n)
	for i := 0; i < n; i++ {
		var y float64
		switch d.OffsetMode {
		case "STV":
			y = (float64(ymax) - float64(vals[i])) * fs
		case "AFL":
			y = (float64(ymin) - float64(vals[i])) * fs
		default:
			y = -float64(vals[i]) * fs
		}
		x := dx * float64(i) * xscaling / 1000.0 // km
		trace[i] = Point{X: x, Y: y}
	}
	f.Trace = trace
	return nil
}

// ---- pyOTDR keyevents.py ----

// decodeEventType 解码 8 字节事件类型（pyOTDR keyevents._process_keyevents，
// 正则 "(.)(.)9999LS" 的手工等价实现）。
func decodeEventType(xtype string) string {
	if len(xtype) >= 8 && xtype[2:6] == "9999" && xtype[6:8] == "LS" {
		subtype := xtype[0]
		manual := xtype[1]
		if manual == 'A' {
			xtype += " {manual}"
		} else {
			xtype += " {auto}"
		}
		switch subtype {
		case '1':
			xtype += " reflection"
		case '0':
			xtype += " loss/drop/gain"
		case '2':
			xtype += " multiple"
		default:
			xtype += " unknown '" + string(subtype) + "'"
		}
		return xtype
	}
	return xtype + " [unknown type " + xtype + "]"
}

func keyevents(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "KeyEvents"); err != nil {
			return err
		}
	}
	k := &f.Keys
	nev := int(r.getUint(2))
	k.NumEvents = nev
	factor := 1e-4 * Sol / f.Fxd.Index

	k.Events = make([]KeyEvent, 0, nev)
	for j := 0; j < nev; j++ {
		var e KeyEvent
		e.ID = uint16(r.getUint(2))
		e.DistanceKM = float64(r.getUint(4)) * factor
		e.Slope = float64(r.getSigned(2)) * 0.001
		e.SpliceLoss = float64(r.getSigned(2)) * 0.001
		e.ReflLoss = float64(r.getSigned(4)) * 0.001
		xtype := string(r.read(8))
		if f.Format == 2 {
			e.EndOfPrevKM = float64(r.getUint(4)) * factor
			e.StartOfCurrKM = float64(r.getUint(4)) * factor
			e.EndOfCurrKM = float64(r.getUint(4)) * factor
			e.StartOfNextKM = float64(r.getUint(4)) * factor
			e.PeakKM = float64(r.getUint(4)) * factor
		}
		e.Comments = r.getString()
		e.Type = decodeEventType(xtype)
		k.Events = append(k.Events, e)
	}

	// 汇总（pyOTDR：total/ORL 三位舍入，位置值六位舍入）
	s := &k.Summary
	s.TotalLoss = pyRound3(float64(r.getSigned(4)) * 0.001)
	s.LossStart = pyRound(float64(r.getSigned(4))*factor, 6)
	s.LossEnd = pyRound(float64(r.getUint(4))*factor, 6)
	s.ORL = pyRound3(float64(r.getUint(2)) * 0.001)
	s.ORLStart = pyRound(float64(r.getSigned(4))*factor, 6)
	s.ORLFinish = pyRound(float64(r.getUint(4))*factor, 6)
	return nil
}

// pyRound3 复现 pyOTDR 的 "%.3f" 格式化往返。
func pyRound3(v float64) float64 { return pyRound(v, 3) }
