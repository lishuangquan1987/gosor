// sordump 解析 SOR 文件并输出 JSON 摘要与曲线 CSV，用于与 pyOTDR 输出做一致性校验。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gosor/sor"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sordump <file.sor> [out.json] [out.csv]")
		os.Exit(2)
	}
	path := os.Args[1]
	f, err := sor.ParseFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	out := map[string]any{
		"filename":      f.Filename,
		"format":        f.Format,
		"version":       f.Version,
		"map":           f.Map,
		"blocks":        f.Blocks,
		"GenParams":     f.Gen,
		"SupParams":     f.Sup,
		"FxdParams":     f.Fxd,
		"DataPts":       f.Data,
		"KeyEvents":     f.Keys,
		"checksum":      f.Checksum,
		"checksum_ours": f.ChecksumOurs,
		"match":         f.ChecksumMatch,
		"trace_points":  len(f.Trace),
	}
	outName := "dump-go.json"
	if len(os.Args) > 2 {
		outName = os.Args[2]
	}
	jf, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(outName, jf, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if len(os.Args) > 3 {
		csvName := os.Args[3]
		var b []byte
		for _, pt := range f.Trace {
			b = append(b, []byte(fmt.Sprintf("%f\t%f\n", pt.X, pt.Y))...)
		}
		if err := os.WriteFile(csvName, b, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	fxd := f.Fxd
	fmt.Printf("parsed %s: format=%d version=%s points=%d wavelength=%.1fnm resolution=%.6fm range=%.6fkm date=%s checksum_match=%v\n",
		f.Filename, f.Format, f.Version, len(f.Trace), fxd.WavelengthNM, fxd.Resolution,
		fxd.AdjustedRangeKM, time.Unix(fxd.DateTimeSec, 0).UTC().Format(time.RFC3339), f.ChecksumMatch)
}
