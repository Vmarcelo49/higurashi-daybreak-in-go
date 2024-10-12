package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

// Go translation of https://github.com/HigurashiArchive/higurashi-daybreak/blob/master/bundle-tools.pl

// listBundle lê um arquivo de pacote e imprime os dados da tabela.
func listBundle(bundlePath string) error {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", bundlePath)
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", bundlePath, err)
	}
	defer file.Close()

	_, list, err := getTableData(file)
	if err != nil {
		return fmt.Errorf("error getting table data: %w", err)
	}

	for _, item := range list {
		itemStr := ""
		for k, v := range item {
			itemStr += fmt.Sprintf("%s: %v, ", k, v)
		}
		// Remove a última vírgula e espaço
		if len(itemStr) > 0 {
			itemStr = itemStr[:len(itemStr)-2]
		}
		fmt.Println("  ", itemStr)
	}

	return nil
}

func convertImage(data *[]byte) error {
	const headerSize = 17
	if len(*data) < headerSize {
		return errors.New("data is too short to read image header")
	}

	// Unpack image header
	bpp := (*data)[0]
	width := binary.LittleEndian.Uint32((*data)[1:5])
	height := binary.LittleEndian.Uint32((*data)[5:9])
	width2 := binary.LittleEndian.Uint32((*data)[9:13])
	zero := (*data)[headerSize-1]

	// Check width consistency
	if width != width2 {
		fmt.Printf(" *** Warning ----: Two width values disagree: %d %d\n", width, width2)
	}

	// Check bits per pixel
	if bpp != 24 && bpp != 32 {
		return fmt.Errorf("BPP must be 24 or 32, not %d", bpp)
	}

	// Check data length consistency
	if int(width2)*int(height)*4+headerSize != len(*data) {
		return fmt.Errorf("data lengths disagree: %d vs %d", int(width2)*int(height)*4+headerSize, len(*data))
	}

	if zero != 0 {
		return errors.New("nonzero value in final header block")
	}

	// Prepare output data
	outData := make([]byte, 0)
	outData = append(outData, 0, 0, 2)    // Header
	outData = append(outData, 0, 0, 0, 0) // No image ID
	outData = append(outData, 0, 0)       // No color map
	outData = append(outData, byte(width), byte(width>>8), byte(height), byte(height>>8))
	outData = append(outData, 32, 0x08) // Bits per pixel and image descriptor

	// Reverse row order for output
	for r := int(height) - 1; r >= 0; r-- {
		for c := 0; c < int(width2); c++ {
			start := headerSize + 4*(int(r)*int(width2)+c)
			if start+4 > len(*data) {
				return errors.New("data overflow while reading pixels")
			}
			outData = append(outData, (*data)[start:start+4]...)
		}
	}

	*data = outData
	return nil
}

func convertWav(data *[]byte) error {
	const headerSize = 22
	if len(*data) < headerSize {
		return errors.New("data is too short to read WAV header")
	}

	// Unpack WAV header
	audioFmt := binary.LittleEndian.Uint16((*data)[0:2])
	nChannels := binary.LittleEndian.Uint16((*data)[2:4])
	sampleRate := binary.LittleEndian.Uint32((*data)[4:8])
	byteRate := binary.LittleEndian.Uint32((*data)[8:12])
	blockAlign := binary.LittleEndian.Uint16((*data)[12:14])
	bitsPerSample := binary.LittleEndian.Uint16((*data)[14:16])
	subchunk2Size := binary.LittleEndian.Uint32((*data)[16:20])

	// Check subchunk size
	if subchunk2Size != uint32(len(*data))-headerSize {
		fmt.Printf(" *** Warning ----: Size mismatch: %d vs %d.\n", subchunk2Size, len(*data)-headerSize)
	}

	// Check byte rate
	if byteRate != (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)) {
		return fmt.Errorf("byte rate mismatch: %d vs %d", byteRate, (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)))
	}

	// Check block align
	if blockAlign != nChannels*(bitsPerSample/8) {
		return fmt.Errorf("block align mismatch: %d vs %d", blockAlign, nChannels*(bitsPerSample/8))
	}

	// Pack new WAV data
	outData := make([]byte, 0)

	// RIFF header
	outData = append(outData, []byte("RIFF")...)
	outData = append(outData, make([]byte, 4)...)
	binary.LittleEndian.PutUint32(outData[4:], subchunk2Size+36)
	outData = append(outData, []byte("WAVE")...)

	// fmt subchunk
	outData = append(outData, []byte("fmt ")...)
	outData = append(outData, 16, 0, 0, 0) // Subchunk size
	binary.LittleEndian.PutUint16(outData[16:], audioFmt)
	binary.LittleEndian.PutUint16(outData[18:], nChannels)
	binary.LittleEndian.PutUint32(outData[20:], sampleRate)
	binary.LittleEndian.PutUint32(outData[24:], byteRate)
	binary.LittleEndian.PutUint16(outData[28:], blockAlign)
	binary.LittleEndian.PutUint16(outData[30:], bitsPerSample)

	// Data subchunk
	outData = append(outData, []byte("data")...)
	binary.LittleEndian.PutUint32(outData[36:], subchunk2Size)
	outData = append(outData, (*data)[headerSize:]...)

	*data = outData
	return nil
}

func extractBundle(bundlePath, extractPath, pattern string) error {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", bundlePath)
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("unable to open %s: %w", bundlePath, err)
	}
	defer file.Close()

	_, list, err := getTableData(file) // _ was hash and was unused
	if err != nil {
		return fmt.Errorf("failed to get table data: %w", err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	for _, h := range list {
		name, ok := h["name"].(string)
		if !ok || !re.MatchString(name) {
			continue
		}

		fmt.Printf("  %+v\n", h)

		// Verifica h["offset"]
		offsetVal, ok := h["offset"]
		if !ok {
			return fmt.Errorf("offset key not found in bundle for %s", name)
		}

		offset, ok := offsetVal.(uint32)
		if !ok {
			return fmt.Errorf("invalid offset value in bundle for %s, got type %T", name, offsetVal)
		}

		// Verifica h["length"]
		lengthVal, ok := h["length"]
		if !ok {
			return fmt.Errorf("length key not found in bundle for %s", name)
		}

		length, ok := lengthVal.(uint32)
		if !ok {
			return fmt.Errorf("invalid length value in bundle for %s, got type %T", name, lengthVal)
		}

		// Mover o cursor do arquivo para a posição correta
		if _, err = file.Seek(int64(offset), 0); err != nil {
			return fmt.Errorf("error seeking in bundle: %w", err)
		}

		// Ler os dados do arquivo
		data := make([]byte, length)
		n, err := file.Read(data)
		if err != nil {
			return fmt.Errorf("error extracting from bundle: %w", err)
		}
		if n != int(length) {
			return fmt.Errorf("expected to read %d bytes, but got %d", length, n)
		}

		key := getFileKey(int64(offset))
		var str2 []byte
		for i := 0; i < int(length); i++ {
			str2 = append(str2, data[i]^byte(key))
		}

		outName := extractPath + string(os.PathSeparator) + name

		// Criar diretórios conforme necessário para o outName
		dir := filepath.Dir(outName)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating directory for %s: %w", outName, err)
		}

		if name[len(name)-4:] == ".cnv" {
			key := str2[0]

			if key == 1 {
				err = convertWav(&str2)
				if err != nil {
					return fmt.Errorf("error converting wav: %w", err)
				}
				outName = outName[:len(outName)-4] + ".wav"
			} else if key == 24 || key == 32 {
				err = convertImage(&str2)
				if err != nil {
					return fmt.Errorf("error converting image: %w", err)
				}
				outName = outName[:len(outName)-4] + ".tga"
			} else {
				fmt.Printf("Bad data key (%d) in %s\n", key, outName)
			}
		}

		// Escrever os dados convertidos em um novo arquivo
		err = os.WriteFile(outName, str2, 0644)
		if err != nil {
			return fmt.Errorf("unable to write %s: %w", outName, err)
		}
	}
	return nil
}

func readFileIn(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to read file: %w", err)
	}

	ext := path.Ext(filename)
	if len(data) >= 44 && ext == ".wav" {
		fmt.Println("Converting wav to cnv")
		var (
			chunkID       [4]byte
			chunkSize     = binary.LittleEndian.Uint32(data[4:8])
			format        [4]byte
			sub1ID        [4]byte
			sub1Size      = binary.LittleEndian.Uint32(data[16:20])
			audioFormat   = binary.LittleEndian.Uint16(data[20:22])
			nChannels     = binary.LittleEndian.Uint16(data[22:24])
			sampleRate    = binary.LittleEndian.Uint32(data[24:28])
			byteRate      = binary.LittleEndian.Uint32(data[28:32])
			blockAlign    = binary.LittleEndian.Uint16(data[32:34])
			bitsPerSample = binary.LittleEndian.Uint16(data[34:36])
			sub2ID        [4]byte
			sub2Size      = binary.LittleEndian.Uint32(data[40:44])
		)

		// Unpack WAV header
		copy(chunkID[:], data[0:4])
		copy(format[:], data[8:12])
		copy(sub1ID[:], data[12:16])
		copy(sub2ID[:], data[36:40])

		if string(chunkID[:]) != "RIFF" || string(format[:]) != "WAVE" ||
			string(sub1ID[:]) != "fmt " || sub1Size != 16 ||
			string(sub2ID[:]) != "data" || chunkSize != sub2Size+36 {
			return nil, fmt.Errorf("bad headers on .wav")
		}
		if sub2Size != uint32(len(data)-44) {
			return nil, fmt.Errorf("size mismatch: %d vs. %d", sub2Size, len(data)-44)
		}
		if sampleRate != 44100 {
			return nil, fmt.Errorf("can only use 44100 bps wavs, not %d", sampleRate)
		}
		if byteRate != (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)) {
			return nil, fmt.Errorf("byte rate mismatch: %d vs. %d", byteRate, (sampleRate * uint32(nChannels) * (uint32(bitsPerSample) / 8)))
		}
		if blockAlign != nChannels*(bitsPerSample/8) {
			return nil, fmt.Errorf("block align mismatch: %d vs. %d", blockAlign, nChannels*(bitsPerSample/8))
		}

		// Pack and return new data
		outData := make([]byte, 0, 44+sub2Size) // Preallocate size for better performance
		outData = append(outData, pack("vvVVvvvV", audioFormat, nChannels, sampleRate, byteRate, blockAlign, bitsPerSample, 0, sub2Size)...)
		outData = append(outData, data[44:]...)
		return outData, nil

	} else if ext == ".tga" {
		fmt.Println("Converting tga to cnv")
		if len(data) < 18 {
			return nil, fmt.Errorf("tga file too short")
		}

		var width, height uint16
		bpp := uint16(data[16])
		trans := uint16(data[17])

		// Unpack TGA header
		width = binary.LittleEndian.Uint16(data[12:14])
		height = binary.LittleEndian.Uint16(data[14:16])

		if data[0] != 0 || data[1] != 0 || data[2] != 2 || data[3] != 0 {
			return nil, fmt.Errorf("bad headers on .tga")
		}
		if bpp != 32 {
			return nil, fmt.Errorf(".tga not 32 bpp")
		}
		if trans != 0x08 {
			return nil, fmt.Errorf(".tga has wrong transparency")
		}

		outData := make([]byte, 0, 18+4*width*height)
		outData = append(outData, byte(bpp), byte(bpp>>8))
		outData = append(outData, byte(width), byte(width>>8))
		outData = append(outData, byte(height), byte(height>>8))
		outData = append(outData, byte(width), byte(width>>8), 0)

		for r := int(height) - 1; r >= 0; r-- {
			for c := 0; c < int(width); c++ {
				start := 18 + 4*(uint16(r)*width+uint16(c))
				outData = append(outData, data[start:start+4]...)
			}
		}
		return outData, nil

	} else {
		fmt.Println("Not converting")
		return data, nil
	}
}

// pack empacota dados, similar ao pack em Perl (simplificada)
func pack(values ...interface{}) []byte {
	var out []byte
	for _, v := range values {
		switch val := v.(type) {
		case uint16:
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, val)
			out = append(out, b...)
		case uint32:
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, val)
			out = append(out, b...)
		}
	}
	return out
}

// hexifyStr converte um slice de bytes para uma string de valores hexadecimais separados por espaços.
func hexifyStr(data []byte) string {
	hexValues := make([]string, len(data))
	for i, b := range data {
		hexValues[i] = fmt.Sprintf("%02x", b) // Convertendo cada byte para hexadecimal
	}
	return strings.Join(hexValues, " ") // Unindo os valores com espaços
}

// getFileKey calcula a chave a partir de um deslocamento.
func getFileKey(offset int64) byte {
	return byte((offset>>1)&0xff | 0x08)
}

func patchFile(OFH *os.File, inFileName string, ftable map[string]map[string]interface{}) error {
	fileEntry, exists := ftable[inFileName]
	if !exists {
		return fmt.Errorf("file entry not found in ftable for %s", inFileName)
	}

	rData, err := readFileIn(inFileName)
	if err != nil {
		return fmt.Errorf("error reading input file: %v", err)
	}
	datLen := len(rData)

	if datLen <= fileEntry["length"].(int) {
		fmt.Printf("Seeking to %d.\n", fileEntry["offset"].(int64))
		_, err := OFH.Seek(fileEntry["offset"].(int64), io.SeekStart)
		if err != nil {
			return fmt.Errorf("unable to seek: %v", err)
		}
	} else {
		fmt.Println("Seeking to end")
		_, err := OFH.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("unable to seek: %v", err)
		}
	}

	// Obter o novo offset
	newOffset, err := OFH.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("unable to get current offset: %v", err)
	}
	fmt.Printf("Updating at %d\n", newOffset)

	// Obter a chave de criptografia
	key := getFileKey(newOffset)

	// Criptografar os dados
	outData := make([]byte, datLen)
	for i := 0; i < datLen; i++ {
		outData[i] = rData[i] ^ byte(key)
	}

	// Escrever os dados criptografados no arquivo de saída
	cPrinted, err := OFH.Write(outData)
	if err != nil || cPrinted != datLen {
		return fmt.Errorf("error writing output data: %v", err)
	}

	// Atualizar o ftable se necessário
	if datLen != fileEntry["length"].(int) || newOffset != fileEntry["offset"].(int64) {
		fIndex := 268*fileEntry["index"].(int) + 260 + 2
		_, err := OFH.Seek(int64(fIndex), io.SeekStart)
		if err != nil {
			return fmt.Errorf("unable to seek: %v", err)
		}

		fmt.Printf("Updating file_table from %d, %d to %d, %d\n", fileEntry["length"], fileEntry["offset"], datLen, newOffset)

		// Ler o bloco antigo (8 bytes)
		oldBlock := make([]byte, 8) // Tamanho deve ser adequado para o que você espera ler
		_, err = OFH.Read(oldBlock)
		if err != nil {
			return fmt.Errorf("error reading file_table block: %v", err)
		}
		fmt.Printf("old was (%s)\n", hexifyStr(oldBlock))

		// Empacotar length e offset em um slice de bytes (little endian)
		newBlock := make([]byte, 8)
		newBlock[0] = byte(datLen & 0xFF)           // lower byte of length
		newBlock[1] = byte((datLen >> 8) & 0xFF)    // higher byte of length
		newBlock[2] = byte(newOffset & 0xFF)        // lower byte of offset
		newBlock[3] = byte((newOffset >> 8) & 0xFF) // higher byte of offset
		// O código acima deve ser ajustado se `length` e `offset` forem 32 bits
		// e necessitarem de mais bytes para armazenar.

		// Atualizar o bloco de tabela de arquivos
		_, err = OFH.Write(decryptFileTableBlock(fIndex-2, newBlock))
		if err != nil {
			return fmt.Errorf("error writing new block: %v", err)
		}

		// Atualizar o ftable com os novos valores
		fileEntry["offset"] = newOffset
		fileEntry["length"] = datLen
	}

	return nil
}

// Função auxiliar para substituir a extensão de um arquivo
func replaceExtension(fileName, oldExt, newExt string) string {
	if filepath.Ext(fileName) == oldExt {
		return fileName[:len(fileName)-len(oldExt)] + newExt
	}
	return fileName
}

// Função auxiliar para calcular a idade do arquivo em dias
func fileAgeInDays(modTime time.Time) float64 {
	return time.Since(modTime).Hours() / 24
}

func rpatchDir(OFH *os.File, fullPath string, localPath string, hTable map[string]map[string]interface{}, bundleMtime float64) {
	dir, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Error opening directory %s: %v\n", fullPath, err)
		return
	}
	defer dir.Close()

	files, err := dir.Readdirnames(0)
	if err != nil {
		fmt.Printf("Error reading directory %s: %v\n", fullPath, err)
		return
	}

	// Percorrer os arquivos no diretório
	for _, fn := range files {
		// Ignorar "." e ".."
		if fn == "." || fn == ".." {
			continue
		}

		fullName := filepath.Join(fullPath, fn)
		localName := filepath.Join(localPath, fn)
		inName := localName

		// Substituir as extensões .wav e .tga por .cnv
		inName = replaceExtension(inName, ".wav", ".cnv")
		inName = replaceExtension(inName, ".tga", ".cnv")

		// Se for um diretório, chamar recursivamente
		fileInfo, err := os.Stat(fullName)
		if err != nil {
			fmt.Printf("Error getting file info for %s: %v\n", fullName, err)
			continue
		}

		if fileInfo.IsDir() {
			rpatchDir(OFH, fullName, localName+"/", hTable, bundleMtime)
		} else if hTable[inName] != nil {
			fmt.Printf("**** PATCH FILE %s ****\n", localName)

			// Verificar se o arquivo no diretório é mais recente
			fileMtime := fileInfo.ModTime()
			if bundleMtime < fileAgeInDays(fileMtime) {
				fmt.Println("File is older; skipping")
				continue
			}

			// Tentar fazer o patch no arquivo
			err = patchFile(OFH, fullName, hTable)
			if err != nil {
				fmt.Printf("Error patching file %s: %v\n", fullName, err)
			}
		} else {
			fmt.Printf("Ignoring file %s\n", localName)
		}
	}
}

// decryptFileTableBlock decrypts the file table block
func decryptFileTableBlock(index int, str []byte) []byte {
	// Aplicar a máscara de 9 bits
	index = index & 0x1ff

	// Inicializar ctr e key
	ctr := (100 + index*77) & 0xff
	key := (100*(index+1) + ((0xff & (index * (index - 1) / 2)) * 77)) & 0xff

	// Criar o slice para armazenar o resultado
	rv := make([]byte, len(str))

	// Loop para decriptar cada byte
	for i := 0; i < len(str); i++ {
		// Aplicar XOR com a chave
		rv[i] = str[i] ^ byte(key)

		// Atualizar key e ctr
		key = (key + ctr) & 0xff
		ctr = (ctr + 77) & 0xff
	}

	return rv
}

// getTableData reads the table data from the file and returns a map of the table and a slice of the table
func getTableData(IFH *os.File) (map[string]map[string]interface{}, []map[string]interface{}, error) {
	// Mover para o início do arquivo
	if _, err := IFH.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("error seeking to start of file: %v", err)
	}

	data := make([]byte, 2)
	n, err := IFH.Read(data)
	if err != nil || n != len(data) {
		return nil, nil, fmt.Errorf("error reading table length: %v", err)
	}
	nFiles := binary.LittleEndian.Uint16(data)

	// Ler a tabela de arquivos
	data = make([]byte, 268*int(nFiles))
	n, err = IFH.Read(data)
	if err != nil || n != len(data) {
		return nil, nil, fmt.Errorf("error reading table: %v", err)
	}

	data2 := decryptFileTableBlock(0, data)

	// Tabelas que serão retornadas
	table := []map[string]interface{}{}
	tableMap := map[string]map[string]interface{}{}

	// Loop para processar cada arquivo
	for i := 0; i < int(nFiles); i++ {
		entry := data2[i*268 : (i+1)*268]
		fname := string(entry[:260])
		length := binary.LittleEndian.Uint32(entry[260:264])
		offset := binary.LittleEndian.Uint32(entry[264:268])

		// Remover null bytes do fname
		fname = strings.TrimRight(fname, "\x00")

		// Decodificar shift_jis
		decodedFnameData := transform.NewReader(strings.NewReader(fname), japanese.ShiftJIS.NewDecoder())
		decodedFname, err := io.ReadAll(decodedFnameData)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding shift_jis: %v", err)
		}

		// Criar a entrada na tabela
		decodedFnameStr := string(decodedFname)
		tmp := map[string]interface{}{
			"index":  i,
			"offset": offset,
			"length": length,
			"name":   decodedFnameStr,
		}

		table = append(table, tmp)
		tableMap[decodedFnameStr] = tmp
	}

	return tableMap, table, nil
}

// decryptFileTableBlock decrypts the file table block
func patchBundle(datFilePath string, outputPath string) { // original line had 3 arguments

	//var IFH *os.File
	var outputFile *os.File

	fileInfo, err := os.Stat(datFilePath)
	if err != nil {
		log.Fatal(err)
	}
	// Get the last modified time of the file
	mtime := time.Since(fileInfo.ModTime()).Hours() / 24

	outputFile, err = os.OpenFile(datFilePath, os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Unable to open %s for writing: %v", datFilePath, err)
	}

	hTable, _, err := getTableData(outputFile)
	if err != nil {
		log.Fatalf("Unable to get table data: %v", err)
	}

	rpatchDir(outputFile, outputPath, "", hTable, mtime)
}

func main() {
	args := os.Args[1:]
	for i, arg := range args {
		fmt.Println("arg", i, "is", arg)
	}
	if len(args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  bundle-tools --extract <input .DAT> <output Folder> [files Pattern]")
		fmt.Println("  bundle-tools --update <input .DAT> <target .DAT>")
		fmt.Println("  bundle-tools --list <input .DAT>")
		os.Exit(1)
	}

	if args[0] == "--update" {
		patchBundle(args[1], args[2])
		//patchBundle(daybreak00dat_backupPath, daybreak00dat_Path, new_00_Path) //original line
	}
	//if args[0] == "--update1" {
	//patchBundle(daybreak01dat_backupPath, daybreak01dat_Path, new_00_Path) // Original line had whole path to the game files
	//}
	if args[0] == "--extract" {
		pattern := ""
		if len(args) > 3 && args[3] != "" {
			pattern = args[3]
		}
		err := extractBundle(args[1], args[2], pattern)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}
	if args[0] == "--list" {
		err := listBundle(args[1])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Invalid command")
		os.Exit(1)
	}

}
