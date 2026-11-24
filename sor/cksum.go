package sor

// cksum 校验和块（对应 pyOTDR cksum.py）：先取累计 CRC，再读文件存储的校验和。
func cksum(r *reader, f *File) error {
	if f.Format == 2 {
		if err := expectHeader(r, "Cksum"); err != nil {
			return err
		}
	}
	f.ChecksumOurs = r.crc // digest 在读取存储值之前
	f.Checksum = uint16(r.getUint(2))
	f.ChecksumMatch = f.ChecksumOurs == f.Checksum
	return nil
}
