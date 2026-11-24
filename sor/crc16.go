package sor

// CRC-16/CCITT-FALSE: poly 0x1021, init 0xFFFF, no reflection, no xor-out.
// Equivalent to Python crcmod.predefined.Crc("crc-ccitt-false") used by pyOTDR.
func crc16Update(crc uint16, b []byte) uint16 {
	for _, x := range b {
		crc ^= uint16(x) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
