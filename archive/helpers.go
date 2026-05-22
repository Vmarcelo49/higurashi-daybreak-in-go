package archive

import (
	"fmt"
	"io"
	"os"
	"time"

	"bundleTools/convert"
	"bundleTools/crypto"
	"bundleTools/fileutil"
)

// readAndDecrypt reads `length` bytes from `f` at `offset` and XOR-decrypts
// them using the canonical file key from the crypto package.
func readAndDecrypt(f *os.File, offset uint32, length uint32) ([]byte, error) {
	if _, err := f.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, fmt.Errorf("error seeking to %d: %w", offset, err)
	}

	buf := make([]byte, int(length))
	n, err := f.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading %d bytes at %d: %w", length, offset, err)
	}
	if n != int(length) {
		return nil, fmt.Errorf("expected to read %d bytes but read %d", length, n)
	}

	key := crypto.GetFileKey(int64(offset))
	for i := 0; i < n; i++ {
		buf[i] = buf[i] ^ byte(key)
	}
	return buf, nil
}

// encryptBytesWithOffset returns a new slice with data XOR-encrypted using
// the canonical file key for the provided offset.
func encryptBytesWithOffset(data []byte, offset uint32) []byte {
	out := make([]byte, len(data))
	key := crypto.GetFileKey(int64(offset))
	for i := range data {
		out[i] = data[i] ^ byte(key)
	}
	return out
}

// handleCNVExtraction centralizes CNV -> wav/bmp/unknown conversion logic.
// It returns the (possibly converted) data and the desired output filename
// (with extension adjusted) or an error.
func handleCNVExtraction(decrypted []byte, name string) ([]byte, string, error) {
	if !fileutil.HasExtCI(name, ".cnv") {
		return decrypted, name, nil
	}
	if len(decrypted) == 0 {
		return decrypted, fileutil.ChangeExt(name, ".unknown"), nil
	}

	dataKey := decrypted[0]
	switch dataKey {
	case 1:
		var convErr error
		// protect against panics coming from conversion
		func() {
			defer func() {
				if r := recover(); r != nil {
					convErr = fmt.Errorf("WAV conversion panicked: %v", r)
				}
			}()
			convErr = convert.ConvertWAV(&decrypted)
		}()
		if convErr != nil {
			return decrypted, fileutil.ChangeExt(name, ".unknown"), convErr
		}
		return decrypted, fileutil.ChangeExt(name, ".wav"), nil
	case 24, 32:
		out, err := convert.ConvertBytesToBMP(decrypted)
		if err != nil {
			return nil, fileutil.ChangeExt(name, ".unknown"), err
		}
		return out, fileutil.ChangeExt(name, ".bmp"), nil
	default:
		return decrypted, fileutil.ChangeExt(name, ".unknown"), nil
	}
}

// preparePatchFile creates a .patched copy and a timestamped .bak backup.
// It returns the patched path and (if created) the backup path.
// If backup creation fails, the function still returns the patched path and
// a nil error; the caller may choose how to proceed.
func preparePatchFile(datPath string) (patchedPath string, backupPath string, err error) {
	sourceData, err := os.ReadFile(datPath)
	if err != nil {
		return "", "", fmt.Errorf("unable to read source DAT file %s: %w", datPath, err)
	}

	patchedPath = fmt.Sprintf("%s.patched", datPath)
	if err := os.WriteFile(patchedPath, sourceData, 0644); err != nil {
		return "", "", fmt.Errorf("unable to create patched file %s: %w", patchedPath, err)
	}

	timeStamp := time.Now().Format("20060102-150405")
	backupPath = fmt.Sprintf("%s.%s.bak", datPath, timeStamp)
	if err := os.WriteFile(backupPath, sourceData, 0644); err != nil {
		// do not fail hard on backup creation; return patchedPath and empty backupPath
		return patchedPath, "", nil
	}
	return patchedPath, backupPath, nil
}

// readInputPossiblyConvertToCNV reads the input file and, if the target
// entry expects CNV data and the input is a BMP, converts it back to CNV.
func readInputPossiblyConvertToCNV(inputPath string, targetIsCNV bool) ([]byte, error) {
	if targetIsCNV {
		_, ext := fileutil.GetLowerBaseAndExt(inputPath)
		if ext == ".bmp" {
			return convert.BMPtoCNV(inputPath)
		}
	}
	return os.ReadFile(inputPath)
}
