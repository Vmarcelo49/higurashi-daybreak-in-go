package crypto

// DecryptFileTableBlock decrypts the file table block
func DecryptFileTableBlock(index int, encryptedData []byte) []byte {
	// Apply 9-bit mask
	index = index & 0x1ff

	// Initialize counter and key
	counter := (100 + index*77) & 0xff
	key := (100*(index+1) + ((0xff & (index * (index - 1) / 2)) * 77)) & 0xff

	// Create slice to store the result
	decryptedData := make([]byte, len(encryptedData))

	// Loop to decrypt each byte
	for i := range encryptedData {
		// Apply XOR with the key
		decryptedData[i] = encryptedData[i] ^ byte(key)

		// Update key and counter
		key = (key + counter) & 0xff
		counter = (counter + 77) & 0xff
	}

	return decryptedData
}

// GetFileKey calculates the encryption key from an offset
func GetFileKey(offset int64) byte {
	return byte((offset>>1)&0xff | 0x08)
}

// EncryptFileTableBlock encrypts the file table block - inverse of decryptFileTableBlock
// but actually the same operation since it's XOR-based
func EncryptFileTableBlock(index int, decryptedData []byte) []byte {
	// Simply reuse decryptFileTableBlock since XOR encryption/decryption is the same operation
	return DecryptFileTableBlock(index, decryptedData)
}
