// Package tools provides internal utilities including the stress-test generator
// for SL1C3D-L4BS validation (O(1) memory, throughput).

package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// TargetStressSize is the intended size of the generated X12 837 stress file (~1.2GB).
	TargetStressSize = 1200 * 1024 * 1024
	// TargetClaimCount is the approximate number of CLM (claim) loops in the stress file (~2M).
	TargetClaimCount = 2_000_000
)

// X12 837 delimiters (standard).
const (
	segTerm = "~"
	elemSep = "*"
)

// GenStress837 writes a synthetic X12 837 (Healthcare Claims) file to out until
// the byte count reaches targetBytes (e.g. 1.2GB). Each iteration writes one
// structurally valid transaction set containing a single CLM (claim) so that
// X12Reader yields one row per iteration. Uses standard delimiters * and ~.
func GenStress837(out io.Writer, targetBytes int64) (written int64, claims int, err error) {
	if targetBytes <= 0 {
		targetBytes = TargetStressSize
	}
	// One transaction block: ISA...IEA with one ST/BHT/NM1/CLM/SE inside.
	// Sized so that (targetBytes / blockSize) ≈ 2M → blockSize ≈ 600.
	block := buildOneInterchange(0)
	blockLen := int64(len(block))
	bw := bufio.NewWriterSize(out, 1024*1024)
	defer func() {
		if fl := bw.Flush(); err == nil && fl != nil {
			err = fl
		}
	}()

	for written < targetBytes {
		n, wErr := bw.Write(block)
		if wErr != nil {
			return written, claims, wErr
		}
		written += int64(n)
		claims++
		// Regenerate block every 100k claims to vary control numbers (optional, keeps validity)
		if claims%100000 == 0 {
			block = buildOneInterchange(claims)
			blockLen = int64(len(block))
		}
		if blockLen <= 0 {
			break
		}
	}
	return written, claims, nil
}

// buildOneInterchange returns one complete X12 837 interchange (ISA through IEA)
// with a single claim (CLM) so the state machine yields one row per interchange.
func buildOneInterchange(seq int) []byte {
	// Control numbers: unique per interchange for validity
	isaCtrl := 1 + (seq % 999999999)
	stCtrl := 1 + (seq % 999999999)
	gsCtrl := 1 + (seq % 999999999)
	clmId := fmt.Sprintf("CLM%07d", seq%10000000)
	// Fixed-size segments to keep block ~600 bytes for ~2M claims → ~1.2GB
	isa := "ISA*00*          *00*          *ZZ*SENDER123456  *ZZ*RECEIVER456789*230101*1200*^*00501*" + fmt.Sprintf("%09d", isaCtrl) + "*0*P*:~"
	gs := "GS*HC*SENDER*RECEIVER*20230101*1200*" + fmt.Sprintf("%d", gsCtrl) + "*X*005010X222A1~"
	st := "ST*837*" + fmt.Sprintf("%07d", stCtrl) + "~"
	bht := "BHT*0019*00*REF" + fmt.Sprintf("%06d", seq%1000000) + "*20230101*1200*CH~"
	nm1a := "NM1*41*2*PAYER* *** *** *46*PAYERID123~"
	nm1b := "NM1*40*2*PROVIDER* *** *** *46*PROVID123~"
	clm := "CLM*" + clmId + "*1500.00*11*HC:95004~"
	se := "SE*7*" + fmt.Sprintf("%07d", stCtrl) + "~"
	ge := "GE*1*" + fmt.Sprintf("%d", gsCtrl) + "~"
	iea := "IEA*1*" + fmt.Sprintf("%09d", isaCtrl) + "~"

	s := isa + gs + st + bht + nm1a + nm1b + clm + se + ge + iea
	return []byte(s)
}

// WriteStressFile creates a file at path and fills it with synthetic X12 837
// up to targetBytes (default 1.2GB). Returns total bytes written and claim count.
func WriteStressFile(path string, targetBytes int64) (written int64, claims int, err error) {
	if targetBytes <= 0 {
		targetBytes = TargetStressSize
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, 0, err
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	return GenStress837(f, targetBytes)
}
