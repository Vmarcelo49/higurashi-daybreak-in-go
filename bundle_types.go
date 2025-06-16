package main

// FileEntry represents an entry in the file table
type FileEntry struct {
	Index  int    // Index of the file in the table
	Offset uint32 // Offset of the file in the bundle
	Length uint32 // Length of the file data
	Name   string // Name of the file
}
