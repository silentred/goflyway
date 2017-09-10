package proxy

import (
	. "github.com/silentred/goflyway/config"
	"github.com/silentred/goflyway/logg"

	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
)

const IV_LENGTH = 16

var primes = []int16{
	11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61, 67, 71,
	73, 79, 83, 89, 97, 101, 103, 107, 109, 113, 127, 131, 137, 139, 149, 151,
	157, 163, 167, 173, 179, 181, 191, 193, 197, 199, 211, 223, 227, 229, 233, 239,
	241, 251, 257, 263, 269, 271, 277, 281, 283, 293, 307, 311, 313, 317, 331, 337,
	347, 349, 353, 359, 367, 373, 379, 383, 389, 397, 401, 409, 419, 421, 431, 433,
	439, 443, 449, 457, 461, 463, 467, 479, 487, 491, 499, 503, 509, 521, 523, 541,
	547, 557, 563, 569, 571, 577, 587, 593, 599, 601, 607, 613, 617, 619, 631, 641,
	643, 647, 653, 659, 661, 673, 677, 683, 691, 701, 709, 719, 727, 733, 739, 743,
	751, 757, 761, 769, 773, 787, 797, 809, 811, 821, 823, 827, 829, 839, 853, 857,
	859, 863, 877, 881, 883, 887, 907, 911, 919, 929, 937, 941, 947, 953, 967, 971,
	977, 983, 991, 997, 1009, 1013, 1019, 1021, 1031, 1033, 1039, 1049, 1051, 1061, 1063, 1069,
	1087, 1091, 1093, 1097, 1103, 1109, 1117, 1123, 1129, 1151, 1153, 1163, 1171, 1181, 1187, 1193,
	1201, 1213, 1217, 1223, 1229, 1231, 1237, 1249, 1259, 1277, 1279, 1283, 1289, 1291, 1297, 1301,
	1303, 1307, 1319, 1321, 1327, 1361, 1367, 1373, 1381, 1399, 1409, 1423, 1427, 1429, 1433, 1439,
	1447, 1451, 1453, 1459, 1471, 1481, 1483, 1487, 1489, 1493, 1499, 1511, 1523, 1531, 1543, 1549,
	1553, 1559, 1567, 1571, 1579, 1583, 1597, 1601, 1607, 1609, 1613, 1619, 1621, 1627, 1637, 1657,
}

type InplaceCTR struct {
	b       cipher.Block
	ctr     []byte
	out     []byte
	outUsed int
}

const streamBufferSize = 512

// From src/crypto/cipher/ctr.go
func (x *InplaceCTR) XorBuffer(buf []byte) {
	for i := 0; i < len(buf); i++ {
		if x.outUsed >= len(x.out)-x.b.BlockSize() {
			// refill
			remain := len(x.out) - x.outUsed
			copy(x.out, x.out[x.outUsed:])
			x.out = x.out[:cap(x.out)]
			bs := x.b.BlockSize()
			for remain <= len(x.out)-bs {
				x.b.Encrypt(x.out[remain:], x.ctr)
				remain += bs

				// Increment counter
				for i := len(x.ctr) - 1; i >= 0; i-- {
					x.ctr[i]++
					if x.ctr[i] != 0 {
						break
					}
				}
			}
			x.out = x.out[:remain]
			x.outUsed = 0
		}

		buf[i] ^= x.out[x.outUsed]
		x.outUsed++
	}
}

func GetCipherStream(key []byte) *InplaceCTR {
	if key == nil {
		return nil
	}

	if len(key) != IV_LENGTH {
		logg.E("[AES] iv is not 128bit long")
		return nil
	}

	return &InplaceCTR{
		b:       G_KeyBlock,
		ctr:     key,
		out:     make([]byte, 0, streamBufferSize),
		outUsed: 0,
	}
}

func _AXor(blk cipher.Block, iv, buf []byte) []byte {
	bsize := blk.BlockSize()
	x := make([]byte, len(buf)/bsize*bsize+bsize)

	for i := 0; i < len(x); i += bsize {
		blk.Encrypt(x[i:], iv)

		for i := len(iv) - 1; i >= 0; i-- {
			if iv[i]++; iv[i] != 0 {
				break
			}
		}
	}

	for i := 0; i < len(buf); i++ {
		buf[i] ^= x[i]
	}

	return buf
}

func generateIV(s, s2 byte) []byte {
	ret := make([]byte, IV_LENGTH)

	var mul uint32 = uint32(primes[s]) * uint32(primes[s2])
	var seed uint32 = binary.LittleEndian.Uint32(G_KeyBytes[:4])

	for i := 0; i < IV_LENGTH/4; i++ {
		seed = (mul * seed) % 0x7fffffff
		binary.LittleEndian.PutUint32(ret[i*4:], seed)
	}

	return ret
}

func AEncrypt(buf []byte) []byte {
	r := NewRand()
	b, b2 := byte(r.Intn(256)), byte(r.Intn(256))
	return append(_AXor(G_KeyBlock, generateIV(b, b2), buf), b, b2)
}

func ADecrypt(buf []byte) []byte {
	if len(buf) < 2 {
		return buf
	}

	b, b2 := byte(buf[len(buf)-2]), byte(buf[len(buf)-1])
	return _AXor(G_KeyBlock, generateIV(b, b2), buf[:len(buf)-2])
}

func AEncryptString(text string) string {
	return hex.EncodeToString(AEncrypt([]byte(text)))
}

func ADecryptString(text string) string {
	buf, err := hex.DecodeString(text)
	if err != nil {
		return ""
	}

	return string(ADecrypt(buf))
}
