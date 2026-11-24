// Package sor 是 pyOTDR（github.com/sid5432/pyOTDR）解析核心的 Go 翻译。
//
// 解析 Telcordia/Bellcore GR-196 SOR 文件：Map 块、GenParams、SupParams、
// FxdParams、KeyEvents、DataPts、Cksum。数据点输出 (X=km, Y=dB) 曲线。
//
// 与 pyOTDR 的对应关系（文件级）：
//
//	pyotdr/read.py      → Parse / ParseFile
//	pyotdr/parts.py     → reader（含顺序 CRC）
//	pyotdr/mapblock.py  → parseMap
//	pyotdr/genparams.py → genparams
//	pyotdr/supparams.py → supparams
//	pyotdr/fxdparams.py → fxdparams
//	pyotdr/keyevents.py → keyevents
//	pyotdr/datapts.py   → datapts
//	pyotdr/cksum.py     → cksum
//
// 设计差异：Go 库保存类型化数值（float64/int/string），不保存 pyOTDR 的
// 展示用格式化字符串；数值语义（缩放、折射率修正公式）与 pyOTDR 一致。
// 已知局限同样保留：EXFO 私有块的校验和与顺序 CRC-CCITT-FALSE 不一致
// （pyOTDR 对 EXFO 文件同样报 match=false），不影响曲线与参数解析。
package sor

// Sol 光速，km/us（pyOTDR parts.sol = 299792.458 / 1e6）。
const Sol = 299792.458 / 1.0e6

// Block Map 块中的一个条目。
type Block struct {
	Name    string
	Version string // "2.00"
	Size    uint32
	Pos     int64
	Order   int
}

// MapBlock Map 块头部信息。
type MapBlock struct {
	NBytes  uint32 // Map 块总字节数（含头部与所有条目）
	NBlocks int    // 后续块数量（读到的值 - 1，与 pyOTDR 一致）
}

// GenParams 通用参数。
type GenParams struct {
	Language           string
	CableID            string
	FiberID            string
	FiberTypeCode      uint16
	FiberType          string // 解码后，如 "G.652 (standard SMF)"
	WavelengthNM       uint16
	LocationA          string
	LocationB          string
	CableCode          string
	BuildConditionCode string // "BC"/"CC"/"RC"/"OT"
	BuildCondition     string // 解码后，如 "BC (as-built)"
	UserOffset         int32
	UserOffsetDistance int32
	Operator           string
	Comments           string
}

// SupParams 供货参数。
type SupParams struct {
	Supplier string
	OTDR     string
	OTDRSN   string
	Module   string
	ModuleSN string
	Software string
	Other    string
}

// FxdParams 固定参数。数值字段的缩放与 pyOTDR fxdparams.py 一致。
type FxdParams struct {
	DateTimeSec              int64   // Unix 秒（UTC）
	Unit                     string  // "mt"/"km"/"mi"/"kf"
	WavelengthNM             float64 // uint16 * 0.1
	AcquisitionOffset        int32
	AcquisitionOffsetDist    int32
	NumPulseWidthEntries     uint16
	PulseWidthNS             uint16
	SampleSpacingUS          float64 // uint32 * 1e-8
	NumDataPoints            uint32
	Index                    float64 // uint32 * 1e-5（经 6 位字符串往返，与 pyOTDR 位级一致）
	BC                       float64 // int16 * -0.1
	NumAverages              uint32
	AveragingTimeSec         float64 // uint16 * 0.1
	RangeKM                  float64 // 原始字段 uint32 * 2e-5
	AcquisitionRangeDist     int32
	FrontPanelOffset         int32
	NoiseFloorLevel          uint16
	NoiseFloorScalingFactor  int16
	PowerOffsetFirstPoint    uint16
	LossThr                  float64 // uint16 * 0.001
	ReflThr                  float64 // int16 * -0.001
	EOTThr                   float64 // uint16 * 0.001
	TraceType                string  // "ST"/"RT"/"DT"/"RF"
	X1, Y1, X2, Y2           int32

	// 折射率修正（pyOTDR fxdparams._process_fields 末尾 corrections）：
	// dx = sample_spacing * Sol / IOR（km）；resolution = dx * 1000（米/点）；
	// adjusted range = dx * num_data_points（km）。
	Resolution      float64 // 米/点
	AdjustedRangeKM float64
}

// KeyEvent 事件。
type KeyEvent struct {
	ID            uint16
	Type          string // 解码后，如 "1F9999LS {auto} reflection"
	DistanceKM    float64
	Slope         float64 // dB/km
	SpliceLoss    float64 // dB
	ReflLoss      float64 // dB
	EndOfPrevKM   float64 // format 2
	StartOfCurrKM float64
	EndOfCurrKM   float64
	StartOfNextKM float64
	PeakKM        float64
	Comments      string
}

// EventSummary KeyEvents 块末尾的汇总。
type EventSummary struct {
	TotalLoss float64
	ORL       float64
	LossStart float64
	LossEnd   float64
	ORLStart  float64
	ORLFinish float64
}

// KeyEvents 关键事件集合。
type KeyEvents struct {
	NumEvents int
	Events    []KeyEvent
	Summary   EventSummary
}

// DataPts 数据点块头信息。
type DataPts struct {
	NumDataPoints   uint32
	NumTraces       int16
	NumDataPoints2  uint32
	ScalingFactor   float64 // uint16 / 1000
	MaxBeforeOffset float64 // 3 位舍入，dB
	MinBeforeOffset float64 // 3 位舍入，dB
	OffsetMode      string  // "STV"（默认）/"AFL"/"RAW"
	XScaling        float64 // 特殊机型修正，默认 1（OFL250 为 0.1）
}

// Point 曲线点：X 单位 km，Y 单位 dB。
type Point struct {
	X float64
	Y float64
}

// File 一个 SOR 文件的完整解析结果。
type File struct {
	Filename string
	Format   int    // 1 或 2（"Map" 头为 2）
	Version  string // "2.00"
	Map      MapBlock
	Blocks   []Block

	Gen  GenParams
	Sup  SupParams
	Fxd  FxdParams
	Data DataPts
	Keys KeyEvents

	Checksum      uint16 // 文件存储的校验和
	ChecksumOurs  uint16 // 顺序读取累积的 CRC-CCITT-FALSE
	ChecksumMatch bool

	Trace []Point // 曲线（长度 = NumDataPoints）
}

// DateTimeUTC 测量时间（Unix 秒 → UTC）。
func (f *FxdParams) DateTimeUTC() (sec int64) { return f.DateTimeSec }
